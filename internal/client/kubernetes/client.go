package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Config    *rest.Config
	Clientset *kubernetes.Clientset
}

func (c *Client) Ping(ctx context.Context) error {
	_ = ctx
	_, err := c.Clientset.Discovery().ServerVersion()
	return err
}

func New() (*Client, error) {
	cfg, err := inClusterOrKubeconfig()
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Config: cfg, Clientset: cs}, nil
}

func inClusterOrKubeconfig() (*rest.Config, error) {
	// Prefer in-cluster config when running inside Kubernetes.
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	// Fall back to kubeconfig for local development.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	if kubeconfig == "" {
		return nil, fmt.Errorf("kubeconfig not found and in-cluster config unavailable")
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
