package streamproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tgoodwin/monolift/pkg/compiler/extract/bootpath"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
	corev1 "k8s.io/api/core/v1"
)

func TestRawTunnelBytesAndHeaders(t *testing.T) {
	headerCh := make(chan http.Header, 1)
	extracted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerCh <- r.Header.Clone()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack extracted: %v", err)
			return
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: raw\r\nConnection: Upgrade\r\n\r\n")); err != nil {
			t.Errorf("write upgrade: %v", err)
			return
		}
		_, _ = io.Copy(conn, conn)
	}))
	defer extracted.Close()

	host := httptest.NewServer(ProxyHandler(addrOf(extracted.URL), nil))
	defer host.Close()

	conn, response := rawUpgrade(t, addrOf(host.URL), "Cookie: sid=abc\r\nAuthorization: Bearer tok\r\nX-Requested-With: XMLHttpRequest\r\nX-Forwarded-For: 127.0.0.1\r\n")
	defer conn.Close()
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status=%d", response.StatusCode)
	}
	payload := []byte("abcdef\x00\x01")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != string(payload) {
		t.Fatalf("echo mismatch got %q want %q", buf, payload)
	}

	headers := <-headerCh
	for key, want := range map[string]string{
		"Cookie":           "sid=abc",
		"Authorization":    "Bearer tok",
		"X-Requested-With": "XMLHttpRequest",
		"X-Forwarded-For":  "127.0.0.1",
	} {
		if got := headers.Get(key); got != want {
			t.Fatalf("%s=%q want %q", key, got, want)
		}
	}
}

func TestGorillaWebsocketPayloadParity(t *testing.T) {
	payloadCh := make(chan []byte, 1)
	upgrader := websocket.Upgrader{}
	extracted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Upgrade: %v", err)
			return
		}
		defer conn.Close()
		mt, payload, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("ReadMessage: %v", err)
			return
		}
		payloadCh <- append([]byte(nil), payload...)
		if err := conn.WriteMessage(mt, payload); err != nil {
			t.Errorf("WriteMessage: %v", err)
		}
	}))
	defer extracted.Close()

	host := httptest.NewServer(ProxyHandler(addrOf(extracted.URL), nil))
	defer host.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws://"+addrOf(host.URL)+"/ws", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer ws.Close()

	want := []byte{0, 1, 2, 3, 255, 254, 253}
	if err := ws.WriteMessage(websocket.BinaryMessage, want); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	mt, got, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if mt != websocket.BinaryMessage || string(got) != string(want) {
		t.Fatalf("echo type=%d payload=%v want %v", mt, got, want)
	}
	if seen := <-payloadCh; string(seen) != string(want) {
		t.Fatalf("extracted payload=%v want %v", seen, want)
	}
}

func TestConnectionLifetimeClientClosePropagates(t *testing.T) {
	closed := make(chan struct{})
	extracted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack extracted: %v", err)
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))
		_, _ = io.Copy(io.Discard, conn)
		close(closed)
	}))
	defer extracted.Close()
	host := httptest.NewServer(ProxyHandler(addrOf(extracted.URL), nil))
	defer host.Close()

	conn, response := rawUpgrade(t, addrOf(host.URL), "")
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status=%d", response.StatusCode)
	}
	_ = conn.Close()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("client close did not propagate to extracted side")
	}
}

func TestConnectionLifetimeExtractedClosePropagates(t *testing.T) {
	extracted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack extracted: %v", err)
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))
		_ = conn.Close()
	}))
	defer extracted.Close()
	host := httptest.NewServer(ProxyHandler(addrOf(extracted.URL), nil))
	defer host.Close()

	conn, response := rawUpgrade(t, addrOf(host.URL), "")
	defer conn.Close()
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status=%d", response.StatusCode)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err := conn.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("client read succeeded after extracted close")
	}
}

func TestFailModes(t *testing.T) {
	closedHost := httptest.NewServer(ProxyHandler(unusedAddr(t), nil))
	defer closedHost.Close()
	resp, err := http.Get(closedHost.URL)
	if err != nil {
		t.Fatalf("closed GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("closed status=%d want 503", resp.StatusCode)
	}

	t.Setenv("MONOLIFT_LIFT_FAILMODE", "open")
	openHost := httptest.NewServer(ProxyHandler(unusedAddr(t), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer openHost.Close()
	resp, err = http.Get(openHost.URL)
	if err != nil {
		t.Fatalf("open GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("open status=%d want 204", resp.StatusCode)
	}
}

func TestInternalServiceIsClusterIP(t *testing.T) {
	service := InternalService("monolift-extracted-session", 8081)
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("service type=%q want ClusterIP", service.Spec.Type)
	}
}

func TestEmitterOutputs(t *testing.T) {
	plan := emit.RegionPlan{
		Region: emit.RegionSpec{Name: "session", Roots: []emit.RegionRootSpec{{
			FuncName:          "ServeWS",
			ReceiverType:      "*Hub",
			File:              "hub.go",
			ExpectedSignature: "func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request)",
		}}},
		PackageImportPath: "example.com/session/platform",
		PackageDir:        "/tmp/platform",
		ExtractedAddress:  "monolift-extracted-session:8081",
		Boot:              bootpath.BootSpec{GoroutineLaunches: []bootpath.GoroutineLaunch{{Callee: "(*Hub).Start"}}},
	}
	requests, err := (Emitter{}).EmitHostStub(plan)
	if err != nil {
		t.Fatalf("EmitHostStub: %v", err)
	}
	if len(requests) != 1 || requests[0].ReceiverType != "*Hub" || !strings.Contains(requests[0].Prelude.GoSource, "monoliftStreamProxy") {
		t.Fatalf("unexpected host stub requests: %+v", requests)
	}
	mainFile, err := (Emitter{}).EmitExtractedMain(plan)
	if err != nil {
		t.Fatalf("EmitExtractedMain: %v", err)
	}
	if !strings.HasSuffix(mainFile.Path, "main.go") {
		t.Fatalf("main path=%q", mainFile.Path)
	}
	oracle, err := (Emitter{}).EmitOracleMain(plan)
	if err != nil {
		t.Fatalf("EmitOracleMain: %v", err)
	}
	if !strings.Contains(oracle.Path, "oracle") {
		t.Fatalf("oracle path=%q", oracle.Path)
	}
	deploy, _, _, err := (Emitter{}).EmitDeployment(plan)
	if err != nil {
		t.Fatalf("EmitDeployment: %v", err)
	}
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("deployment containers=%d", len(deploy.Spec.Template.Spec.Containers))
	}
	if got := ReplayGoroutines(plan.Boot); len(got) != 1 || got[0] != "(*Hub).Start" {
		t.Fatalf("ReplayGoroutines=%v", got)
	}
}

func TestMultiRootToyPlanIntegration(t *testing.T) {
	plan := emit.RegionPlan{
		Region: emit.RegionSpec{Name: "streamproxy-multiroot", Roots: []emit.RegionRootSpec{
			{FuncName: "ServeAlpha", ReceiverType: "*Alpha", File: "main.go", ExpectedSignature: "func (a *Alpha) ServeAlpha(w http.ResponseWriter, r *http.Request)", Route: "/alpha"},
			{FuncName: "ServeBeta", ReceiverType: "*Beta", File: "main.go", ExpectedSignature: "func (b *Beta) ServeBeta(w http.ResponseWriter, r *http.Request)", Route: "/beta"},
		}},
		PackageImportPath: "streamproxy-multiroot-toy",
		PackageDir:        "../../../test/e2e/targets/streamproxy-multiroot-toy",
		ExtractedAddress:  "127.0.0.1:8081",
		Boot: bootpath.BootSpec{ConfigSources: []bootpath.ConfigSource{
			bootpath.EnvSource{Name: "STREAMPROXY_ALPHA"},
			bootpath.EnvSource{Name: "STREAMPROXY_BETA"},
			bootpath.FlagSource{Name: "config", Default: "config.json"},
		}},
	}
	requests, err := (Emitter{}).EmitHostStub(plan)
	if err != nil {
		t.Fatalf("EmitHostStub: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("patch requests=%d want 2", len(requests))
	}
	if requests[0].Prelude.GoSource == "" || requests[1].Prelude.GoSource == "" {
		t.Fatalf("patch requests missing preludes: %+v", requests)
	}
	deploy, configMap, _, err := (Emitter{}).EmitDeployment(plan)
	if err != nil {
		t.Fatalf("EmitDeployment: %v", err)
	}
	if deploy.Name == "" || configMap == nil || len(configMap.Data) != 3 {
		t.Fatalf("deployment/configMap not rendered: deploy=%s config=%#v", deploy.Name, configMap)
	}
	mainFile, err := (Emitter{}).EmitExtractedMain(plan)
	if err != nil {
		t.Fatalf("EmitExtractedMain: %v", err)
	}
	oracle, err := (Emitter{}).EmitOracleMain(plan)
	if err != nil {
		t.Fatalf("EmitOracleMain: %v", err)
	}
	if len(mainFile.Content) == 0 || len(oracle.Content) == 0 {
		t.Fatal("generated main/oracle files are empty")
	}
}

func rawUpgrade(t *testing.T, addr, extraHeaders string) (net.Conn, *http.Response) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	req := fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: raw\r\n%s\r\n", addr, extraHeaders)
	if _, err := conn.Write([]byte(req)); err != nil {
		_ = conn.Close()
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("ReadResponse: %v", err)
	}
	return conn, resp
}

func addrOf(rawURL string) string {
	return strings.TrimPrefix(strings.TrimPrefix(rawURL, "http://"), "https://")
}

func unusedAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func TestBridgeContextCancel(t *testing.T) {
	left, right := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		bridge(ctx, left, right)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not stop after context cancellation")
	}
}
