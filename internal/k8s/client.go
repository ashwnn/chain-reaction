package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Config    *rest.Config
	Clientset kubernetes.Interface
}

func NewClient(kubeconfig string, qps float32, burst int) (*Client, error) {
	cfg, err := buildRESTConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	if qps > 0 {
		cfg.QPS = qps
	}
	if burst > 0 {
		cfg.Burst = burst
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes clientset: %w", err)
	}

	return &Client{Config: cfg, Clientset: clientset}, nil
}

func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	return names, nil
}

func buildRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		if inCluster, err := rest.InClusterConfig(); err == nil {
			return inCluster, nil
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		if kubeconfig == "" {
			return nil, fmt.Errorf("load kubeconfig: %w (set --kubeconfig or run inside a cluster)", err)
		}
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	return restCfg, nil
}
