# SPRINT-0005 — v2 Pragma Parser + Harness Integration

**Status:** planned · **Scope:** parser + harness integration only, no extraction or codegen
**Primary deliverable:** in-place replacement of `pkg/compiler/pragma.go` with a v2 EBNF parser, wired into the SPRINT-0004 harness so `MLV2_PRAGMA_*` refusal codes flow through stages 3–4 on malformed fixtures
**Primary inputs:** `docs/specs/monolift-v2-contract.md` §Pragma Syntax v2 and §Refusal Diagnostic Index; `docs/sprints/SPRINT-0004.md`; `docs/specs/e2e-test-strategy.md`; ADR-0004 (annotation surfaces); ADR-0011 (harness-before-compiler)
**Prerequisite for:** SPRINT-0006+ v2 compiler epics (extraction, refusal-diagnostic framework, canonical-shape classifier, state-class inference)

---

## Why this sprint exists

The v2 contract (SPRINT-0003) defined a full pragma grammar. The harness
(SPRINT-0004) went green against a stub compiler with hand-forged closure
reports — nothing parses real Go source today. The pragma parser is the
**front door of the v2 contract**: extraction reads the annotated
declaration, its surface class, and its option set. Without a real parser,
every downstream compiler epic has nothing to consume except fixture JSON.

This sprint ships that parser, replaces v1 in place (breaking the v1 demo
codepath — acceptable, explicitly acknowledged), and completes the first
real-compiler hand-off into the harness: malformed pragma fixtures produce
`MLV2_PRAGMA_*` codes that stage 3/4 assertions see via the real parser,
not fixture JSON.

## Goals

- [x] Full v2 EBNF parser at `pkg/compiler/pragma.go` (in-place replacement of v1).
- [x] Doc-comment attachment validator (`ast.Decl.Doc`-only acceptance; scans `*ast.File.Comments` for misattached `monolift:` markers).
- [x] Per-annotation-surface key validator for interface / function / method / struct (encoded as a Go table, not prose).
- [x] `x-*` extension-key reservation honored on every surface; never produces `UNKNOWN_KEY` or `INVALID_KEY_FOR_SURFACE`.
- [x] Duplicate-pragma, unknown-verb, misattached detection.
- [x] v1-migration recognition of both `// @monolift ...` and `//monolift:offload ...`, emitting `MLV2_PRAGMA_V1_DEPRECATED` warnings with synthesized v2 rewrite suggestions (recovering `trigger=`/`metric=`/`threshold=` from v1 text).
- [x] Parser-internal diagnostic types — **never** `reportv2.Diagnostic`. Import-boundary enforced by a mechanical check.
- [x] Stub compiler invokes the real parser against target source; seven pragma-refusal micro-fixture rows flow through harness stages 3–4.
- [x] Harness-before-compiler discipline preserved: pragma-refusal fixture rows land red in Phase 2 (before parser exists); parser implementation flips them green.
- [x] ADR-0012 records the parser-internal vs `reportv2.Diagnostic` boundary (per-project convention from ADR-0011 precedent).

## Non-goals

- No SSA extraction, call-graph closure, state inference, adapter classification, codegen, or lifted deployable work.
- No `reportv2.Diagnostic` construction inside the parser — the stub compiler handles the one-shot translation at the harness seam. The refusal-diagnostic framework epic (SPRINT-0006+) owns the production version.
- No preservation of the v1 `demo/monolith/*` codepath — breakage is expected and explicit.
- No parser-side validation of rules that require canonical-shape knowledge. For example: `transport=handler` validity against an HTTP-shaped method, `mode=local + transport=grpc`, `state=affinity` requiring an `affinity` key. These are **deferred** with `// TODO(canonical-shape-epic)` markers — parser overreach here paints later epics into corners.
- No `pkg/compiler/compiler.go` / `artifacts.go` / `manifests.go` restructuring beyond minimal compatibility edits forced by the parser-API change.
- No changes to `reportv2` schema, `pkg/lift/*`, `pkg/pragma/*`, runtime metrics, or Kubernetes paths.
- No CI integration (SPRINT-0006+).

## Scope boundaries

**In scope:**
- `pkg/compiler/pragma.go` (full replacement)
- `pkg/compiler/pragma_test.go` (full replacement — v2 grammar coverage)
- Optional sibling files under `pkg/compiler/` as needed: `pragma_errors.go`, `pragma_keys.go`, `pragma_parse.go`
- `test/e2e/targets/pragma/fixtures/` — seven source-fixture directories, one per `MLV2_PRAGMA_*` code
- `test/e2e/targets/pragma/target.go` — `TargetCase` declarations
- `test/e2e/stubcompiler/main.go` — real-parser seam; source-directory plumbing
- `test/e2e/harness/verdict.go` — warning-severity diagnostic support (if not already generic)
- `test/e2e/e2e_test.go` — register pragma target cases
- `docs/decisions/0012-pragma-parser-diagnostics.md` — new ADR
- `docs/evolution.md` — closeout entry
- Minimal compatibility edits to `pkg/compiler` callers of v1 parser APIs

**Out of scope (do not touch unless a compile break forces it):**
- `pkg/compiler/reportv2/`, `pkg/compiler/compiler.go` body logic, `pkg/compiler/artifacts.go`, `pkg/compiler/manifests.go`
- `pkg/lift/*`, `pkg/pragma/*`, `pkg/metrics/*`
- `demo/`, `evaluation/`
- Any non-pragma `MLV2_*` diagnostic

---

## Tasks

All work is checkboxed. Phases are ordered. **Phase 2 (red-first fixtures) lands before Phases 3–6 (parser implementation)** per harness-before-compiler discipline (ADR-0011).

### Phase 0 — Setup, baseline, inventory

- [x] Record pre-sprint baseline: `go test ./pkg/compiler ./test/e2e/stubcompiler ./test/e2e/harness` and `MONOLIFT_E2E=1 make e2e`. Capture results; this is the diff anchor for the v1-demo break.
  - Baseline captured 2026-04-20: `go test ./pkg/compiler ./test/e2e/stubcompiler ./test/e2e/harness` failed in existing v1 `pkg/compiler/pragma_test.go` expectations; `test/e2e/stubcompiler` and `test/e2e/harness` passed. `MONOLIFT_E2E=1 make e2e` passed: Caddy and Pocketbase green; Miniflux/Listmonk/Gitea/Mattermost skipped.
- [x] Enumerate every in-repo caller of v1 parser APIs: `parsePragmaLine`, `getPragmasFromCommentGroup`, `getFuncDeclPragmas`, `GetTypeSpecPragmas`, `IsInterface`, `Pragma.Attributes`, `MonoliftPragmaPrefix`. The inventory determines Phase 7 compatibility-edit scope; unknown call sites = unknown breakage.
  - Inventory: `pkg/compiler/pragma.go`; `pkg/compiler/pragma_test.go`; `pkg/compiler/compiler.go` lines around the current AST walk (`getFuncDeclPragmas`, `IsInterface`, `getPragmasFromCommentGroup`, `Pragma.Attributes`). No other in-repo callers found.
- [x] Decide the smallest replacement API in package `compiler` (keep the package, not a new subpackage per the replace-in-place directive). Document exported names + signatures: `ParseLine`, `FromDecl`, `Parse` (over directories), plus constants.
  - API decision: `ParseLine(text string, basePos token.Position) (*Pragma, []Diagnostic)`; `FromDecl(decl ast.Decl, fset *token.FileSet) ([]*Pragma, []Diagnostic)`; `Parse(sourceDirs []string) ([]*Pragma, []Diagnostic, error)`; exported `Surface*`, `Severity*`, and `Code*` constants. `Parse` owns source-directory walking and AST file scanning; `reportv2` translation stays outside `pkg/compiler/pragma*`.
- [x] **Required-key diagnostic mapping decision.** The spec's §Refusal Diagnostic Index has no dedicated "missing required key" code. Decision: required-key failures emit `MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE` with a message naming the missing key, unless the spec is amended. Record this in ADR-0012.
- [x] **Duplicate-option-key-within-a-line decision.** The spec is silent. Decision: refuse as `MLV2_PRAGMA_PARSE`. Record in ADR-0012.
- [x] Open ADR-0012 stub at `docs/decisions/0012-pragma-parser-diagnostics.md` with Context + pending Decision sections.

### Phase 1 — Parser-internal types & spec-drift guard

- [x] Define `Surface` constants: `SurfaceInterface`, `SurfaceFunction`, `SurfaceMethod`, `SurfaceStruct`, `SurfaceUnknown`.
- [x] Define v2 `Pragma` struct: `Name`, `Surface`, `Options map[string]string`, source span, raw text, attached declaration identity.
- [x] Define `Diagnostic` parser-internal type: `Code` string, `Severity` (`error` | `warning`), `Message`, `Span`, optional `Suggestion`. Separate type from `reportv2.Diagnostic`.
- [x] Define exported string constants for every `MLV2_PRAGMA_*` code: `CodeParse`, `CodeUnknownKey`, `CodeInvalidKeyForSurface`, `CodeMisattached`, `CodeDuplicate`, `CodeUnknownVerb`, `CodeV1Deprecated`. Values exactly match §Refusal Diagnostic Index spelling.
- [x] Add span helpers converting `token.Pos` + comment byte offsets to line/offset spans. No import of `reportv2`.
- [x] **Spec-drift regex test.** Unit test reads `docs/specs/monolift-v2-contract.md`, regex-extracts `MLV2_PRAGMA_*` entries from the Refusal Diagnostic Index table, compares against the parser's `Code*` constants. Fails loudly if either side drifts.
- [x] **Import-boundary mechanical check.** Add a test (or `go vet`-style script in `pkg/compiler/pragma_guard_test.go`) that fails if `pkg/compiler/pragma` imports `pkg/compiler/reportv2`. Documents the boundary.
- [x] **Test-brittleness guidance.** Assert diagnostic code + severity + useful line/offset *ranges* — not byte-perfect spans (except where parser math is the subject of the test).

### Phase 2 — Red-first harness fixtures *(lands before parser implementation)*

- [x] Create `test/e2e/targets/pragma/fixtures/` with seven subdirectories, one per code, each a minimal Go package:
  - [x] `parse/` — syntactically broken pragma (unterminated quote, invalid escape)
  - [x] `unknown-key/` — unknown non-`x-` key
  - [x] `invalid-key-for-surface/` — `methods=` on a function pragma
  - [x] `misattached/` — `//monolift:lift` as trailing/separated comment, not `Doc`
  - [x] `duplicate/` — two `//monolift:lift` on one declaration
  - [x] `unknown-verb/` — `//monolift:retire` or similar
  - [x] `v1-deprecated/` — `// @monolift` + `//monolift:offload` usage (warning-only)
- [x] Add at least one fixture exercising each of the four annotation surfaces.
- [x] Add `test/e2e/targets/pragma/target.go` exposing one `harness.TargetCase` per fixture: `ExpectedVerdict: "refuse-blocking"` (or `"accept-with-warnings"` for v1-deprecated), `StopAtStage: 4`, `RequiredDiagnostics` listing the code.
- [x] Register pragma target cases in `test/e2e/e2e_test.go`. Do NOT enable stages 5–10.
- [x] **Do not add hard-coded `closure-report.json` golden files for pragma cases.** These rows must depend on parser output; golden-as-fixture defeats the sprint's purpose.
- [x] Verify: `MONOLIFT_E2E=1 make e2e` now shows the seven pragma rows **red** (stub compiler ignores source). This is the red-first baseline.
  - Red-first captured 2026-04-20: Caddy/Pocketbase still passed; existing deferred targets skipped; all seven `pragma-*` rows failed at stage 3 with missing stub fixture output because source dirs are not yet propagated/parsed.

### Phase 3 — EBNF lexer + parser

- [x] `ParseLine(text string, basePos token.Position) (*Pragma, []Diagnostic)` implements the EBNF from §Pragma Syntax v2:
  - [x] Line-comment detection: only `//monolift:lift ...` is a v2 pragma (doc-comment attachment is Phase 4's job)
  - [x] Whitespace-separated options; spaces and tabs; no newline continuation
  - [x] `key = ident { "." ident | ":" ident }` with letters, digits, `_`, `-` in each segment
  - [x] `option = key "=" value` — **valueless flags (v1 style) MUST be `MLV2_PRAGMA_PARSE`**
  - [x] Bare values: letters, digits, `_`, `-`, `.`, `/`, `:`, `,`
  - [x] Quoted values with escapes `\"`, `\\`, `\n`, `\t`
  - [x] `MLV2_PRAGMA_PARSE` on: unterminated quotes, invalid escapes, missing `=`, empty keys, non-ASCII in bare values, trailing garbage, duplicate option keys on one line
- [x] Emit `MLV2_PRAGMA_UNKNOWN_VERB` for `//monolift:<verb>` where verb ≠ `lift`.
- [x] Ignore ordinary comments with no Monolift prefix.
- [x] Unit tests: every EBNF production with ≥1 positive and ≥1 negative case. Worked examples from spec §Worked Pragma Examples must round-trip. Quoted values with embedded `=` (e.g. `policy="trigger=CPU threshold=0.70"`) must work. `x-*` extension keys must parse.

### Phase 4 — AST attachment + surface classification

- [x] `FromDecl(decl ast.Decl, fset *token.FileSet) ([]*Pragma, []Diagnostic)` — inspects `decl.Doc` only for accepted pragmas.
- [x] Scan `*ast.File.Comments` for stray `monolift:` markers outside any `Decl.Doc` group; emit `MLV2_PRAGMA_MISATTACHED`.
- [x] Emit `MLV2_PRAGMA_DUPLICATE` when more than one `//monolift:lift` attaches to a single declaration.
- [x] Surface classification from AST shape:
  - [x] `*ast.GenDecl` + `*ast.TypeSpec` + `*ast.InterfaceType` → `SurfaceInterface`
  - [x] `*ast.GenDecl` + `*ast.TypeSpec` + `*ast.StructType` → `SurfaceStruct`
  - [x] `*ast.FuncDecl` with `Recv == nil` → `SurfaceFunction`
  - [x] `*ast.FuncDecl` with `Recv != nil` → `SurfaceMethod`
  - [x] Anything else (vars, consts, imports, anonymous funcs, generic instantiation expressions, type aliases) → `MLV2_PRAGMA_PARSE` with message identifying the unsupported decl kind
- [x] **Misattachment-detector scope:** only scan comment tokens — never string literals. `monolift:` in a `*ast.BasicLit` value must not fire the detector.
- [x] Unit tests: table-driven `ast.File` fixtures covering each surface; trailing/floating/separated comment variants for misattachment; duplicate lifts; doc-group vs line-comment ambiguity.

### Phase 5 — Per-surface key matrix validation

- [x] Encode the per-surface key matrix in `pragma_keys.go` as a Go table: `map[Surface]map[string]KeyRule{Allowed, Required, ValueValidator}`. The matrix rows come from spec §Keys and Requirements.
- [x] Validation pass over `(Surface, Options)`:
  - [x] `MLV2_PRAGMA_UNKNOWN_KEY` for unknown non-`x-` keys
  - [x] `MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE` for `impl=` / `methods=` on function pragmas and other surface violations (see matrix)
  - [x] `MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE` for missing required keys (per Phase 0 decision)
  - [x] `MLV2_PRAGMA_PARSE` for malformed values of known keys (e.g. `mode=lolwat`, `state=nonsense`) — enum-checked
- [x] `x-*` keys: accept on all four surfaces, retain verbatim in `Pragma.Options`, never diagnose.
- [x] **Defer canonical-shape checks.** Do NOT flag `mode=local + transport=grpc`, `state=affinity` missing `affinity` key, or `transport=handler` on non-handler shapes as parser-level errors. These require canonical-shape knowledge and belong in a later epic. Leave `// TODO(canonical-shape-epic)` markers with explanatory comments. Parser tests must assert the parser does NOT overreach into these checks.
- [x] Unit tests: every matrix row with valid + invalid cases; each worked example from spec §Worked Pragma Examples parses + validates clean; `x-vendor.key` + `x-caddy.mode="custom"` accepted on all four surfaces.

### Phase 6 — v1 migration warnings

- [x] Recognize `// @monolift ...` in any comment position (not just doc groups).
- [x] Recognize `//monolift:offload ...` identically.
- [x] Parse enough v1 syntax to recover `trigger=` / `metric=` / `threshold=` values for suggestion synthesis.
- [x] Emit `MLV2_PRAGMA_V1_DEPRECATED` as a **warning** (severity = warning, not refusal).
- [x] **A v1 pragma alone MUST NOT produce an accepted v2 `Pragma` object** (per PS-MIGRATE-1).
- [x] Synthesize v2 rewrite suggestion text: `//monolift:lift name=<required-by-user> mode=dynamic policy="trigger=<metric> threshold=<value>"` when v1 supplies enough. When v1 doesn't supply `policy=` inputs, suggestion omits `policy=` and instructs user to add one.
- [x] Unit tests: both v1 forms; `trigger` vs `metric` aliases; missing threshold; mixed v1 + v2 on same decl (warning for v1, accepted pragma for v2).

### Phase 7 — Stub-compiler seam

- [x] Extend `harness.TargetCase` to expose `SourceDirs` (list of directories to pass to the compiler executable). Extend the e2e compiler-invocation path so these propagate via `--source=<dir>` flags (or equivalent).
- [x] Modify `test/e2e/stubcompiler/main.go`:
  - [x] Add a source-walk helper: iterate `.go` files under `SourceDirs`, skip vendor/generated/test directories, parse with `go/parser.ParseComments`, invoke `pragma.Parse`
  - [x] Translate parser-internal `Diagnostic` → `reportv2.Diagnostic` at this seam only (this is the one place the translation lives)
  - [x] Build pragma-only `closure-report.json` with minimal root / closure / state / adapter / pruning scaffolding, populating `pragma` section and `diagnostics` array from parser output
  - [x] Verdict logic: `refuse-blocking` when diagnostics contain any `error`-severity pragma code; `accept-with-warnings` when only `MLV2_PRAGMA_V1_DEPRECATED` is present
- [x] **Do NOT churn existing Caddy / Pocketbase / Miniflux goldens.** Preserve their current stub behavior. If the real parser legitimately diverges from their fixture `pragma` section (e.g. option normalization), surface via `make e2e-update-golden` with reviewer sign-off — but prefer fixture stability in this sprint.
- [x] `harness/verdict.go::AssertRefuse` / `AssertAccept`: extend to recognize warning-severity diagnostics distinctly from refusals (if current generic behavior is insufficient). Preserve existing `[stage=N target=X kind=compiler]` failure-message prefixes.
- [x] Integration test: with Phases 1–6 in place, the seven pragma target rows flip from red → green.
  - Flip captured 2026-04-20: `MONOLIFT_E2E=1 make e2e` passed; Caddy and Pocketbase remained green; all seven `pragma-*` rows passed; existing deferred targets remained skipped.

### Phase 8 — Compatibility touches, verification, handoff

- [x] Update v1-parser callers identified in Phase 0 inventory. Minimum edits — leave v1 demo codepath broken per explicit non-goal; do NOT reintroduce compatibility parsing.
- [x] Run `go test ./pkg/compiler/...` — all green.
- [x] Run `go test ./test/e2e/stubcompiler ./test/e2e/harness` — all green.
- [x] Run `MONOLIFT_E2E=1 make e2e` (if Kind available): Caddy + Pocketbase still green, seven pragma rows now green, Miniflux/Listmonk/Gitea/Mattermost still skipped. If Kind unavailable, record that e2e was unreachable and log the unit-level verification commands.
- [x] Grep check: `rg -n "@monolift|monolift:offload|monolift:lift" pkg/compiler test/e2e` — v1 syntax only in migration-test files or the accepted `v1-deprecated` fixture.
- [x] Grep check: no file under `pkg/compiler/pragma*` imports `pkg/compiler/reportv2` (this should already be enforced by the import-boundary test from Phase 1).
- [x] Finalize ADR-0012: Decision + Consequences sections covering parser-internal diagnostic model, required-key mapping, duplicate-key-in-line refusal, import boundary.
- [x] Append a closeout entry to `docs/evolution.md`: v1-in-place replacement, new parser surface, pragma-refusal harness rows, deliberate v1-demo break.
- [x] Add to `demo/monolith/README.md` (or top-level README): "v1 demo pragma parsing is intentionally broken as of SPRINT-0005; see the SPRINT-0006+ extraction/codegen epics for restoration."
- [x] Append `## SPRINT-0006 Seed Epics` to the bottom of this sprint file (seed list below).
- [x] Set ledger: `python3 .claude/skills/sprint-planner/scripts/ledger.py set-status SPRINT-0005 done`. Not run in this session per orchestrator instruction: do not modify the ledger.

---

## Sequencing

Strict: **Phase 0 → 1 → 2 → {3, 4, 6 parallel after API is stable} → 5 → 7 → 8.**

- Phase 2 lands **before** Phases 3–6 (harness-before-compiler — pragma rows are red until parser exists).
- Phase 5 depends on Phases 3–4 (key validation reads surface + options).
- Phase 6 (v1 migration) is independent of Phase 4 (attachment) but depends on Phase 3 (grammar) for the v1 value recovery.
- Phase 7 stub rewiring requires Phases 1–6 API to be stable.
- Phase 8 is closeout — only after all tests pass.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Parser overreaches into canonical-shape semantic checks | Phase 5 explicitly defers with `// TODO(canonical-shape-epic)` markers; parser tests assert non-overreach |
| `reportv2.Diagnostic` leaks into `pkg/compiler/pragma` | Phase 1 import-boundary mechanical check; ADR-0012 asserts the boundary |
| V1-parser replacement breaks more than the demo | Phase 0 caller inventory enumerates scope before implementation; compatibility edits bounded by inventory |
| Spec constants drift between parser and spec markdown | Phase 1 spec-drift regex test fails loudly on divergence |
| Misattachment detector fires on `monolift:` substrings in code string literals | Detector scans `*ast.File.Comments` only, never `*ast.BasicLit` |
| EBNF ambiguity around quoted values with embedded `=` | Phase 3 explicit unit test for `policy="trigger=CPU threshold=0.70"` shape |
| Duplicate-option-key-in-line semantics differ from downstream consumer expectations | Phase 0 decision recorded in ADR-0012 (refuse as PARSE); easy to flip if later epic needs different semantics |
| Byte-perfect span assertions cause brittle tests | Phase 1 guidance: assert code + severity + useful ranges, not byte-perfect spans |
| Surface-inference gaps on unusual decls (generic funcs, type aliases) | Refuse safely with `MLV2_PRAGMA_PARSE` and an unsupported-decl-kind message rather than silently accept |
| Warning-only v1 cases don't fit current harness verdict model | Phase 7 extends `AssertRefuse`/`AssertAccept` to distinguish warning severity; Phase 2 fixture for v1-deprecated exercises this |
| Existing Caddy/Pocketbase goldens churn unnecessarily | Phase 7 prefers fixture stability; golden updates require reviewer sign-off |
| V1-demo breakage interpreted as regression | README note + evolution.md entry + explicit sprint non-goal pre-empts confusion |

## Acceptance criteria

- [x] `pkg/compiler/pragma.go` content entirely reflects v2 grammar; no references to `MonoliftPragmaPrefix` or v1 `Pragma.Attributes` shape remain.
- [x] Package `compiler` exports a v2 parser API (names chosen in Phase 0 after caller inventory).
- [x] `pkg/compiler/pragma*` does not import `pkg/compiler/reportv2` — verified by the import-boundary test.
- [x] Spec-drift regex test passes: parser `Code*` constants exactly match §Refusal Diagnostic Index entries.
- [x] Every EBNF production has ≥1 positive and ≥1 negative unit test.
- [x] Every `MLV2_PRAGMA_*` code is producible by the parser and covered by a parser unit test.
- [x] Every spec §Worked Pragma Examples entry parses and validates clean.
- [x] `x-vendor.anything=value` accepted on all four annotation surfaces.
- [x] v1 `// @monolift ...` and `//monolift:offload ...` produce `MLV2_PRAGMA_V1_DEPRECATED` warnings with synthesized v2 rewrite suggestions; neither produces an accepted v2 pragma.
- [x] Seven `test/e2e/targets/pragma/fixtures/*` rows run stages 0–4 green, one per `MLV2_PRAGMA_*` code, using the real parser (no fixture JSON).
- [x] Existing Caddy and Pocketbase e2e behavior is preserved (goldens unchanged, or updated with reviewer sign-off).
- [x] `go test ./pkg/compiler ./test/e2e/stubcompiler ./test/e2e/harness` passes.
- [x] `MONOLIFT_E2E=1 make e2e` passes when Kind is available, or unreachable-reason is recorded in closeout.
- [x] ADR-0012 committed covering parser-internal diagnostic model, required-key mapping, duplicate-option-key decision, import boundary.
- [x] `docs/evolution.md` closeout entry records parser landing + deliberate v1-demo break.
- [x] V1 demo codepath breakage documented in `demo/monolith/README.md` (or top-level).
- [x] This sprint file ends with a `## SPRINT-0006 Seed Epics` section.

---

## SPRINT-0006 Seed Epics

_Finalized in Phase 8 closeout. Seed candidates derived from the SPRINT-0003 follow-on list, updated for what SPRINT-0005 now unblocks:_

- **SSA-based extraction pass** consuming parsed pragma output (root decl, surface, options) and producing `reportv2.Root` + `reportv2.Closure` + `reportv2.ExternalDeps`. Harness flip: Caddy closure section moves from stub fixture to real compiler output.
- **Refusal-diagnostic framework.** Takes ownership of the parser-internal `Diagnostic` → `reportv2.Diagnostic` translation (moves the seam out of the stub compiler), source-span formatting with remediation text, and implements the remaining §Refusal Diagnostic Index codes. Harness flip: Pocketbase refusal codes start being raised by real compiler code instead of stub.
- **Canonical-shape signature classifier + per-shape adapter templates.** Picks up the parser's deferred checks (`transport=handler` validity, `mode=local + transport=grpc`, `methods=` exposed subset).
- **State-class inference + singleton/affinity codegen.**
- **V1 demo repair or retirement.** Decide whether to port `demo/monolith/*` to v2 pragmas or retire it as historical.
- **End-to-end miniflux smoke.** Removes `t.Skip`; exercises Postgres + RSS fixtures through stages 0–10 against real compiler.
