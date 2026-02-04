package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/jeliasson/mcp-sandboxd/internal/client/docker"
	"github.com/jeliasson/mcp-sandboxd/internal/config"
)

type Sandbox struct {
	Identifier  string    `json:"identifier"`
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Image       string    `json:"image"`
}

type Manager struct {
	cfg          config.Config
	dockerClient *docker.Client

	mu                    sync.Mutex
	sandboxesByIdentifier map[string]*Sandbox
}

func NewManager(cfg config.Config, dockerClient *docker.Client) *Manager {
	return &Manager{
		cfg:                   cfg,
		dockerClient:          dockerClient,
		sandboxesByIdentifier: map[string]*Sandbox{},
	}
}

func (m *Manager) policyFingerprint() string {
	capAdd := append([]string{}, m.cfg.SandboxCapAdd...)
	capDrop := append([]string{}, m.cfg.SandboxCapDrop...)
	sort.Strings(capAdd)
	sort.Strings(capDrop)

	data := fmt.Sprintf(
		"network=%s;nnp=%t;cap_add=%s;cap_drop=%s",
		m.cfg.SandboxNetworkMode,
		m.cfg.SandboxNoNewPrivileges,
		strings.Join(capAdd, ","),
		strings.Join(capDrop, ","),
	)

	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])[:16]
}

func (m *Manager) Get(identifier string) (*Sandbox, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sandboxesByIdentifier[identifier]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sandboxesByIdentifier)
}

func (m *Manager) Ensure(ctx context.Context, identifier string, ttlSeconds int) (*Sandbox, error) {
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
		// If container cannot be started, drop record and recreate.
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
	// Prefer in-memory record.
	m.mu.Lock()
	s, ok := m.sandboxesByIdentifier[identifier]
	if ok {
		delete(m.sandboxesByIdentifier, identifier)
	}
	m.mu.Unlock()

	containerID := ""
	if ok {
		containerID = s.ContainerID
	} else {
		found, err := m.findByIdentifier(ctx, identifier)
		if err != nil {
			return err
		}
		if found != nil {
			containerID = found.ContainerID
		}
	}
	if containerID == "" {
		return nil
	}

	timeoutSeconds := 10
	_ = m.dockerClient.Raw().ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSeconds})
	_ = m.dockerClient.Raw().ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true, RemoveVolumes: true})
	return nil
}

func (m *Manager) Restart(ctx context.Context, identifier string, ttlSeconds int) (*Sandbox, error) {
	_ = m.Delete(ctx, identifier)
	return m.create(ctx, identifier, ttlSeconds)
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

func (m *Manager) ensureRunning(ctx context.Context, containerID string) error {
	insp, err := m.dockerClient.Raw().ContainerInspect(ctx, containerID)
	if err != nil {
		return err
	}
	if insp.State != nil && insp.State.Running {
		return nil
	}
	return m.dockerClient.Raw().ContainerStart(ctx, containerID, container.StartOptions{})
}

func (m *Manager) create(ctx context.Context, identifier string, ttlSeconds int) (*Sandbox, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = m.cfg.DefaultTTLSeconds
	}
	if ttlSeconds > m.cfg.MaxTTLSeconds {
		ttlSeconds = m.cfg.MaxTTLSeconds
	}

	now := time.Now().UTC()
	expires := now.Add(time.Duration(ttlSeconds) * time.Second)

	name := containerName(identifier)

	labels := map[string]string{
		"mcp.role":               "sandbox",
		"mcp.identifier":         identifier,
		"mcp.created_at_unix_ms": fmt.Sprintf("%d", now.UnixMilli()),
		"mcp.expires_at_unix_ms": fmt.Sprintf("%d", expires.UnixMilli()),
		"mcp.image":              m.cfg.SandboxImage,
		"mcp.policy_fingerprint": m.policyFingerprint(),
	}

	memBytes := int64(m.cfg.DefaultMemoryMB) * 1024 * 1024
	nanoCPUs := int64(m.cfg.DefaultCPUCores * 1e9)
	pids := int64(m.cfg.DefaultPIDs)

	containerCfg := &container.Config{
		Image:      m.cfg.SandboxImage,
		Cmd:        []string{"sleep", "infinity"},
		WorkingDir: "/workspace",
		User:       "1000:1000",
		Labels:     labels,
	}

	hostCfg := &container.HostConfig{
		NetworkMode: container.NetworkMode(m.cfg.SandboxNetworkMode),
		CapDrop:     append([]string{}, m.cfg.SandboxCapDrop...),
		CapAdd:      append([]string{}, m.cfg.SandboxCapAdd...),
		Resources: container.Resources{
			Memory:    memBytes,
			NanoCPUs:  nanoCPUs,
			PidsLimit: &pids,
		},
		Privileged: false,
	}
	if m.cfg.SandboxNoNewPrivileges {
		hostCfg.SecurityOpt = []string{"no-new-privileges:true"}
	}

	if err := m.ensureSandboxImage(ctx); err != nil {
		return nil, err
	}

	resp, err := m.dockerClient.Raw().ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, name)
	if err != nil {
		// If the image is missing, provide a clearer message.
		if client.IsErrNotFound(err) {
			return nil, fmt.Errorf("sandbox image not found: %s (build it with `make docker-build-sandbox` or set SANDBOX_IMAGE to an existing image)", m.cfg.SandboxImage)
		}
		return nil, err
	}

	if err := m.dockerClient.Raw().ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = m.dockerClient.Raw().ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true, RemoveVolumes: true})
		return nil, err
	}

	s := &Sandbox{
		Identifier:  identifier,
		ContainerID: resp.ID,
		Name:        name,
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

func (m *Manager) ensureSandboxImage(ctx context.Context) error {
	_, _, err := m.dockerClient.Raw().ImageInspectWithRaw(ctx, m.cfg.SandboxImage)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return err
	}

	var buildErr error
	if m.cfg.AutoBuildSandboxImage {
		log.Printf(
			"sandbox image missing; building via Docker API: image=%s dockerfile=%s",
			m.cfg.SandboxImage,
			m.cfg.SandboxDockerfilePath,
		)
		buildErr = m.buildSandboxImage(ctx)
		if buildErr == nil {
			log.Printf("sandbox image build succeeded: image=%s", m.cfg.SandboxImage)
			return nil
		}
		log.Printf("sandbox image build failed: image=%s error=%v", m.cfg.SandboxImage, buildErr)
	}

	// Best-effort pull. If this is a local dev tag, pulling from docker.io will fail.
	log.Printf("sandbox image missing; attempting pull: image=%s", m.cfg.SandboxImage)
	rc, pullErr := m.dockerClient.Raw().ImagePull(ctx, m.cfg.SandboxImage, image.PullOptions{})
	if pullErr == nil {
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
		_, _, err2 := m.dockerClient.Raw().ImageInspectWithRaw(ctx, m.cfg.SandboxImage)
		if err2 == nil {
			log.Printf("sandbox image pulled successfully: image=%s", m.cfg.SandboxImage)
			return nil
		}
	} else {
		log.Printf("sandbox image pull failed: image=%s error=%v", m.cfg.SandboxImage, pullErr)
	}

	if pullErr != nil && buildErr != nil {
		return fmt.Errorf(
			"sandbox image not found: %s (auto-build failed: %v; pull failed: %v; dockerfile=%s)",
			m.cfg.SandboxImage,
			buildErr,
			pullErr,
			m.cfg.SandboxDockerfilePath,
		)
	}
	if buildErr != nil {
		return fmt.Errorf(
			"sandbox image not found: %s (auto-build failed: %v; dockerfile=%s)",
			m.cfg.SandboxImage,
			buildErr,
			m.cfg.SandboxDockerfilePath,
		)
	}

	return fmt.Errorf(
		"sandbox image not found: %s (build it with `make docker-build-sandbox` or set SANDBOX_IMAGE to an existing image)",
		m.cfg.SandboxImage,
	)
}

func (m *Manager) buildSandboxImage(ctx context.Context) error {
	dockerfileBytes, err := os.ReadFile(m.cfg.SandboxDockerfilePath)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "Dockerfile", Mode: 0o644, Size: int64(len(dockerfileBytes))}); err != nil {
		return err
	}
	if _, err := tw.Write(dockerfileBytes); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	resp, err := m.dockerClient.Raw().ImageBuild(ctx, &buf, types.ImageBuildOptions{
		Tags:       []string{m.cfg.SandboxImage},
		Dockerfile: "Dockerfile",
		Remove:     true,
		PullParent: true,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Docker/Podman build output is a stream of JSON objects.
	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		var msg struct {
			Stream      string `json:"stream"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := dec.Decode(&msg); err != nil {
			break
		}
		if msg.Stream != "" {
			line := strings.TrimSpace(msg.Stream)
			if line != "" {
				log.Printf("sandbox build: %s", line)
			}
		}
		if msg.Error != "" {
			if msg.ErrorDetail.Message != "" {
				return fmt.Errorf("build error: %s", msg.ErrorDetail.Message)
			}
			return fmt.Errorf("build error: %s", msg.Error)
		}
	}

	_, _, err = m.dockerClient.Raw().ImageInspectWithRaw(ctx, m.cfg.SandboxImage)
	if err != nil {
		return fmt.Errorf("build finished but image still missing: %w", err)
	}
	return nil
}

func (m *Manager) findByIdentifier(ctx context.Context, identifier string) (*Sandbox, error) {
	f := filters.NewArgs()
	f.Add("label", "mcp.role=sandbox")
	f.Add("label", "mcp.identifier="+identifier)

	containers, err := m.dockerClient.Raw().ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, nil
	}

	c := containers[0]
	expected := m.policyFingerprint()
	actual := c.Labels["mcp.policy_fingerprint"]
	if actual != expected {
		log.Printf(
			"sandbox policy mismatch; recreating: identifier=%s container=%s expected=%s actual=%s",
			identifier,
			c.ID,
			expected,
			actual,
		)
		timeoutSeconds := 10
		_ = m.dockerClient.Raw().ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeoutSeconds})
		_ = m.dockerClient.Raw().ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true, RemoveVolumes: true})
		return nil, nil
	}

	created := time.Unix(c.Created, 0).UTC()

	expiresAt := created.Add(time.Duration(m.cfg.DefaultTTLSeconds) * time.Second)
	if v, ok := c.Labels["mcp.expires_at_unix_ms"]; ok {
		if ms, parseErr := parseInt64(v); parseErr == nil {
			expiresAt = time.UnixMilli(ms).UTC()
		}
	}

	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}

	return &Sandbox{
		Identifier:  identifier,
		ContainerID: c.ID,
		Name:        name,
		CreatedAt:   created,
		ExpiresAt:   expiresAt,
		Image:       c.Labels["mcp.image"],
	}, nil
}

var safeIdentifierRe = regexp.MustCompile(`[^a-z0-9-]+`)

func containerName(identifier string) string {
	lower := strings.ToLower(identifier)
	safe := safeIdentifierRe.ReplaceAllString(lower, "-")
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "chat"
	}
	if len(safe) > 40 {
		safe = safe[:40]
	}

	h := sha256.Sum256([]byte(identifier))
	hash8 := hex.EncodeToString(h[:])[:8]

	return fmt.Sprintf("mcp-sbx-%s-%s", safe, hash8)
}

func parseInt64(v string) (int64, error) {
	var out int64
	_, err := fmt.Sscanf(v, "%d", &out)
	if err != nil {
		return 0, err
	}
	return out, nil
}

func (m *Manager) ExpiredIdentifiers(now time.Time) []string {
	now = now.UTC()
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []string
	for id, s := range m.sandboxesByIdentifier {
		if now.After(s.ExpiresAt) {
			expired = append(expired, id)
		}
	}
	return expired
}

func (m *Manager) ReapOnce(ctx context.Context) (int, error) {
	// Best-effort: prefer in-memory expiries, but also discover containers by label.
	f := filters.NewArgs()
	f.Add("label", "mcp.role=sandbox")
	containers, err := m.dockerClient.Raw().ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()

	// Index by identifier.
	seen := map[string]string{}
	for _, c := range containers {
		id := c.Labels["mcp.identifier"]
		if id != "" {
			seen[id] = c.ID
		}
	}

	count := 0
	for identifier := range seen {
		m.mu.Lock()
		s, ok := m.sandboxesByIdentifier[identifier]
		m.mu.Unlock()

		expiresAt := time.Time{}
		if ok {
			expiresAt = s.ExpiresAt
		} else {
			// Fall back to label.
			found, ferr := m.findByIdentifier(ctx, identifier)
			if ferr != nil {
				return count, ferr
			}
			if found != nil {
				expiresAt = found.ExpiresAt
			}
		}

		if !expiresAt.IsZero() && now.After(expiresAt) {
			_ = m.Delete(ctx, identifier)
			count++
		}
	}

	return count, nil
}
