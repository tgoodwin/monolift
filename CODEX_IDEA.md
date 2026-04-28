 Your hunch is right. The current compiler is asking “what is inside the lifted region?” but the Mattermost
  blocker needs a separate pass that asks “what externally invokes this region, and through what initialized object
  graph?”

  Concrete evidence from the repo:

  - pkg/compiler/surface/surface.go:31 only inspects root.ExposedOperations / reachable region functions. That
    misses route-bound handlers outside the declared Hub/WebConn roots.
  - pkg/compiler/extract/bootpath/walk.go:13 scans the union plus main, but it does not reconstruct the path from
    the served HTTP API back to the region.
  - Mattermost’s actual external surface is evaluation/mattermost/server/channels/api4/websocket.go:57, registered
    by evaluation/mattermost/server/channels/api4/websocket.go:52, wrapped by evaluation/mattermost/server/
    channels/web/handlers.go:544, then eventually invoking NewWebConn, HubRegister, and wc.Pump.
  - The route init is reachable from evaluation/mattermost/server/channels/api4/api.go:185, which is called by the
    Mattermost server command path.

  The strategy I’d land is a new generic Invocation Trace pass, not a Mattermost recognizer.

  Core idea:

  1. Build a higher-order invocation graph, not just a call graph.
     It needs edges for direct calls, method calls, function values, interface dispatch, handler wrappers, field
     stores/loads, returned handler objects, goroutine launches, and callback registration.
  2. Seed it from both ends:
      - Region side: lifted roots plus region-owned types/functions, e.g. Hub, WebConn, PlatformService.NewWebConn,
        HubRegister, Pump.
      - Entrypoint side: main.main, http.Server.Handler, Serve, ListenAndServe, and values implementing
        http.Handler.
  3. Find intersections:
     A function outside the region becomes an external surface candidate if it is reachable from the application
     entrypoint and it touches/constructs/calls into the lifted region. For Mattermost, that should identify
     connectWebSocket.
  4. Produce an InvocationTrace artifact:
     It should record:
      - external callable: connectWebSocket
      - registration chain: api4.Init -> InitWebSocket -> Router.Handle(...
        APIHandlerTrustRequester(connectWebSocket)).Methods(GET)
      - wrapper chain: APIHandlerTrustRequester -> web.Handler.HandleFunc -> Handler.ServeHTTP
      - region touchpoints: NewWebConn, HubRegister, wc.Pump, PopulateWebConnConfig
      - boot/start chain candidates: app.NewServer -> platform.New -> PlatformService.Start -> hubStart ->
        Hub.Start

  This avoids a hard-coded taxonomy of app patterns. There will still be a small set of ecosystem boundary
  semantics, mainly net/http concepts like http.Handler, http.Server.Handler, and ServeHTTP. That is different from
  hard-coding Mattermost, gorilla/mux, or “methods named InitWebSocket.” The analyzer should discover wrapper and
  callback flow from SSA/value flow.

  Acceptance bar for “strategy based in concrete evidence”:

  - A standalone probe or test emits an InvocationTrace for Mattermost that names connectWebSocket as the region’s
    external session surface.
  - surface.Derive classifies the trace-derived surface as Session because connectWebSocket calls
    websocket.Upgrader.Upgrade.
  - A negative fixture proves unrelated websocket handlers are not attached to the Hub/WebConn region.
  - Toy fixtures cover generic callback shapes: handler wrapper, function stored in struct field then invoked,
    interface http.Handler, goroutine launch, route builder chaining.
  - The Mattermost probe stays under the existing resource budget.

  I would make the next sprint a report-first vertical slice: land pkg/compiler/invocation plus report output and
  tests, then wire surface derivation to consume InvocationTrace. Emission can remain deferred until the compiler
  can prove the invocation answer accurately.

