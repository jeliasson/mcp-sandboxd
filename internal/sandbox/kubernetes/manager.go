package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
	"github.com/jeliasson/mcp-sandboxd/internal/sandbox"
	kubeclient "k8s.io/client-go/kubernetes"
)

type Manager struct {
	cfg       config.Config
	client    kubeclient.Interface
	namespace string

	mu                    sync.Mutex
	sandboxesByIdentifier map[string]*sandbox.Sandbox
}

func NewManager(cfg config.Config, client kubeclient.Interface) (*Manager, error) {
	ns := strings.TrimSpace(cfg.KubernetesSandboxNamespace)
	if ns == "" {
		ns = currentNamespace()
	}
	if ns == "" {
		ns = "default"
	}

	return &Manager{
		cfg:                   cfg,
		client:                client,
		namespace:             ns,
		sandboxesByIdentifier: map[string]*sandbox.Sandbox{},
	}, nil
}

func currentNamespace() string {
	if v := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); v != "" {
		return v
	}
	b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (m *Manager) labelKey(name string) string {
	return m.cfg.KubernetesSandboxLabelPrefix + "/" + name
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sandboxesByIdentifier)
}

func (m *Manager) Ensure(ctx context.Context, identifier string, ttlSeconds int) (*sandbox.Sandbox, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = m.cfg.DefaultTTLSeconds
	}
	if ttlSeconds > m.cfg.MaxTTLSeconds {
		ttlSeconds = m.cfg.MaxTTLSeconds
	}

	// Fast path: in-memory record.
	m.mu.Lock()
	s, ok := m.sandboxesByIdentifier[identifier]
	m.mu.Unlock()
	if ok {
		m.extend(identifier, ttlSeconds)
		if err := m.ensureRunning(ctx, s.ContainerID); err == nil {
			cp := *s
			return &cp, nil
		}
		_ = m.Delete(ctx, identifier)
	}

	found, err := m.findByIdentifier(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if found != nil {
		m.mu.Lock()
		m.sandboxesByIdentifier[identifier] = found
		m.mu.Unlock()
		m.extend(identifier, ttlSeconds)
		if err := m.ensureRunning(ctx, found.ContainerID); err != nil {
			_ = m.Delete(ctx, identifier)
			return m.create(ctx, identifier, ttlSeconds)
		}
		cp := *found
		return &cp, nil
	}

	return m.create(ctx, identifier, ttlSeconds)
}

func (m *Manager) Delete(ctx context.Context, identifier string) error {
	m.mu.Lock()
	s, ok := m.sandboxesByIdentifier[identifier]
	if ok {
		delete(m.sandboxesByIdentifier, identifier)
	}
	m.mu.Unlock()

	podName := ""
	if ok {
		podName = s.ContainerID
	} else {
		found, err := m.findByIdentifier(ctx, identifier)
		if err != nil {
			return err
		}
		if found != nil {
			podName = found.ContainerID
		}
	}
	if podName == "" {
		return nil
	}

	grace := int64(0)
	policy := metav1.DeletePropagationBackground
	return m.client.CoreV1().Pods(m.namespace).Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
		PropagationPolicy:  &policy,
	})
}

func (m *Manager) Restart(ctx context.Context, identifier string, ttlSeconds int) (*sandbox.Sandbox, error) {
	_ = m.Delete(ctx, identifier)
	return m.create(ctx, identifier, ttlSeconds)
}

func (m *Manager) ReapOnce(ctx context.Context) (int, error) {
	pods, err := m.client.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=sandbox", m.labelKey("role"))})
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	count := 0
	for _, p := range pods.Items {
		identifier := p.Labels[m.labelKey("identifier")]
		if identifier == "" {
			continue
		}

		expiresAt := time.Time{}
		m.mu.Lock()
		if s, ok := m.sandboxesByIdentifier[identifier]; ok {
			expiresAt = s.ExpiresAt
		}
		m.mu.Unlock()
		if expiresAt.IsZero() {
			if ms, ok := p.Labels[m.labelKey("expires-at-unix-ms")]; ok {
				if v, parseErr := parseInt64(ms); parseErr == nil {
					expiresAt = time.UnixMilli(v).UTC()
				}
			}
		}

		if !expiresAt.IsZero() && now.After(expiresAt) {
			_ = m.Delete(ctx, identifier)
			count++
		}
	}
	return count, nil
}

func (m *Manager) extend(identifier string, ttlSeconds int) {
	now := time.Now().UTC()
	expires := now.Add(time.Duration(ttlSeconds) * time.Second)

	m.mu.Lock()
	s, ok := m.sandboxesByIdentifier[identifier]
	if ok {
		s.ExpiresAt = expires
	}
	m.mu.Unlock()
}

func (m *Manager) ensureRunning(ctx context.Context, podName string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		p, err := m.client.CoreV1().Pods(m.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		switch p.Status.Phase {
		case corev1.PodRunning:
			return nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return fmt.Errorf("pod not runnable: %s", p.Status.Phase)
		default:
			// Pending/Unknown: keep waiting a bit.
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("pod not running after timeout: %s", p.Status.Phase)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (m *Manager) create(ctx context.Context, identifier string, ttlSeconds int) (*sandbox.Sandbox, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = m.cfg.DefaultTTLSeconds
	}
	if ttlSeconds > m.cfg.MaxTTLSeconds {
		ttlSeconds = m.cfg.MaxTTLSeconds
	}

	now := time.Now().UTC()
	expires := now.Add(time.Duration(ttlSeconds) * time.Second)

	name := podName(identifier)

	labels := map[string]string{
		m.labelKey("role"):               "sandbox",
		m.labelKey("identifier"):         identifier,
		m.labelKey("created-at-unix-ms"): fmt.Sprintf("%d", now.UnixMilli()),
		m.labelKey("expires-at-unix-ms"): fmt.Sprintf("%d", expires.UnixMilli()),
		m.labelKey("policy-fingerprint"): m.policyFingerprint(),
	}

	falseVal := false
	rootUID := int64(0)

	capDrop := append([]corev1.Capability{}, toKubernetesCaps(m.cfg.SandboxCapDrop)...)
	capAdd := append([]corev1.Capability{}, toKubernetesCaps(m.cfg.SandboxCapAdd)...)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyAlways,
			AutomountServiceAccountToken:  &falseVal,
			TerminationGracePeriodSeconds: ptrInt64(1),
			Containers: []corev1.Container{
				{
					Name:       m.cfg.KubernetesSandboxContainerName,
					Image:      m.cfg.SandboxImage,
					WorkingDir: "/workspace",
					Command:    []string{"sleep", "infinity"},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:                &rootUID,
						RunAsGroup:               &rootUID,
						AllowPrivilegeEscalation: ptrBool(!m.cfg.SandboxNoNewPrivileges),
						Capabilities: &corev1.Capabilities{
							Drop: capDrop,
							Add:  capAdd,
						},
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
						{Name: "artifacts", MountPath: "/artifacts"},
						{Name: "tmp", MountPath: "/tmp"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "artifacts", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}

	created, err := m.client.CoreV1().Pods(m.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	s := &sandbox.Sandbox{
		Identifier:  identifier,
		ContainerID: created.Name,
		Name:        created.Name,
		CreatedAt:   now,
		ExpiresAt:   expires,
		Image:       m.cfg.SandboxImage,
	}

	m.mu.Lock()
	m.sandboxesByIdentifier[identifier] = s
	m.mu.Unlock()

	cp := *s
	return &cp, nil
}

func (m *Manager) findByIdentifier(ctx context.Context, identifier string) (*sandbox.Sandbox, error) {
	selector := fmt.Sprintf("%s=sandbox,%s=%s", m.labelKey("role"), m.labelKey("identifier"), identifier)
	pods, err := m.client.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, nil
	}

	p := pods.Items[0]
	expected := m.policyFingerprint()
	actual := p.Labels[m.labelKey("policy-fingerprint")]
	if actual != expected {
		log.Printf(
			"sandbox policy mismatch; recreating: identifier=%s pod=%s expected=%s actual=%s",
			identifier,
			p.Name,
			expected,
			actual,
		)
		grace := int64(0)
		policy := metav1.DeletePropagationBackground
		_ = m.client.CoreV1().Pods(m.namespace).Delete(ctx, p.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &grace,
			PropagationPolicy:  &policy,
		})
		return nil, nil
	}

	createdAt := p.CreationTimestamp.Time.UTC()
	expiresAt := createdAt.Add(time.Duration(m.cfg.DefaultTTLSeconds) * time.Second)
	if v, ok := p.Labels[m.labelKey("expires-at-unix-ms")]; ok {
		if ms, parseErr := parseInt64(v); parseErr == nil {
			expiresAt = time.UnixMilli(ms).UTC()
		}
	}

	return &sandbox.Sandbox{
		Identifier:  identifier,
		ContainerID: p.Name,
		Name:        p.Name,
		CreatedAt:   createdAt,
		ExpiresAt:   expiresAt,
		Image:       m.cfg.SandboxImage,
	}, nil
}

var safeNameRe = regexp.MustCompile(`[^a-z0-9-]+`)

func podName(identifier string) string {
	lower := strings.ToLower(identifier)
	safe := safeNameRe.ReplaceAllString(lower, "-")
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "chat"
	}
	if len(safe) > 40 {
		safe = safe[:40]
	}

	h := sha256.Sum256([]byte(identifier))
	hash8 := hex.EncodeToString(h[:])[:8]

	name := fmt.Sprintf("mcp-sbx-%s-%s", safe, hash8)
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}
	return name
}

func (m *Manager) policyFingerprint() string {
	capAdd := append([]string{}, m.cfg.SandboxCapAdd...)
	capDrop := append([]string{}, m.cfg.SandboxCapDrop...)
	sort.Strings(capAdd)
	sort.Strings(capDrop)

	data := fmt.Sprintf(
		"nnp=%t;cap_add=%s;cap_drop=%s",
		m.cfg.SandboxNoNewPrivileges,
		strings.Join(capAdd, ","),
		strings.Join(capDrop, ","),
	)

	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])[:16]
}

func parseInt64(v string) (int64, error) {
	var out int64
	_, err := fmt.Sscanf(v, "%d", &out)
	if err != nil {
		return 0, err
	}
	return out, nil
}

func ptrInt64(v int64) *int64 { return &v }

func ptrBool(v bool) *bool { return &v }

func toKubernetesCaps(caps []string) []corev1.Capability {
	out := make([]corev1.Capability, 0, len(caps))
	for _, capName := range caps {
		capName = strings.TrimSpace(capName)
		if capName == "" {
			continue
		}
		out = append(out, corev1.Capability(capName))
	}
	return out
}

func init() {
	// Ensure the namespace file can be read in initContainers or rootless envs.
	_, _ = os.Stat(filepath.Dir("/var/run/secrets/kubernetes.io/serviceaccount/namespace"))
}
