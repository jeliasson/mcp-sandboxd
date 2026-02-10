package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestManagerEnsureCreatesAndReusesPod(t *testing.T) {
	cs := fake.NewSimpleClientset()

	cfg := config.Config{
		SandboxImage:                   "example:sandbox",
		DefaultTTLSeconds:              60,
		MaxTTLSeconds:                  60,
		KubernetesSandboxNamespace:     "sandboxes",
		KubernetesSandboxContainerName: "sandbox",
		SandboxNetworkMode:             "bridge",
		SandboxNoNewPrivileges:         true,
		SandboxCapDrop:                 []string{"ALL"},
		SandboxCapAdd:                  []string{"SETUID"},
		KubernetesSandboxLabelPrefix:   "mcp-sandboxd.jeliasson.dev",
	}

	mgr, err := NewManager(cfg, cs)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	s1, err := mgr.Ensure(ctx, "chat1", 1)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	mgr.mu.Lock()
	mgr.sandboxesByIdentifier["chat1"].ExpiresAt = time.Now().Add(-1 * time.Second)
	mgr.mu.Unlock()

	deleted, err := mgr.ReapOnce(ctx)
	if err != nil {
		t.Fatalf("ReapOnce: %v", err)
	}
	if deleted == 0 {
		t.Fatalf("expected deleted > 0")
	}

	_, err = cs.CoreV1().Pods("sandboxes").Get(ctx, s1.ContainerID, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected pod deleted")
	}
}

func TestManagerPodSecurityContextMirrorsSandboxConfig(t *testing.T) {
	cs := fake.NewSimpleClientset()

	cfg := config.Config{
		SandboxImage:                   "example:sandbox",
		DefaultTTLSeconds:              60,
		MaxTTLSeconds:                  60,
		KubernetesSandboxNamespace:     "sandboxes",
		KubernetesSandboxContainerName: "sandbox",
		SandboxNoNewPrivileges:         true,
		SandboxCapDrop:                 []string{"ALL"},
		SandboxCapAdd:                  []string{"SETUID", "NET_RAW"},
		KubernetesSandboxLabelPrefix:   "mcp-sandboxd.jeliasson.dev",
	}

	mgr, err := NewManager(cfg, cs)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	s, err := mgr.Ensure(ctx, "chat1", 30)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	p, err := cs.CoreV1().Pods("sandboxes").Get(ctx, s.ContainerID, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if len(p.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(p.Spec.Containers))
	}

	sc := p.Spec.Containers[0].SecurityContext
	if sc == nil {
		t.Fatalf("expected securityContext")
	}
	if sc.Capabilities == nil {
		t.Fatalf("expected capabilities")
	}

	haveDrop := map[string]bool{}
	for _, c := range sc.Capabilities.Drop {
		haveDrop[string(c)] = true
	}
	if !haveDrop["ALL"] {
		t.Fatalf("expected capabilities.drop to include ALL, got %#v", sc.Capabilities.Drop)
	}

	haveAdd := map[string]bool{}
	for _, c := range sc.Capabilities.Add {
		haveAdd[string(c)] = true
	}
	for _, expected := range []string{"SETUID", "NET_RAW"} {
		if !haveAdd[expected] {
			t.Fatalf("expected capabilities.add to include %s, got %#v", expected, sc.Capabilities.Add)
		}
	}

	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Fatalf("expected allowPrivilegeEscalation=false, got %#v", sc.AllowPrivilegeEscalation)
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("expected seccompProfile RuntimeDefault, got %#v", sc.SeccompProfile)
	}
}

func TestPodReadyForExec(t *testing.T) {
	p := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
		Name:  "sandbox",
		Ready: true,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}}}
	if ok, why := podReadyForExec(p, "sandbox"); !ok {
		t.Fatalf("expected ready, got: %s", why)
	}

	p2 := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
		Name:  "sandbox",
		Ready: false,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}}}
	if ok, _ := podReadyForExec(p2, "sandbox"); ok {
		t.Fatalf("expected not ready")
	}

	p3 := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}
	if ok, _ := podReadyForExec(p3, "sandbox"); ok {
		t.Fatalf("expected not ready")
	}

	p4 := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	if ok, why := podReadyForExec(p4, "sandbox"); ok || why == "" {
		t.Fatalf("expected not ready with reason, got ok=%t reason=%q", ok, why)
	}
}

func TestManagerPolicyMismatchRecreates(t *testing.T) {
	cs := fake.NewSimpleClientset()

	cfg := config.Config{
		SandboxImage:                   "example:sandbox",
		DefaultTTLSeconds:              60,
		MaxTTLSeconds:                  60,
		KubernetesSandboxNamespace:     "sandboxes",
		KubernetesSandboxContainerName: "sandbox",
		SandboxNetworkMode:             "bridge",
		SandboxNoNewPrivileges:         true,
		SandboxCapDrop:                 []string{"ALL"},
		SandboxCapAdd:                  []string{"SETUID"},
		KubernetesSandboxLabelPrefix:   "mcp-sandboxd.jeliasson.dev",
	}

	mgr, err := NewManager(cfg, cs)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	name := podName("chat1")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "sandboxes",
			Labels: map[string]string{
				mgr.labelKey("role"):               "sandbox",
				mgr.labelKey("identifier"):         "chat1",
				mgr.labelKey("policy-fingerprint"): "bad",
				mgr.labelKey("expires-at-unix-ms"): "9999999999999",
			},
		},
		Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "sandbox", Image: cfg.SandboxImage}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	if _, err := cs.CoreV1().Pods("sandboxes").Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create pod: %v", err)
	}

	s, err := mgr.Ensure(ctx, "chat1", 30)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	p, err := cs.CoreV1().Pods("sandboxes").Get(ctx, s.ContainerID, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if p.Labels[mgr.labelKey("policy-fingerprint")] == "bad" {
		t.Fatalf("expected fingerprint to be updated")
	}
}
