# Lift-utility rubric

A region is a candidate **useful lift region** if it would plausibly benefit from being moved to a separate compute resource and triggered remotely under load. This rubric guides the candidate search; agents must justify each candidate against it.

## Background — what Monolift lifts

Monolift takes a code region (function or interface method, today) annotated by the developer and compiles two artifacts: an in-process implementation and a remote-callable implementation. A runtime metric oracle (CPU, MEM, IPS thresholds, today) decides per call whether to invoke the local copy or dispatch to a remote replica that can be scaled independently. A lift only pays off if the region has work worth offloading and is loosely enough coupled to the rest of the binary that the remote dispatch is feasible.

## Five inclusion criteria

Each candidate should be scored **yes / maybe / no** on all five. A candidate with three or more `yes` and no `no` makes the corpus; one with any `no` should be excluded or downgraded with reasoning.

### 1. Compute envelope

The region does meaningful work per call. Examples that qualify:

- CPU-bound: parsing/serializing non-trivial payloads, hashing, encoding/decoding (image, video, JSON-of-records, archives), text processing (search, ranking, diff, syntax highlighting, markdown→HTML), template rendering, regex on large inputs, compression, crypto over more than a small constant.
- Memory-bound: building large in-memory indexes/snapshots, batch transforms over collections sized by tenant data.
- IO-bound *that scales with payload*: outbound network calls whose latency dominates and that aggregate well (webhook fan-out, search index pushes, batch DB writes), or filesystem-heavy transforms (archive build, attachment processing) that are not bound to local-only paths.

Negative: a function that does only a single tiny DB read and returns a struct. Negative: an O(1) constant-time helper.

### 2. Variable / spikable load

The call frequency is uneven across time, tenants, or input characteristics. Concretely:

- Bursty (webhook delivery on a noisy upstream, image processing on user upload, OAuth callback flurries during a campaign launch).
- Periodic but heavy (cron jobs that scan large tables, scheduled report renders, mailing list send campaigns).
- Per-tenant uneven (one big customer's feed refresh dominates).

Negative: uniformly low-traffic (e.g. once-a-day vacuum) — the lift overhead probably exceeds the benefit. Negative: uniformly hot but on the request critical path with sub-ms expected latency.

### 3. Coherent unit

The region is named, callable, and has a clear input/output contract. Concretely:

- A function or method with parameters that are values or interface-typed (no smuggling whole `*App` pointers as the only input).
- Side effects, if any, go through a documented interface (DB, queue, mailer, message bus) rather than mutating ad-hoc package globals.
- The region is implementable without dragging in 30% of the binary's dependency closure.

Negative: a 500-line function that branches on twelve flags reading global config, half of whose branches mutate package-level state.

### 4. State independence

The region does not depend on **pervasive in-process mutable state** that would be expensive or impossible to share with a remote replica. Concretely:

- Reads stable config or input parameters; writes go to durable stores (DB, queue, external API).
- Local caches (LRU, memo tables) are acceptable if they can be made replica-local without correctness loss.
- Per-call state (request, session) is OK if it is passed in as an argument.

Negative: holds a long-lived per-request resource (open WebSocket, SSE stream, gRPC server-stream). Negative: relies on goroutine-local state, leader election, or in-process pub-sub fan-out where every replica must see every event. Negative: mutates shared in-process maps as part of its primary effect.

### 5. Latency tolerance and failure model

Adding a network hop and a possible failure mode wouldn't break the user experience. Concretely:

- Caller is async or background (cron, queue worker, goroutine launched by a request).
- Caller is on the request path but the region's contribution is already O(100 ms) or more, so an extra hop is in the noise.
- The region has a natural retry/idempotency story (queue-backed, DB-transactional, end-to-end-checked).

Negative: caller is on a tight synchronous request path with strict p99 budget. Negative: failure has unbounded blast radius (e.g. the function is the only place a critical invariant is enforced and it must succeed atomically with a DB write).

## Disqualifiers (any one is sufficient to exclude)

- Holds an open per-request connection (WebSocket session, SSE stream, gRPC stream) for its full lifetime.
- Direct read/write of repo-local filesystem paths that cannot be virtualized (e.g. reads a config file by hardcoded path on every call).
- Relies on the goroutine being the same goroutine that holds a context-bound lock or transaction.
- Is itself the activation-handoff machinery of the framework (the dispatch loop, the cron scheduler, the mux) — these are infrastructure, not lift-able units.

## Output format per candidate

```markdown
### C-<n>: <short name>

- **Region root:** `<file:line>` — function/method name and one-line purpose.
- **Caller(s):** one or two file:line citations showing where this gets invoked.
- **Why useful (rubric scoring):**
  - Compute envelope: yes/maybe/no — one-line justification.
  - Load profile: yes/maybe/no — one-line justification.
  - Coherent unit: yes/maybe/no — one-line justification.
  - State independence: yes/maybe/no — one-line justification.
  - Latency / failure: yes/maybe/no — one-line justification.
- **Activation shape (informational, not a selection criterion):** one-line — e.g. "HTTP route handler", "cron-registered closure", "queue worker", "OS-signal goroutine".
- **Confidence:** high / medium / low — one line on what would change your mind.
- **Risk notes:** anything that would make the lift hard in practice (state coupling, large dependency closure, hidden synchronous contract).
```

## What NOT to include

- Generic "all HTTP handlers" — pick the *specific* handlers that are computationally meaningful.
- The framework's own dispatch loop or scheduler.
- Anything you cannot cite to a specific file:line you actually opened.
- Initialization functions whose work happens once at boot.
- Tests, fixtures, examples — only production code paths.

## Cross-pollination with SPRINT-0033

You may *re-pick* regions that appeared in `docs/research/runs/SPRINT-0033-lift-target-catalog.md` if (and only if) they pass this utility rubric on their own merits. Do not anchor on that catalog — its selection criterion was structural diversity, not utility. Justify each pick fresh.

## Calibration: positive examples

These are illustrative of the kind of region that should clearly land on the corpus (drawn from priors, not pre-validated):

- A feed-refresh function that fetches an external URL, parses RSS/Atom, diffs against stored entries, and writes deltas — variable per-feed cost, batch-dispatched, IO-bound but cleanly bounded inputs/outputs.
- A webhook-delivery worker that pulls a job, marshals payload, attempts HTTP POST with retries, records result — bursty fan-out, bounded state, naturally async.
- An OAuth-token-exchange handler that does an outbound HTTPS round-trip plus signature verification — bursty, compute-and-IO mixed, well-bounded inputs.
- An image-thumbnail or markdown-render endpoint that does CPU work proportional to input size — naturally CPU-heavy, easy to batch.

## Calibration: negative examples

These should be excluded:

- The HTTP server's `Accept` loop or the route mux's dispatch.
- A `init()` function that registers handlers.
- A WebSocket connect handler that holds the socket open and pumps events for the connection's lifetime.
- A function that takes a `*App` and reaches into ten of its fields to mutate them.
- A 12-line helper that wraps a single DB query.
