package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const DefaultClusterName = "monolift-e2e"

type Cluster struct {
	Name       string
	ConfigPath string
	Logf       func(format string, args ...any)
}

func NewCluster() Cluster {
	return Cluster{
		Name:       DefaultClusterName,
		ConfigPath: "test/e2e/fixtures/kind-config.yaml",
	}
}

func (c Cluster) Ensure(ctx context.Context) error {
	exists, err := c.exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.create(ctx); err != nil {
			return err
		}
	}
	return c.WaitNodesReady(ctx, 60*time.Second)
}

func (c Cluster) Reset(ctx context.Context) error {
	if _, err := RunCommand(ctx, "kind", "delete", "cluster", "--name", c.clusterName()); err != nil {
		return fmt.Errorf("delete kind cluster %q: %w", c.clusterName(), err)
	}
	if err := c.create(ctx); err != nil {
		return err
	}
	return c.WaitNodesReady(ctx, 60*time.Second)
}

func (c Cluster) LoadImage(ctx context.Context, imageRef string) error {
	args := []string{"load", "docker-image", imageRef, "--name", c.clusterName()}
	if c.Logf != nil {
		c.Logf("running %s", commandString("kind", args))
	}
	result, err := RunCommand(ctx, "kind", args...)
	if err != nil {
		return fmt.Errorf("kind load docker-image %s: %w: %s", imageRef, err, TailLines(result.Stderr, 20))
	}
	return nil
}

func (c Cluster) WaitNodesReady(ctx context.Context, timeout time.Duration) error {
	client, err := c.clientset()
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	var last string
	for {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			last = err.Error()
		} else if len(nodes.Items) > 0 && nodesReady(nodes.Items) {
			return nil
		} else {
			last = summarizeNodes(nodes.Items)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("kind cluster %s not ready after %s: %s", c.clusterName(), timeout, last)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c Cluster) exists(ctx context.Context) (bool, error) {
	result, err := RunCommand(ctx, "kind", "get", "clusters")
	if err != nil {
		return false, fmt.Errorf("kind get clusters: %w: %s", err, TailLines(result.Stderr, 20))
	}
	for _, name := range strings.Fields(result.Stdout) {
		if name == c.clusterName() {
			return true, nil
		}
	}
	return false, nil
}

func (c Cluster) create(ctx context.Context) error {
	args := []string{"create", "cluster", "--name", c.clusterName(), "--config", c.configPath()}
	if c.Logf != nil {
		c.Logf("running %s", commandString("kind", args))
	}
	result, err := RunCommand(ctx, "kind", args...)
	if err != nil {
		return fmt.Errorf("kind create cluster %q: %w: %s", c.clusterName(), err, TailLines(result.Stderr, 20))
	}
	return nil
}

func (c Cluster) clientset() (*kubernetes.Clientset, error) {
	config, err := c.restConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func (c Cluster) restConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err == nil && config.Host != "" {
		return config, nil
	}
	result, kindErr := RunCommand(context.Background(), "kind", "get", "kubeconfig", "--name", c.clusterName())
	if kindErr != nil {
		return nil, fmt.Errorf("load kubeconfig: %v; kind get kubeconfig: %v: %s", err, kindErr, TailLines(result.Stderr, 20))
	}
	return clientcmd.RESTConfigFromKubeConfig([]byte(result.Stdout))
}

func (c Cluster) clusterName() string {
	if c.Name != "" {
		return c.Name
	}
	return DefaultClusterName
}

func (c Cluster) configPath() string {
	if c.ConfigPath != "" {
		return FromRepoRoot(c.ConfigPath)
	}
	return FromRepoRoot("test/e2e/fixtures/kind-config.yaml")
}

func nodesReady(nodes []corev1.Node) bool {
	for _, node := range nodes {
		ready := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if !ready {
			return false
		}
	}
	return true
}

func summarizeNodes(nodes []corev1.Node) string {
	if len(nodes) == 0 {
		return "no nodes found"
	}
	var parts []string
	for _, node := range nodes {
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				status = string(condition.Status)
				break
			}
		}
		parts = append(parts, node.Name+"="+status)
	}
	return strings.Join(parts, ", ")
}
