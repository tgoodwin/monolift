 Read all four docs plus the current bootpath.Walk and surface.Derive. Now I see why you're stuck: today's "boot
  path" is just union ∪ {main} walked forward for literals/env/Getenv. There is no reverse edge from a region root
  to its caller, and surface.Derive only inspects the root method's own body — so it can't see that (*WebConn).Pump
  is reachable only because connectWebSocket upgrades the request and calls Register. That's the missing piece, and
  it's a straight callgraph problem, not a pattern-matching problem.

  Recommendation, in one sentence: build a whole-program SSA callgraph (RTA or VTA from
  golang.org/x/tools/go/callgraph), do reverse BFS from each region-root *ssa.Function to main.main / package-init,
  and along the way track function-value escape via SSA def-use — every *ssa.Function or *ssa.MakeClosure reference,
   followed forward to where it gets stored, passed, or invoked. Two generic primitives, no taxonomy:

  1. Reverse reachability answers "how is this region invoked?" — the union of minimal reverse paths is your
  invocation surface; the leaves are the external entry points (functions reachable from main whose only forward
  link into the region is a function-value handoff or interface dispatch).
  2. Function-value flow answers "how is it registered?" without naming http.HandleFunc — when an *ssa.Function
  flows into a callee parameter whose static type is an interface (e.g. http.Handler) or whose target stores it in a
   long-lived field/global, that call site is the registration. The wire-side stub goes there.

  The promise test, evidence-first, is cheap: extend pkg/compiler/extract/bootpath (or a sibling entrypath package)
  to compute callgraph + reverse BFS for one region, run it on the existing Mattermost overlay, and check whether
  the path from (*Hub).Start / (*WebConn).Pump lands at connectWebSocket and then at a call whose argument is a
  function value of http.Handler-shaped type — without any string match on "websocket", "Mux", or "HandleFunc" in
  the analyzer. If RTA recovers that path on Mattermost-scale SSA in the existing 55s/2.4 GB budget, the strategy is
   real. If RTA blows up or loses precision through the chi/mux indirection, that's the signal to try VTA or to add
  a narrow, principled escape tracker rather than a taxonomy. Want me to sketch the package layout and a falsifiable
   success criterion for that probe before any code lands?

