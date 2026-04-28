package streamproxy

import (
	"bufio"
	"context"
	"fmt"
	"go/format"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tgoodwin/monolift/pkg/compiler/extract/bootpath"
	"github.com/tgoodwin/monolift/pkg/compiler/transport"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit/liftpatch"
	manifestcfg "github.com/tgoodwin/monolift/pkg/compiler/transport/emit/manifest"
)

type Emitter struct{}

func init() {
	emit.Register(transport.TemplateStreamProxy, func(ctx emit.Context) (emit.Artifact, error) {
		return emit.Artifact{}, fmt.Errorf("%w: streamproxy requires a region plan", emit.ErrTemplateUnsupported)
	})
}

func (Emitter) EmitHostStub(plan emit.RegionPlan) ([]liftpatch.PatchSymbolRequest, error) {
	if len(plan.Region.Roots) == 0 {
		return nil, fmt.Errorf("streamproxy: region %q has no roots", plan.Region.Name)
	}
	addr := plan.ExtractedAddress
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	var out []liftpatch.PatchSymbolRequest
	for _, root := range plan.Region.Roots {
		source := fmt.Sprintf(`return monoliftStreamProxy(w, r, %q, func() {
	%s
})`, addr, originalCall(root))
		out = append(out, liftpatch.PatchSymbolRequest{
			PackageImportPath: plan.PackageImportPath,
			PackageDir:        plan.PackageDir,
			File:              root.File,
			FuncName:          root.FuncName,
			ReceiverType:      root.ReceiverType,
			ExpectedSignature: root.ExpectedSignature,
			Prelude: liftpatch.PreludeSpec{
				GoSource:        source,
				RequiredImports: []string{"net/http"},
			},
			GeneratedFiles: []liftpatch.GeneratedFile{helperFile(plan)},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReceiverType == out[j].ReceiverType {
			return out[i].FuncName < out[j].FuncName
		}
		return out[i].ReceiverType < out[j].ReceiverType
	})
	return out, nil
}

func (Emitter) EmitExtractedMain(plan emit.RegionPlan) (liftpatch.GeneratedFile, error) {
	service := serviceName(plan)
	body := "package main\n\nimport \"net/http\"\n\nfunc main() {\n"
	body += "\t_ = http.ListenAndServe(\":8081\", nil)\n"
	body += "}\n"
	formatted, err := format.Source([]byte(body))
	if err != nil {
		return liftpatch.GeneratedFile{}, err
	}
	return liftpatch.GeneratedFile{Path: path.Join("cmd", service, "main.go"), Content: formatted}, nil
}

func (Emitter) EmitOracleMain(plan emit.RegionPlan) (liftpatch.GeneratedFile, error) {
	service := serviceName(plan)
	body := "package main\n\nfunc main() {}\n"
	formatted, err := format.Source([]byte(body))
	if err != nil {
		return liftpatch.GeneratedFile{}, err
	}
	return liftpatch.GeneratedFile{Path: path.Join("cmd", service+"-oracle", "main.go"), Content: formatted}, nil
}

func (Emitter) EmitDeployment(plan emit.RegionPlan) (appsv1.Deployment, *corev1.ConfigMap, *corev1.Secret, error) {
	env, configMap, secret, mounts, args := manifestcfg.RenderConfig(plan.Boot)
	labels := map[string]string{"app.kubernetes.io/name": serviceName(plan)}
	deployment := appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: serviceName(plan), Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:         serviceName(plan),
					Image:        serviceName(plan) + ":latest",
					Env:          env,
					Args:         args,
					VolumeMounts: mounts,
				}}},
			},
		},
	}
	return deployment, configMap, secret, nil
}

func InternalService(name string, port int32) corev1.Service {
	labels := map[string]string{"app.kubernetes.io/name": name}
	return corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports:    []corev1.ServicePort{{Port: port}},
		},
	}
}

func ProxyHandler(extractedAddr string, original http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := Tunnel(r.Context(), w, r, extractedAddr)
		if err == nil {
			return
		}
		if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "open" && original != nil {
			original(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	}
}

func Tunnel(ctx context.Context, w http.ResponseWriter, r *http.Request, extractedAddr string) error {
	var dialer net.Dialer
	outbound, err := dialer.DialContext(ctx, "tcp", extractedAddr)
	if err != nil {
		return err
	}
	defer outbound.Close()
	if err := r.Write(outbound); err != nil {
		return err
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("response writer does not implement http.Hijacker")
	}
	inbound, rw, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer inbound.Close()
	if rw.Reader.Buffered() > 0 {
		if _, err := io.CopyN(outbound, rw, int64(rw.Reader.Buffered())); err != nil {
			return err
		}
	}
	response, err := http.ReadResponse(bufio.NewReader(outbound), r)
	if err != nil {
		return err
	}
	if err := response.Write(inbound); err != nil {
		return err
	}
	bridge(ctx, inbound, outbound)
	return nil
}

func bridge(ctx context.Context, inbound net.Conn, outbound net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = inbound.Close()
			_ = outbound.Close()
			cancel()
		})
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(outbound, inbound)
		closeBoth()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(inbound, outbound)
		closeBoth()
	}()
	go func() {
		<-ctx.Done()
		closeBoth()
	}()
	wg.Wait()
}

func helperFile(plan emit.RegionPlan) liftpatch.GeneratedFile {
	content := []byte(`package ` + helperPackage(plan) + `

import "net/http"

func monoliftStreamProxy(w http.ResponseWriter, r *http.Request, addr string, original func()) {
	_ = original
	streamproxy.ProxyHandler(addr, nil)(w, r)
}
`)
	return liftpatch.GeneratedFile{Path: path.Join("monolift_streamproxy.go"), Content: content}
}

func originalCall(root emit.RegionRootSpec) string {
	if root.ReceiverType == "" {
		return root.FuncName + "()"
	}
	return "_ = " + strings.TrimPrefix(root.ReceiverType, "*")
}

func helperPackage(plan emit.RegionPlan) string {
	if plan.PackageImportPath == "" {
		return "main"
	}
	return path.Base(plan.PackageImportPath)
}

func serviceName(plan emit.RegionPlan) string {
	if plan.ServiceName != "" {
		return plan.ServiceName
	}
	if plan.Region.Name != "" {
		return "monolift-extracted-" + strings.ToLower(strings.ReplaceAll(plan.Region.Name, "_", "-"))
	}
	return "monolift-extracted-session"
}

func ReplayGoroutines(spec bootpath.BootSpec) []string {
	out := make([]string, 0, len(spec.GoroutineLaunches))
	for _, launch := range spec.GoroutineLaunches {
		out = append(out, launch.Callee)
	}
	sort.Strings(out)
	return out
}

func CloseAfter(conn net.Conn, d time.Duration) {
	time.AfterFunc(d, func() { _ = conn.Close() })
}
