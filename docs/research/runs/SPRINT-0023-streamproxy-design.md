# SPRINT-0023 stream-proxy design

## Mechanism

Session surfaces use a host-side hijack-and-tunnel stub:

1. Dial the extracted service over the cluster-internal address.
2. Forward the original HTTP upgrade request, preserving method, URL, headers,
   cookies, auth headers, `X-Requested-With`, and `X-Forwarded-*`.
3. Hijack the inbound host connection with `http.Hijacker`.
4. Read the extracted service's upgrade response and write it byte-for-byte to
   the inbound client connection.
5. Bridge both raw connections with paired `io.Copy` loops. Either side closing
   closes both conns, unblocking the peer loop.

The extracted service binds the same route internally and runs the original
handler. The host stub does not replay boot goroutines; the extracted-service
main does.

## Non-choices

- `httputil.ReverseProxy` is not the v0 mechanism. The host stub must take raw
  byte ownership after upgrade, and `ReverseProxy` hides too much upgrade
  lifecycle policy behind `net/http`.
- `gorilla/websocket` is not used in emitted host stubs. It is allowed only in
  tests to prove frame byte parity through the raw tunnel.

## Failure modes

Default fail mode is closed: if the extracted service cannot be reached before
the inbound connection is hijacked, the host returns `503`.

`MONOLIFT_LIFT_FAILMODE=open` runs the original in-host handler on dial failure.
This mirrors the earlier HTTP/JSON fail-open escape hatch and is intentionally
limited to pre-hijack failures.

## Disconnect propagation

The tunnel closes both conns after either copy loop returns. Client close,
extracted close, and context cancellation all converge on the same shutdown
path: both sides close and all copy goroutines return.
