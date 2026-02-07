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
