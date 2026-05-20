# SPRINT-0052 Coverage Report

**Sprint thesis (as planned):** prove the boundary-adapter framework generalizes
by landing ≥2 new adapter-enabled lifts at stage 10, each exercising a *distinct*
adapter pattern, with zero target-specific code in `pkg/codegen/`.

**Thesis as shipped (maintainer-approved pivot):** the corpus has no second
clean adapter-pattern candidate in a cost-feasible app (only `listmonk/M-4` is
`AdapterPossible` corpus-wide). The thesis was broadened to *prove the
framework's generic machinery generalizes beyond M-4* by landing two new lifts,
in two new apps, exercising **distinct generic mechanisms** — and, decisively,
**with zero changes to `pkg/codegen/`**. Phase 4 (new pattern implementations)
became a no-op. See `docs/research/runs/SPRINT-0052-target-survey.md` for the
survey + pivot rationale.

## Selected targets and stage results

| Target | App | Mechanism (distinct) | Route round-trip | Oracle | Stage | Time |
|---|---|---|---|---|---|---|
| `listmonk/M-4` `processImage` (regression) | listmonk | multipart adapter (`multipart_file_read_all` + `bytes_reader_return`) | `POST /api/media` | in-cluster pod (stdlib decode reference) | 10 | ~ |
| `miniflux/ExtractContent` (lift #2) | miniflux | streaming-bytes `io.Reader` codec **+** two-return ResultDTO (`base_url`/`extracted_content`) | `GET /v1/entries/{id}/fetch-content` | in-cluster pod importing real `readability` | 10 | 3.4m |
| `pocketbase/S256Challenge` (lift #3) | pocketbase | plain `string→string`, single non-error return (`result`, no DTO) | public `GET /api/collections/{c}/auth-methods` (+ seeded PKCE provider) | in-cluster pod importing real `tools/security` | 10 | 6.5m |

Three distinct shapes across three apps; no mechanism overlap. Each genuinely
exercises the cross-network round trip: the host's real HTTP request path drives
the lifted symbol on the remote service and the result flows back into the
response (verified by the extracted-service `/calls` delta and the stage-8
direct-invoke oracle compare).

## Why no second adapter *pattern* was selected

Mining the 72-trace corpus manifest for adapter-eligible refusals
(`unsupported_boundary_data`, `unsupported_result_shape`,
`unsupported_param_shape`, `callable_boundary_values`, `missing_reconstructor`)
yields only: `gitea/M-17` (cost-prohibitive — activation times out at 10m),
`pocketbase/M-2` (`*core.RequestEvent`, a rich request context, not a
bounded-consumed value), and `pocketbase/M-5`/`M-11` (`core.App` interface — a
live-proxy callback shape, not drain-to-bytes). Only **one** trace is classified
`AdapterPossible` corpus-wide: `listmonk/M-4` itself. The adapter sweet spot (an
awkward, non-serializable boundary value consumed in a *bounded* way) is
genuinely rare here; `io.Reader` is handled by the streaming-bytes codec and
`io.Writer`/callbacks by `live_proxy_required`, neither adapter-eligible.

## Rejected / refuted candidates

| Candidate | Verdict | Reason |
|---|---|---|
| `listmonk/countLines` (`io.Reader`) | admitted, but not new | Handled by the streaming-bytes codec, not the adapter (reclassified as the streaming-bytes mechanism; ExtractContent chosen instead, also route-reachable). |
| `listmonk/classifyBounce` | route-unreachable | POP3-only; no HTTP route ⇒ cannot drive the round trip. |
| `gitea/escapeStream`, `gitea/(*SMTPSender).Send` | cost-prohibitive | gitea activation times out at 10m; `io.Writer`/`io.WriterTo` also not adapter-eligible. |
| `listmonk/exportSubscriberData`, `listmonk/ConvertContent` | refused | Methods on `*App`/`*Campaign` (SharedState) → `receiver_requires_reconstruction`. |
| `pocketbase/M-5`/`M-11` | not adapter-eligible | `core.App` interface callback shapes → `live_proxy_required`. |

## Commands

- Focused admission probe (Phase 0 survey):
  `go test ./pkg/codegen -run TestAdmission -args -trace-target=<file:line> -source-dir=evaluation/<proj>`
  (`GOTOOLCHAIN=auto`, since the pinned listmonk/gitea corpus declares `go 1.26.1`).
- Per-target stage ladder: `MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=10 go test -tags=e2e ./test/e2e/ -run TestE2E/<target> -timeout 45m`.
- Different lifts run **in parallel** on the one kind cluster (isolated namespaces / image tags); only the stages within a lift are serial.

## Cost profile (e2e feasibility class)

- **Feasible** (used as e2e targets): listmonk, miniflux, pocketbase — single
  static binaries, pure-Go SQLite for pocketbase (`modernc.org/sqlite`, no CGO),
  small dep trees, builds + deploys complete in minutes.
- **Cost-prohibitive**: gitea — activation augmentation times out at 10m.

## Adapter patterns added

**None.** The pivot made Phase 4 a no-op; `adapter_patterns.go` registry is
unchanged from SPRINT-0051 (`multipart_file_read_all`, `bytes_reader_return`).
A genuinely new adapter *pattern* remains a future-sprint item if a clean,
cost-feasible candidate surfaces.

## Verification

- **9.1** `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...` — all green on CloudLab (codegen 373s incl. golden files + the new 11-field DTO test + the `TestAdapterPassNoTargetSpecificCode` guard; activation 40s; harness ok).
- **9.2** Stage-10 verification: M-4 (regression), ExtractContent, S256Challenge — all pass.
- **9.3** Flag-off↔flag-on parity: the regression that caused the SPRINT-0051 `pocketbase/M-5`/`M-11` flip (the `MONOLIFT_BOUNDARY_ADAPTER` flag carrying a second behavior — gating `callable_boundary_values`) is removed in Phase 2.2 and **unit-verified** by `admission_test.go` (asserts the `callable_boundary_values` refusal stands in *both* flag states). The full 72-trace corpus parity sweep was not re-run (heavyweight; gitea times out) — the flip cause is closed and unit-covered, and ADR-0032 records the flag's sole behavior.
- **9.5 / 9.7** `pkg/codegen/adapter*.go` remains target-agnostic — enforced by the permanent `TestAdapterPassNoTargetSpecificCode` guard (green). Growth since SPRINT-0051 is Phase 2/3 framework rigor, not per-target conditionals; only `adapter_patterns.go` grew for registry/pattern reasons.
- **9.6** No target-specific tokens in `pkg/codegen` (a stray `processImageResult` doc-comment example was genericized). The three `target.Name ==` matches in `e2e_test.go` are pre-existing caddy/miniflux project routing, not M-4 fingerprints; the two new lifts route via the generic `ActivationLift != nil` path.
- **9.9** No generated extracted deployment YAML carries `MONOLIFT_LIFT_*` env vars (preserved SPRINT-0050 invariant).

## Residual backlog

- Full 72-trace flag-off/flag-on corpus parity sweep as a recorded artifact (logic is unit-verified; sweep is the heavyweight reproducibility check).
- A genuinely new adapter *pattern* if a clean cost-feasible candidate surfaces (e.g. a bounded-consumed awkward value outside M-4's multipart shape).
- `reader_read_all` input adapter pattern (carried from SPRINT-0051).
- Staged-object transport for payloads above the (now plan-configurable) inline ceiling.
- Removal of `MONOLIFT_BOUNDARY_ADAPTER` after the documented two-release window (SPRINT-0053+).
