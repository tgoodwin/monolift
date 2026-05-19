# SPRINT-0052 Target Survey

**Status:** provisional analytical pre-selection. The CloudLab admission sweep
called for by Phase 0.2/0.3 has not yet been run on the build node; the picks
below are based on source-level inspection of the corpus and the analyses
under `docs/research/activation-paths/analyses/`. The sweep must run before
Phase 4 begins and may invalidate either pick — backups are documented.

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
  However, the *next level deeper* — `(*SMTPSender).Send(from string,
  to []string, msg io.WriterTo) error` — is a strong candidate for a new
  `writerto_input` pattern (see below).

## Provisional picks

### Target #2: `listmonk/countLines(r io.Reader) (int, error)`

- **Project:** listmonk (reuses the SPRINT-0051 Postgres + auth e2e fixture)
- **Source:** `evaluation/listmonk/internal/subimporter/importer.go`
- **Signature:** `func countLines(r io.Reader) (int, error)`
- **Body:** allocates a 32 KiB buffer; reads in a `for { r.Read(buf) }` loop
  counting `\n` bytes; returns on `io.EOF`. Bounded sequential consumption,
  exactly the `reader_read_all` shape.
- **Receiver:** none (package-level function).
- **Boundary data class:** parameters: `io.Reader` (currently
  `unsupported_param_shape`); return: `(int, error)` trivial.
- **State class:** `Stateless`.
- **Pattern exercised:** `reader_read_all` (new). The host buffers the
  reader to `[]byte` via `io.ReadAll`, ships bytes inline (≤ 8 MiB ceiling
  for now), and the normalized helper wraps with `bytes.NewReader(input)`
  before counting.
- **Oracle policy:** **direct-equality on the returned `(int, error)` tuple.**
  Workload submits CSV byte payloads of known line counts via the listmonk
  bulk-import endpoint; oracle compares the integer count returned by the
  extracted service to a host-computed reference.
- **First runnable stage:** 4 (compile-only of the rewritten helper). Stage
  ladder 4 → 10 mirrors SPRINT-0051's M-4 ladder.
- **Expected e2e package:** `test/e2e/targets/activation_listmonk_countlines/`.

### Target #3 (preferred): `gitea/(*SMTPSender).Send(from string, to []string, msg io.WriterTo) error`

- **Project:** gitea
- **Source:** `evaluation/gitea/services/mailer/sender/smtp.go`
- **Signature:** `func (s *SMTPSender) Send(from string, to []string, msg io.WriterTo) error`
- **Body:** opens net connection, sets up TLS, runs SMTP protocol,
  `msg.WriteTo(client.Data())` to stream message bytes.
- **Receiver:** `*SMTPSender` zero-sized struct (`type SMTPSender struct{}`)
  — trivial reconstruction.
- **Pattern exercised:** `writerto_input` (new). The host calls
  `msg.WriteTo(&buf)` to materialize the message bytes, ships the bytes
  inline, the normalized helper rebuilds an `io.WriterTo` via a small
  `bytes.Buffer` wrapper.
- **Distinct from target #2:** yes — input is an `io.WriterTo` interface,
  not a raw `io.Reader`. Host transformation is *push* (WriteTo) vs target
  #2's *pull* (Read).
- **Caveat:** the gitea recommended trace cuts at `send` (one level up).
  Picking `(*SMTPSender).Send` requires an extended trace OR a fresh
  fixture-level cut. If extending the trace is not viable, fall back to
  target #3 alternate below.
- **Oracle policy:** direct-equality on the returned `error` plus a
  byte-level comparison of the bytes that flowed through `WriteTo`. The
  workload uses gitea's mail-test admin endpoint with a known message;
  oracle records bytes written to a captive SMTP listener (no real SMTP
  server in the cluster).

### Target #3 alternate: `gitea/escapeStream(in io.Reader, out io.Writer)`

- **Source:** `evaluation/gitea/modules/charset/escape_stream.go`
- **Signature:** `func escapeStream(locale translation.Locale, in io.Reader, out io.Writer, opts ...EscapeOptions) (*EscapeStatus, error)`
- **Pattern exercised:** mixed `reader_read_all` (in) + `writer_buffer_back`
  (out). Host buffers `in` via `io.ReadAll`, ships bytes, receives
  transformed bytes back, writes them to `out`.
- **Distinct from target #2:** partially. Input side reuses
  `reader_read_all`; output side is a new `writer_buffer_back` pattern.
  Acceptable per the plan if the new output pattern carries the falsification
  weight.
- **Caveat:** `translation.Locale` parameter likely requires its own
  reconstructor — needs investigation.

### Target #3 backup #2: `miniflux/opml.parse(data io.Reader) ([]subscription, error)`

- **Source:** `evaluation/miniflux/internal/reader/opml/parser.go`
- **Pattern exercised:** `reader_read_all` only — overlaps target #2.
- **Use only if both target #3 picks are infeasible**, and re-pick target
  #2 with a non-overlap distinct pattern.

## Rejected candidates

- `pocketbase/M-9` `safeFileFromURL`: return type `*filesystem.File`
  wraps bytes — structurally similar to SPRINT-0051's `bytes_reader_return`
  (struct-with-Reader-inside). Not distinct enough.
- `mattermost/M-13` `sendBatchedEmailNotification`: heavy `*Service`
  receiver with `userService`, `store`, `config`, `license` — receiver
  reconstruction would dominate; no new adapter pattern in the cut body.
- `miniflux/M-9` `SendEntry`: directly liftable once the integration
  package's transitive imports are wrangled — no adapter pattern needed.
  May be a good admission improvement target separately.
- `listmonk/ExportSubscribers`: returns a callback iterator with closure
  state (prepared stmt, tx). Excluded by Phase 0.8 selection rule (d) —
  "refuse … callback values stored for later use … callbacks requiring
  reverse invocation."

## What remains for Phase 0 (deferred to CloudLab)

The Phase 0 admission sweep with `MONOLIFT_BOUNDARY_ADAPTER=1` and
widened caps is the falsification layer for these picks: if the sweep
shows that `listmonk/countLines` (or its parents) is already admitted by
the current pipeline, or that an adapter-eligible refusal points at a
different shape, the picks above need revision before Phase 4 begins.
The sweep should run on the `monolift-buildserver` experiment, focused on
phase-5 traces, with artifacts under
`.moab/runs/sprint-0052-survey/`.

## Oracle determinism check (Phase 0.9)

Both selected targets are deterministic byte-in / byte-out under fixed
input:

- `countLines`: pure function on the input bytes; identical input
  produces identical line count.
- `(*SMTPSender).Send`: the SMTP protocol bytes are deterministic given
  the message + a stub SMTP listener that records bytes; the real network
  side effects are stubbed in the e2e harness.

The SPRINT-0051 per-target determinism check template applies as-is.
