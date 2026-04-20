package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

type Deployer struct {
	Cluster Cluster
	Target  string
}

func (d Deployer) Apply(ctx context.Context, ns string, manifests []string) error {
	dynamicClient, discoveryClient, err := d.dynamicClients()
	if err != nil {
		return StageError(7, d.Target, KindHarness, "create k8s clients: %v", err)
	}
	mapper, err := restMapper(discoveryClient)
	if err != nil {
		return StageError(7, d.Target, KindHarness, "create REST mapper: %v", err)
	}

	for _, manifest := range manifests {
		data, err := os.ReadFile(FromRepoRoot(manifest))
		if err != nil {
			return StageError(7, d.Target, KindHarness, "read manifest %s: %v", manifest, err)
		}
		for _, doc := range splitYAMLDocuments(data) {
			if len(bytes.TrimSpace(doc)) == 0 {
				continue
			}
			if err := d.applyDocument(ctx, dynamicClient, mapper, ns, doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d Deployer) WaitReady(ctx context.Context, ns string, timeout time.Duration) error {
	client, err := d.clientset()
	if err != nil {
		return StageError(7, d.Target, KindHarness, "create k8s client: %v", err)
	}
	deadline := time.Now().Add(timeout)
	var last string
	for {
		pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			last = err.Error()
		} else if len(pods.Items) > 0 && podsReady(pods.Items) {
			return nil
		} else {
			last = summarizePods(pods.Items)
		}

		if time.Now().After(deadline) {
			return StageError(7, d.Target, KindHarness, "pods not ready in namespace %s after %s: %s\n%s", ns, timeout, last, d.dumpNamespace(ctx, client, ns))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (d Deployer) CreateNamespace(ctx context.Context, ns string) error {
	client, err := d.clientset()
	if err != nil {
		return StageError(0, d.Target, KindHarness, "create k8s client: %v", err)
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return StageError(0, d.Target, KindHarness, "create namespace %s: %v", ns, err)
	}
	return nil
}

func (d Deployer) DeleteNamespace(ctx context.Context, ns string, timeout time.Duration) error {
	if KeepNamespaces() {
		return nil
	}
	client, err := d.clientset()
	if err != nil {
		return StageError(10, d.Target, KindHarness, "create k8s client: %v", err)
	}
	err = client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return StageError(10, d.Target, KindHarness, "delete namespace %s: %v", ns, err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return StageError(10, d.Target, KindHarness, "namespace %s still exists after %s", ns, timeout)
}

func Namespace(prefix, target, runID string) string {
	return fmt.Sprintf("mlv2-%s-%s-%s", prefix, target, runID)
}

func NewRunID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (d Deployer) applyDocument(ctx context.Context, dynamicClient dynamic.Interface, mapper *restmapper.DeferredDiscoveryRESTMapper, ns string, doc []byte) error {
	jsonDoc, err := yaml.YAMLToJSON(doc)
	if err != nil {
		return StageError(7, d.Target, KindHarness, "manifest YAML parse failed: %v", err)
	}
	var obj unstructured.Unstructured
	if err := json.Unmarshal(jsonDoc, &obj); err != nil {
		return StageError(7, d.Target, KindHarness, "manifest JSON decode failed: %v", err)
	}
	if obj.GetKind() == "" {
		return nil
	}
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		return StageError(7, d.Target, KindHarness, "map %s: %v", gvk.String(), err)
	}
	if mapping.Scope.Name() != "root" && obj.GetNamespace() == "" {
		obj.SetNamespace(ns)
	}
	payload, err := json.Marshal(&obj)
	if err != nil {
		return StageError(7, d.Target, KindHarness, "marshal %s/%s: %v", obj.GetKind(), obj.GetName(), err)
	}
	if mapping.Scope.Name() != "root" {
		_, err = dynamicClient.Resource(mapping.Resource).Namespace(ns).Patch(ctx, obj.GetName(), types.ApplyPatchType, payload, metav1.PatchOptions{
			FieldManager: "monolift-e2e",
			Force:        ptr(true),
		})
	} else {
		_, err = dynamicClient.Resource(mapping.Resource).Patch(ctx, obj.GetName(), types.ApplyPatchType, payload, metav1.PatchOptions{
			FieldManager: "monolift-e2e",
			Force:        ptr(true),
		})
	}
	if err != nil {
		return StageError(7, d.Target, KindHarness, "apply %s/%s: %v", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}

func (d Deployer) clientset() (*kubernetes.Clientset, error) {
	return d.cluster().clientset()
}

func (d Deployer) dynamicClients() (dynamic.Interface, discovery.DiscoveryInterface, error) {
	config, err := d.cluster().restConfig()
	if err != nil {
		return nil, nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return dyn, disco, nil
}

func (d Deployer) cluster() Cluster {
	if d.Cluster.Name != "" || d.Cluster.ConfigPath != "" {
		return d.Cluster
	}
	return NewCluster()
}

func (d Deployer) dumpNamespace(ctx context.Context, client *kubernetes.Clientset, ns string) string {
	var b strings.Builder
	pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "pod list failed: " + err.Error()
	}
	for _, pod := range pods.Items {
		fmt.Fprintf(&b, "pod/%s phase=%s\n", pod.Name, pod.Status.Phase)
		for _, condition := range pod.Status.Conditions {
			fmt.Fprintf(&b, "  condition %s=%s reason=%s message=%s\n", condition.Type, condition.Status, condition.Reason, condition.Message)
		}
		for _, container := range pod.Spec.Containers {
			req := client.CoreV1().Pods(ns).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name, TailLines: ptr[int64](50)})
			stream, err := req.Stream(ctx)
			if err != nil {
				fmt.Fprintf(&b, "  logs/%s failed: %v\n", container.Name, err)
				continue
			}
			data, _ := io.ReadAll(stream)
			_ = stream.Close()
			fmt.Fprintf(&b, "  logs/%s:\n%s\n", container.Name, data)
		}
	}
	return b.String()
}

func restMapper(client discovery.DiscoveryInterface) (*restmapper.DeferredDiscoveryRESTMapper, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(client))
	_, err := mapper.ResourcesFor(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"})
	if err != nil {
		return nil, err
	}
	return mapper, nil
}

func splitYAMLDocuments(data []byte) [][]byte {
	parts := bytes.Split(data, []byte("\n---"))
	return parts
}

func podsReady(pods []corev1.Pod) bool {
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				goto nextPod
			}
		}
		return false
	nextPod:
	}
	return true
}

func summarizePods(pods []corev1.Pod) string {
	if len(pods) == 0 {
		return "no pods found"
	}
	parts := make([]string, 0, len(pods))
	for _, pod := range pods {
		parts = append(parts, fmt.Sprintf("%s=%s", pod.Name, pod.Status.Phase))
	}
	return strings.Join(parts, ", ")
}

func ptr[T any](v T) *T {
	return &v
}
