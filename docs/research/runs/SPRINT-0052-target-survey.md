# SPRINT-0052 Target Survey

**Status:** **CONFIRMED via CloudLab focused-admission sweep (2026-05-19).** The
sweep falsified the original reader/writer/callback picks and the survey's own
caveat (lines below) proved correct: `listmonk/countLines` is already admitted
by the current pipeline, so it is *not* a new-adapter target. After reviewing
the results with the maintainer, the sprint's "two new adapter patterns" thesis
was **broadened** to "two lifts that prove the framework's *generic* machinery
generalizes beyond M-4" (decision recorded below). Final picks are at the top;
the original analytical pre-selection is retained underneath for the record.

## Final picks (confirmed admitted on CloudLab)

> **Re-selection note (route-reachability is the binding e2e constraint).** An
> end-to-end test must *exercise the target function through the host's real
> request path* so the cross-network round-trip (monolith → remote service →
> back) is genuinely demonstrated. The first reclassified picks (`countLines`,
> `classifyBounce`) failed this: `classifyBounce` is reachable *only* from the
> POP3 bounce-mailbox scanner (no HTTP route at all), and a survey of fresh
> listmonk candidates showed the route-reachable ones are nearly all methods on
> `SharedState` receivers (`*App`, `*Campaign`) that refuse with
> `receiver_requires_reconstruction`. Re-selected with route-reachability +
> free-function (or trivial-receiver) as the first filter, across cost-feasible
> apps. Both final picks are confirmed admitted **and** invoked synchronously on
> an HTTP request path, so the workload can drive the real round-trip.

### Lift #1: `miniflux/ExtractContent(page io.Reader) (baseURL, content string, err error)` — streaming-bytes + DTO

- **Source:** `evaluation/miniflux/internal/reader/readability/readability.go:73`
- **Sweep result:** `ADMITTED: ExtractContent (boundary params: 1, reconstructed: 0, results: 3)`.
- **Route round-trip:** `GET /v1/entries/{id}/fetch-content` → `fetchContentHandler`
  → `processor.ProcessEntryWebPage` → `scraper.ScrapeWebsite` → `ExtractContent`;
  the returned `content` surfaces as `entry.Content` in the JSON response.
- **Mechanisms proven (two at once):** `io.Reader` param → `CodecStreamingBytes`
  (streaming-bytes), and two non-error string returns → ResultDTO packing (task
  2.1 refusal-shadow gate). Free function (no receiver). Different app from M-4.
- **Oracle:** direct-equality on the returned `(baseURL, content)` for a fixed HTML page.

### Lift #2: `pocketbase/S256Challenge(code string) string` — plain transform, third app

- **Source:** `evaluation/pocketbase/tools/security/crypto.go:18`
- **Sweep result:** `ADMITTED: S256Challenge (boundary params: 1, reconstructed: 0, results: 1)`.
- **Route round-trip:** `GET /api/collections/{collection}/auth-methods` →
  `recordAuthMethods` calls `security.S256Challenge(info.CodeVerifier)` per
  PKCE-enabled OAuth2 provider; result surfaces as `codeChallenge` in the JSON
  response. (Workload must seed a PKCE OAuth2 provider in the fixture.)
- **Mechanism proven:** a clean free-function pure transform (`string → string`)
  lifting + round-tripping in a **third** app (pocketbase) — breadth across apps
  and shapes (M-4 adapter / miniflux streaming+DTO / pocketbase plain).

### Confirmed-but-not-chosen (kept as backups)

- `listmonk/countLines(io.Reader)(int,error)` — streaming, route-reachable via
  `POST /api/import/subscribers` → `importer.go:474`. Solid backup if either pick stalls.
- `miniflux/EstimateReadingTime(string,int,int) int` — plain transform via `PUT /v1/entries/{id}`.

### Refuted in re-selection (route-reachable but not liftable as-is)

- `listmonk/exportSubscriberData` — method on `*App` (SharedState) → `receiver_requires_reconstruction`.
- `listmonk/ConvertContent` — method on `*Campaign` (SharedState) → `receiver_requires_reconstruction`.
- `listmonk/getI18nLang` — `missing recommended cut` (interface param / shape).
- `listmonk/classifyBounce` — POP3-only, no HTTP route ⇒ cannot demonstrate the round-trip.

## CloudLab sweep results (2026-05-19, focused admission, `MONOLIFT_BOUNDARY_ADAPTER=1`)

Run via `go test ./pkg/codegen -run TestAdmission -args -trace-target=<file:line>
-source-dir=evaluation/<proj>` on the build node (`GOTOOLCHAIN=auto`, since the
pinned listmonk/gitea corpus declares `go 1.26.1`).

| Candidate | Result | Interpretation |
|---|---|---|
| `listmonk/countLines` (importer.go:717) | **ADMITTED** (1 param, 2 results) | `io.Reader` handled by streaming-bytes codec — *not* an adapter case |
| `gitea/escapeStream` (escape_stream.go:36) | **timeout @ 10m** (activation augmentation) | gitea activation cost-prohibitive for an e2e target |
| `gitea/(*SMTPSender).Send` (smtp.go:27) | **timeout @ 10m** | same; also `io.WriterTo` is not adapter-eligible |
| `listmonk/GetTplSubject` (notifs.go:106) | **ADMITTED** (2 params, 2 results) | multi-return DTO |
| `listmonk/classifyBounce` (pop.go:214) | **ADMITTED** (1 param, 2 results) | multi-return DTO |

### Why no second *adapter-pattern* target was selected

Mining the 72-trace manifest (`activation_corpus_traces.yaml`) for adapter-eligible
refusals (`unsupported_boundary_data`, `unsupported_result_shape`,
`unsupported_param_shape`, `callable_boundary_values`, `missing_reconstructor`)
yields only: `gitea/M-17` (cost-prohibitive), `pocketbase/M-2` (`*core.RequestEvent`
— a rich request context, not a bounded-consumed value), and `pocketbase/M-5`/`M-11`
(`core.App` interface — a live-proxy callback shape, not drain-to-bytes). Only **one**
trace in the entire corpus is classified `AdapterPossible` — `listmonk/M-4` itself.

The adapter's sweet spot (an awkward, non-serializable boundary value that the
function consumes in a *bounded* way, so it can be drained to a wire value and
reconstructed host-side) is genuinely rare in this corpus. The two shapes that
superficially look like candidates are already resolved elsewhere: `io.Reader`
by the streaming-bytes codec, and `io.Writer`/callbacks by `live_proxy_required`
(not adapter-eligible). M-4's `*multipart.FileHeader` was the standout case.

**Decision (maintainer-approved):** rather than force a weak or cost-prohibitive
adapter target, prove generalization through the framework's generic machinery on
two fresh non-M-4 functions (streaming-bytes + multi-return DTO). A genuinely new
adapter *pattern* remains a future-sprint item if a clean candidate surfaces.

---

## Original analytical pre-selection (retained for the record)

> The sections below were the source-level pre-selection made before the sweep.
> The sweep refuted the reader/writer picks; they are kept to document the
> reasoning and the (correct) self-caveat at the end.

## Brief's proposed candidates — skepticism check

### `pocketbase/M-7` — `(*SMTPClient).send(m *Message) error`

- **Brief label:** "callback shape" → adapter target.
- **Actual signature:** receiver `*SMTPClient` carries config-only fields
  (TLS, Port, Host, Username, Password, AuthMethod, LocalName) plus an
  `onSend *hook.Hook[*SendEvent]` hook *consumed by `(*SMTPClient).Send`,
  not by the inner `send`*. The recommended cut at `send` does not exercise
  the hook.
- **Boundary data class** (per analysis): `Reconstructible`. Param `*Message`
  is a config-like serializable struct.
- **Verdict:** **rejected.** This is structurally a *direct lift* once a
  receiver reconstructor for `*SMTPClient` is added (`ConfigOnly` state).
  No new adapter pattern is exercised — the brief's "callback shape" label
  is incorrect for this cut point.

### `gitea/M-13` — `func send(sender Sender, msg *Message) error`

- **Brief label:** "queue handler with `io.Reader`" → `reader_read_all`.
- **Actual signature:** parameter `sender Sender` is an *interface*
  implemented by `SMTPSender`, `SendmailSender`, `DummySender`. Parameter
  `msg *Message` calls `msg.ToMessage()` to get a `*gomail.Message`, then
  calls `sender.Send(from, to, m)` where the third arg is `io.WriterTo`.
- **Verdict:** **rejected as recommended cut.** `send` itself does no
  `io.Reader` work; it dispatches via the `Sender` interface. To lift `send`
  the remote side would have to know which `Sender` impl to instantiate —
  that is a receiver reconstructor problem, not an adapter pattern.

## Provisional picks (REFUTED by the sweep)

### Target #2: `listmonk/countLines(r io.Reader) (int, error)`

- Proposed pattern: `reader_read_all` (new). **Refuted:** the sweep showed
  `countLines` is admitted directly via the streaming-bytes codec — the
  `io.Reader` param never reaches the adapter path. Retained as Lift #1 above,
  reclassified as a *streaming-bytes* (generic-machinery) target.

### Target #3 (preferred): `gitea/(*SMTPSender).Send(...)` — REFUTED (cost)

- Proposed pattern: `writerto_input` (new). **Refuted:** gitea activation times
  out at 10m, making it cost-prohibitive as an e2e target. (`io.WriterTo` is also
  not adapter-eligible.)

### Target #3 alternate: `gitea/escapeStream(...)` — REFUTED (cost)

- Proposed: `reader_read_all` + `writer_buffer_back`. **Refuted:** gitea timeout;
  `io.Writer` output is `streaming_type` (not adapter-eligible).

### Target #3 backup #2: `miniflux/opml.parse(...)` — not pursued

- `reader_read_all` only — would overlap Lift #1's streaming-bytes shape.

## Rejected candidates (unchanged)

- `pocketbase/M-9` `safeFileFromURL`: `*filesystem.File` return wraps bytes —
  structurally similar to SPRINT-0051's `bytes_reader_return`. Not distinct.
- `mattermost/M-13` `sendBatchedEmailNotification`: heavy `*Service` receiver —
  reconstruction would dominate; no new adapter pattern in the cut body.
- `miniflux/M-9` `SendEntry`: directly liftable; no adapter pattern needed.
- `listmonk/ExportSubscribers`: callback iterator with closure state — excluded
  by Phase 0.8 rule (d).

## Oracle determinism check (Phase 0.9)

Both final picks are deterministic byte-in / value-out under fixed input:

- `countLines`: pure function on the input bytes; identical input → identical count.
- `classifyBounce`: pure classification on the input bytes; identical input →
  identical `(string, string)`.

The SPRINT-0051 per-target determinism check template applies as-is.
