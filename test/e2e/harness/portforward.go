package harness

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

type PortForward struct {
	URL  string
	stop func()
}

func (p PortForward) Stop() {
	if p.stop != nil {
		p.stop()
	}
}

func StartPortForward(ctx context.Context, target, ns, service string, servicePort int) (PortForward, error) {
	localPort, err := freeLocalPort()
	if err != nil {
		return PortForward{}, err
	}
	kubeconfig, cleanup, err := writeKindKubeconfig(ctx, DefaultClusterName)
	if err != nil {
		return PortForward{}, StageError(2, target, KindWorkload, "write kubeconfig for port-forward failed: %v", err)
	}
	pfCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(pfCtx, "kubectl", "--kubeconfig", kubeconfig, "-n", ns, "port-forward", "svc/"+service, fmt.Sprintf("%d:%d", localPort, servicePort))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		cancel()
		cleanup()
		return PortForward{}, StageError(2, target, KindWorkload, "port-forward start failed: %v", err)
	}
	if err := waitLocalPort(ctx, localPort, 30*time.Second); err != nil {
		cancel()
		_ = cmd.Wait()
		cleanup()
		return PortForward{}, StageError(2, target, KindWorkload, "port-forward timed out: %v: %s", err, output.String())
	}
	return PortForward{
		URL: fmt.Sprintf("http://127.0.0.1:%d", localPort),
		stop: func() {
			cancel()
			_ = cmd.Wait()
			cleanup()
		},
	}, nil
}

func freeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitLocalPort(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return fmt.Errorf("%s did not open within %s", addr, timeout)
}

func writeKindKubeconfig(ctx context.Context, clusterName string) (string, func(), error) {
	result, err := RunCommand(ctx, "kind", "get", "kubeconfig", "--name", clusterName)
	if err != nil {
		return "", func() {}, fmt.Errorf("kind get kubeconfig: %w: %s", err, TailLines(result.Stderr, 20))
	}
	file, err := os.CreateTemp("", "monolift-e2e-kubeconfig-*")
	if err != nil {
		return "", func() {}, err
	}
	defer file.Close()
	if _, err := file.WriteString(result.Stdout); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}
