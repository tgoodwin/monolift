# SPRINT-0052 Target Survey

**Status:** **CONFIRMED via CloudLab focused-admission sweep (2026-05-19).** The
sweep falsified the original reader/writer/callback picks and the survey's own
caveat (lines below) proved correct: `listmonk/countLines` is already admitted
by the current pipeline, so it is *not* a new-adapter target. After reviewing
the results with the maintainer, the sprint's "two new adapter patterns" thesis
was **broadened** to "two lifts that prove the framework's *generic* machinery
generalizes beyond M-4" (decision recorded below). Final picks are at the top;
the original analytical pre-selection is retained underneath for the record.

## Final picks (confirmed admitted on CloudLab, ~46s activation each)

Both are **listmonk** (cost-feasible; gitea activation times out at 10m — see
below) and **reuse the M-4 processimage e2e harness** (Postgres + listmonk
container + direct-invoke). Neither needs a new adapter pattern or any new
`pkg/codegen` code — they exercise mechanisms that already exist but that, today,
**only `listmonk/M-4` exercises end-to-end**. None of the 9 passing corpus e2e
targets use streaming-bytes or multi-return DTO.

### Lift #1: `listmonk/countLines(r io.Reader) (int, error)` — streaming-bytes codec

- **Source:** `evaluation/listmonk/internal/subimporter/importer.go:717`
- **Sweep result:** `ADMITTED: countLines (boundary params: 1, reconstructed: 0, results: 2)`.
- **Mechanism proven:** the `io.Reader` parameter is classified `CodecStreamingBytes`
  (`isStreamingReader` → true for `io.Reader/ReadSeeker/ReadCloser`) and admitted
  directly — the client serializes the reader's content to `[]byte` in the JSON
  request and the service rehydrates it. This is the **streaming-bytes codec**,
  not an adapter. It is the reason the original `reader_read_all` adapter premise
  was a mirage: `io.Reader` never reaches the adapter path.
- **Oracle:** direct-equality on the returned `int` line count for fixed CSV byte input.

### Lift #2: `listmonk/classifyBounce(b []byte) (string, string)` — multi-return DTO packing

- **Source:** `evaluation/listmonk/internal/bounce/mailbox/pop.go:214`
- **Sweep result:** `ADMITTED: classifyBounce (boundary params: 1, reconstructed: 0, results: 2)`.
- **Mechanism proven:** two non-error returns `(string, string)` trigger generic
  **ResultDTO packing** — the path governed by the SPRINT-0052 task 2.1
  refusal-shadow gate (`baseResultShapeRefusal`: `len==2 && !hasError` ⇒ pack).
  Proves DTO result-shaping generalizes beyond M-4's `([]byte,int,int,error)`.
- **Oracle:** direct-equality on the returned `(string, string)` tuple for fixed input.
- **Alternative considered:** `GetTplSubject(string, []byte) (string, []byte)` (also
  confirmed admitted, 2 params / 2 results) — same DTO mechanism; `classifyBounce`
  chosen for the simpler signature.

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
