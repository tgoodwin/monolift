# SPRINT-0023 — Boot-path extraction + RegionPatchRequest + stream-proxy emitter (Mattermost machinery)

**Status:** done — branch (R)
**Anchor ADRs:** ADR-0017 (admission), ADR-0018 (frozen unless additive), ADR-0022 (composite regions), ADR-0023 (cmd-inside-host emission), ADR-0024 (multi-root pragma), ADR-0025 (compiler-derived distribution shape).
**Predecessors:** SPRINT-0021 (region-granularity cliff → C), SPRINT-0022 (multi-root pipeline + composite + region admission → R at G.gate-1, blocked on liftpatch API).

## Intent

SPRINT-0023 lands three pieces of compiler machinery that together close SPRINT-0022's emission gap and unlock lifting regions with long-lived state:

1. **Surface-derivation pass.** Implement the ADR-0025 v0 — derive surface category (Call / Session / refused-AsyncProducerConsumer) from the region's external entry points. Required by both stream-proxy emission and boot-path scoping.
2. **`RegionPatchRequest` API.** Additive extension to `liftpatch` per the SPRINT-0022 emission-gap doc, supporting multi-symbol/multi-receiver patches. Legacy `PatchSymbolBody` retained byte-identical for SPRINT-0019/0020 targets.
3. **Boot-path extraction pass.** Reverse SSA walk from `main` toward each lifted region's roots, capturing initialization context (long-lived goroutine launches, dependency-init order, externalized config sources). Output drives the extracted service's `main` and Kubernetes manifests.
4. **Stream-proxy emitter.** Session-surface v0 wire protocol per ADR-0025 — host-side hijack-and-tunnel that preserves the external API surface and bridges raw bytes between client and extracted service.

Mattermost Hub/WebConn is the forcing target. Acceptance bar is **buildable extracted-service binary**, not kind-cluster e2e — the four-layer e2e on a websocket workload is deferred to SPRINT-0024 with a clean blast radius.

## Scope choice — option (ii)

Three options on the table. All three drafts converged on **(ii)**: boot-path + RegionPatchRequest + stream-proxy, ending with buildable artifacts. Reasoning:

- **(i)** Boot-path alone leaves SPRINT-0022's known emitter blocker unresolved; the boot-path data model is unfalsifiable without a downstream consumer.
- **(iii)** Full Mattermost kind-cluster e2e stacks three new pipeline stages plus a websocket fanout workload plus baseline boot (an unproven Mattermost cliff from SPRINT-0021). SPRINT-0022's branch (S) already encoded this ambition and missed.
- **(ii)** Each piece has a downstream consumer: boot-path feeds emission; emission calls the new patcher API; the patcher API replaces SPRINT-0022's blocker. Acceptance is `go build` on host + extracted-service + oracle. Kind e2e is SPRINT-0024.

## Design decisions

### Pipeline placement

`pragma → closure-union → seam-detection → stateclass → admission → surface-derivation → boot-path → emission`.

Surface-derivation runs **before** boot-path so the boot-path walk knows which entry points are surface-relevant — walking a dep chain for an entry point the surface pass will refuse wastes work, and `LiteralSource` refusal can be specialized per surface category. Boot-path does not inform admission; admission decides "boundary semantics admit a wire," boot-path decides "host's initialization can be reconstructed for the extracted service." A boot-path refusal sets `emittable: false` with a `BootPathRefusal` rationale; it does not retroactively reject admission.

### Inter-pass contract

Three passes feed emission via a typed aggregator:

```go
type RegionPlan struct {
    Region   *Region
    Closure  Closure        // union from SPRINT-0022
    Surface  RegionSurface  // from §Surface-derivation
    Boot     BootSpec       // from §Boot-path
}
```

`RegionPatchRequest` carries only patch-relevant metadata; `Surface` and `Boot` stay in `RegionPlan` and are consumed by the emitter, which then calls the patcher. Patcher does not know boot semantics. Emission is a pure function of `RegionPlan`; if any input is missing or refused, emission produces a `RegionPatchResult{Refused: ..., Rationale: ...}` rather than partial files.

### Externalized config representation

Typed `ConfigSource` evidence values, with deployability fields for Kubernetes manifest emission:

```go
type ConfigSource interface{ isConfigSource() }
type EnvSource     struct { Name, Default string; Required bool;            SSAOrigin token.Pos }
type FlagSource    struct { Name, Default string; Required bool; FlagSet string; SSAOrigin token.Pos }
type FileSource    struct { Path string; Format FileFormat; MountName string; Required bool; SSAOrigin token.Pos }
type LiteralSource struct { Value string;                                    SSAOrigin token.Pos } // const / hardcoded
type DBSource      struct { Name string; QueryShape string; Required bool;   SSAOrigin token.Pos } // report-only v0
```

Manifest emission rule (per source kind) — implemented in the streamproxy/httpjson manifest helpers:
- `EnvSource` → Deployment env passthrough; Secret reference if name matches the conservative classification rule (substring match on `PASSWORD` / `SECRET` / `TOKEN` / `KEY` / `CREDENTIAL` / Mattermost datasource names like `MM_SQLSETTINGS_DATASOURCE`).
- `FlagSource` → command args appended to container args, sourced from same ConfigMap/Secret.
- `FileSource` → ConfigMap (or Secret if path/name matches classification rule) + volumeMount at the same path.
- `LiteralSource` is recorded but does not emit a manifest delta (literals bake at build time).
- `DBSource` is **report-only** in v0; if Mattermost requires it for correctness, sprint lands (R) with a characterized config gap.

Dependency-init list orders the extracted service's `main`. Each entry classified as `required` (must be invoked in the extracted service), `substitutable` (the host's instance becomes the extracted service's own client; e.g., Postgres datasource), or `disabled-by-minimal-config` (e.g., plugin manager). Conservative default: `required` if the analyzer can't classify, with a refusal diagnostic if `required` reaches an unportable shape.

### `RegionPatchRequest` shape

Additive API. `PatchSymbolBody` retained byte-identical for SPRINT-0019/0020 single-function targets.

```go
type RegionPatchRequest struct {
    RegionName           string
    Symbols              []PatchSymbolRequest
    SharedGeneratedFiles []GeneratedFile
}

type PatchSymbolRequest struct {
    PackageImportPath string
    PackageDir        string
    File              string
    FuncName          string
    ReceiverType      string  // empty = bare function (legacy shape)
    ExpectedSignature string
    Prelude           PreludeSpec
    SentinelIdent     string  // computed: hash(RegionName + PackageImportPath)
    GeneratedFiles    []GeneratedFile
}

type RegionPatchResult struct {
    Files          []PatchedFileResult       // per-file: path, OriginalSHA256, PatchedSHA256, AddedImports
    GeneratedFiles []GeneratedFileResult
    Refused        *RegionPatchRefusal
}
```

Routing rule (in `pkg/compiler/transport/emit/emit.go`): regions with `len(Roots) > 1` OR any root with non-empty `ReceiverType` → `PatchRegion`; everything else → `PatchSymbolBody`. Determined statically from `Region` shape. The `DiagnosticMethodReceiver` rejection is removed inside `PatchRegion` only; `PatchSymbolBody` keeps its existing rejection so its contract is unchanged.

### Stream-proxy emitter mechanism

Hijack-and-tunnel via `http.Hijacker`. **Not** `httputil.ReverseProxy` — fights raw-byte ownership; Go-stdlib edge cases around `Connection: Upgrade` and HTTP/2. **Not** `gorilla/websocket` in the host stub — would force frame-level round-tripping, losing byte-for-byte bridging.

Host stub template (replaces the original handler):

1. Forward original HTTP upgrade request (method + URL + headers + body) to the extracted service's internal endpoint over plain TCP. Cookies, `Authorization`, `X-Requested-With`, `X-Forwarded-*`, and Mattermost session headers preserved verbatim.
2. Hijack the inbound conn via `http.Hijacker`.
3. Read upgrade response from extracted service, write back to inbound conn.
4. Two `io.Copy` goroutines (inbound→outbound, outbound→inbound) coupled by a shared `*sync.WaitGroup` + `context.CancelFunc`. First side to error closes both.
5. Connection lifetime: either side closes → both goroutines unblock → both conns close → extracted service's pump goroutines see EOF and exit naturally.
6. Failure modes: dial failure → 503 (fail-closed default) or run original in-host handler if `MONOLIFT_LIFT_FAILMODE=open` (fail-open mirrors SPRINT-0019).

Extracted-service binary binds the same route on its own port and runs the original handler unchanged. Auth flows naturally because the extracted side sees the original cookies/headers in the forwarded upgrade request. Internal Service is **not** externally exposed — only the host dials it.

`httputil.ReverseProxy` is rejected as a documented non-choice this sprint. If a future fixture proves it's needed (handler hasn't already taken ownership of the connection), it lands as a Block-D follow-up under hypothesis-doc discipline.

## Frozen boundaries

- `cmd/main.go` byte-identical (no exceptions).
- `evaluation/mattermost/` byte-identical pre/post compile.
- `syscall.Flock` startup guard preserved.
- SPRINT-0017 Caddy `alternative_set` goldens byte-identical.
- SPRINT-0019/0020/0022 e2e (caddy + miniflux + pocketbase + pragma + multi-root machinery) byte-identical after any rule changes.
- ADR-0018 Layer-1 properties unchanged; no new `liftability.PropertyID` constants.
- `PatchSymbolBody` API surface unchanged (additive-only — `RegionPatchRequest` is a new sibling).

May move under documented hypothesis (per SPRINT-0021/0022 discipline checklist (a)–(e)): admission rules (default expectation: zero changes — SPRINT-0022 seam-shape check already accepted Hub/WebConn), surface-derivation pass internals (new), boot-path pass internals (new), liftpatch API (additive `RegionPatchRequest`), `pkg/compiler/transport/emit/streamproxy/` (new directory).

## Anticipated cliffs

| # | Cliff | Symptom | Stop |
|---|---|---|---|
| 1 | **`RegionPatchRequest` rewrite leaks beyond `pkg/compiler/transport/emit/`.** Downstream consumers in `extract/`, `e2ecompile/` assume single-symbol patcher results in a way that doesn't survive. | A.gate-1 wiring run breaks contracts in >2 packages outside `emit/`. | (C). Cliff doc enumerates the contract surface. |
| 2 | **Boot-path SSA walk OOMs at Mattermost scale.** SPRINT-0021 single-Hub-root closure was 88s/2GB; reverse walk from `main` plus dep-init traversal might blow up. | B.gate-1 closure-only boot-path probe exceeds 30 min wall / 16 GiB RSS. | (C). Reuse SPRINT-0021 cliff-doc shape. |
| 3 | **`cmd/mattermost/main.go` boot chain goes through an unportable shape** — closure capturing host-only state, plugin-loading reflection, etc. | F.gate-1 finds `BootSpec.Refusals` non-empty after honest walk. | (R). Characterize per refusal (fundamental vs tooling-immaturity per SPRINT-0021 framing). |
| 4 | **Stream-proxy `http.Hijacker` mechanism doesn't compose with Mattermost's route binding** — custom handler stack, plugin-injected middleware. | D toy fixture works but Mattermost route binding doesn't fit the host-stub template. | (R) — characterize, not (C). Toy fixture proves the mechanism; Mattermost-shape gap is documented follow-up. |
| 5 | **Auth-context cannot be preserved by cookie/header forwarding alone** — some authn state lives in server memory or host-only globals. | F.x reveals authenticated session can't reconstitute on the extracted side. | (R). Document auth-context transfer gap; do not fake authentication in the extracted service. |
| 6 | **Config source is database-backed or runtime-mutated.** Required Hub/WebConn config can't be represented as env/flag/file. | F.x finds Hub.Start dependency surfaces a `DBSource` with `Required: true`. | (R). Record required config shape and proposed representation. |
| 7 | **Secret-vs-ConfigMap classification is unsafe.** Datasource credentials would be emitted as plaintext ConfigMap/env. | C.x manifest renderer would emit a credential without Secret reference. | Stop manifest emission until Secret classification is fixed. Not a sprint-stop — a within-block hard gate. |
| 8 | **Generated extracted service drags too much host-only init.** Plugin manager, webapp, or server-global initialization not needed by Hub/WebConn. | F.x extracted-service `go build` pulls unportable global state. | (R). Document dependency-minimization gap. |
| 9 | **`FileSource` config representation collides with how Mattermost loads `config.json`** — in-process struct, not just a path. | C.x manifest emitter can't render a ConfigMap that yields the same in-memory struct on the extracted side. | (R). Document; consider whether `FileSource` needs a `Loader` discriminator. |
| 10 | **Hijack mechanism fundamentally doesn't compose with Go's `net/http` server** (e.g., HTTP/2 conns aren't hijackable on the version we use). | D.gate-1 toy fixture fails. | (C). Cliff doc + reproduction. |

## Sequencing

Block 0 → A → B → C → D → E → F → G → H, strict.

Cheapest kills in order: **A.gate-1** (RegionPatchRequest fits without spilling beyond `emit/`), **B.gate-1** (boot-path SSA walk completes under budget), **D.gate-1** (hijack mechanism composes), **F.gate-1** (Mattermost attempt either produces buildable artifacts → S, or refuses with characterization → R).

### Block 0 — Surface-derivation pass (ADR-0025 minimal landing)

Goal: implement minimum surface-derivation for Call/Session detection. AsyncProducerConsumer refused with diagnostic per ADR-0025.

- [x] **0.1** Create `pkg/compiler/surface/` package. Types: `SurfaceCategory`, `RegionSurface`, `EntryPoint`, `WireProtocol`.
- [x] **0.2** Implement `Derive(region *Region, closure Closure) (RegionSurface, error)`. Detection:
    - Every entry point function-shaped with marshalable args+result → `Call`.
    - Any entry point body calls `(*websocket.Upgrader).Upgrade()`, `http.Hijacker.Hijack()`, or exposes `net.Conn` → `Session`.
    - Any entry point with channel-passing in arg/return → refuse with `MLV2_SURFACE_ASYNC_UNSUPPORTED`.
    - Mixed categories within one region → refuse with `MLV2_SURFACE_MIXED`.
- [x] **0.3** Wire pass into pipeline at `pkg/compiler/extract/extract.go` between admission and emission.
- [x] **0.4** Unit fixtures: caddy `CleanPath` → Call; miniflux `EstimateReadingTime` → Call; pocketbase pragma → Call; toy hijack handler → Session; toy `func(in <-chan A)` → refused. Caddy/miniflux/pocketbase classifications must match what implicitly drives `httpjson` today.
- [x] **0.5** SPRINT-0019/0020/0022 e2e byte-identical (surface-derivation on existing targets is a no-op semantically — same wire-protocol selection).

**Block 0 gate:** existing targets classify as Call and select httpjson exactly as today; toy hijack fixture classifies as Session.

### Block A — `RegionPatchRequest` API addition

Goal: region-patcher API lands without breaking `PatchSymbolBody`. **First sprint gate.**

- [x] **A.1** Hypothesis doc at `docs/research/runs/SPRINT-0023-liftpatch-api-extension.md`: shape per §RegionPatchRequest, why additive-only, which legacy fixtures must remain byte-identical.
- [x] **A.2** Add `RegionPatchRequest`, `PatchSymbolRequest`, `RegionPatchResult`, `PatchedFileResult`, `GeneratedFileResult`, `RegionPatchRefusal` types in `pkg/compiler/transport/emit/liftpatch/types.go`.
- [x] **A.3** Implement `Patcher.PatchRegion(req RegionPatchRequest) (RegionPatchResult, error)`: per-symbol prelude rendering, multi-file patch coordination, deterministic per-package sentinel computation (`SentinelIdent = fmt.Sprintf("monolift_%s_sentinel", hash(RegionName, PackageImportPath))`).
- [x] **A.4** Method-receiver support: drop `DiagnosticMethodReceiver` rejection inside `PatchRegion` only. `PatchSymbolBody` keeps its rejection. Methods patched by `(ReceiverType, FuncName, ExpectedSignature)` triple.
- [x] **A.5** Routing in `pkg/compiler/transport/emit/emit.go`: regions with `len(Roots) > 1` OR any root with non-empty `ReceiverType` → `PatchRegion`; everything else → `PatchSymbolBody`.
- [x] **A.6** Toy multi-symbol multi-receiver fixture under `pkg/compiler/transport/emit/liftpatch/testdata/region/`: two methods on two receiver types in one package, plus a shared generated client file.
- [x] **A.7** Toy multi-package fixture: two receivers in two different packages. Sentinel uniqueness asserted.
- [x] **A.8** Negative tests: duplicate symbol identities, signature mismatch, generated-file collision, method receiver mismatch.
- [x] **A.gate-1** **Wire `PatchRegion` into the call site that consumes patcher results in `pkg/compiler/extract/`.** If contract collision spans more than 2 packages outside `emit/`, stop at Cliff 1: write the cliff doc and land (C).
- [x] **A.9** SPRINT-0019/0020/0022 fixtures byte-identical — `PatchSymbolBody` path unchanged.

**Block A gate:** toy multi-root region patches successfully; legacy targets byte-identical; sentinel collision impossible by construction.

### Block B — Boot-path extraction pass

Goal: reverse SSA walk from `main` toward each region root, producing `BootSpec`. OOM probe before Mattermost.

- [x] **B.1** Hypothesis doc at `docs/research/runs/SPRINT-0023-bootpath-design.md`: walk algorithm, what counts as a `ConfigSource` SSA-wise, what counts as a goroutine launch overlapping the union, how the walk is bounded.
- [x] **B.2** Create `pkg/compiler/extract/bootpath/` package. Types: `BootSpec`, `ConfigSource` interface (Env/Flag/File/Literal/DB), `DependencyInit` (with `required`/`substitutable`/`disabled-by-minimal-config` classification), `GoroutineLaunch`, `BootPathRefusal`.
- [x] **B.3** Implement `Walk(prog *ssa.Program, mainPkg *ssa.Package, region *Region, surface RegionSurface, union Closure) (BootSpec, error)`. Entry path: SSA reachability from `main.main` to each surface entry point, deduplicated. Detection:
    - `os.Getenv("X")` / `os.LookupEnv("X")` → `EnvSource{X}`.
    - `flag.String("name", ...)` / `flag.Var(...)` → `FlagSource{name}`.
    - `os.Open` / `os.ReadFile` with constant string operand → `FileSource{Path, MountName}`.
    - String literals reachable along the path → `LiteralSource`; if value matches host-only-filesystem-path heuristic → `BootPathRefusal{Kind: UnportableLiteralPath}`.
    - DB-config reads (`sql.DB.QueryRow` flowing into Hub/WebConn fields) → `DBSource{Required: true}` (report-only emission v0).
    - `*ssa.Go` whose callee or captured values appear in `union.Functions` → `GoroutineLaunch`.
    - Constructors/factories on the entry path that produce values consumed by the region → `DependencyInit`.
- [x] **B.4** Dependency classification: walk known Mattermost-shaped patterns (`platform.NewService`, `app.New`, `server.New`, `HubsStart`) — classify Postgres datasource `substitutable`, plugin manager `disabled-by-minimal-config`, file-store `required-iff-reachable`. Conservative default: `required` if unclassifiable.
- [x] **B.5** Determinism: `BootSpec.ConfigSources` sorted by source kind then identifier; `EntryPath` sorted by SSA function ID; refusals sorted likewise.
- [x] **B.6** Toy fixtures under `pkg/compiler/extract/bootpath/testdata/`:
    - (a) `main` reads one env var, calls one constructor, launches one goroutine writing to a region channel → expected: 1 EnvSource, 1 DependencyInit, 1 GoroutineLaunch.
    - (b) literal file path `/etc/host-only-state` → `BootPathRefusal{UnportableLiteralPath}`.
    - (c) `flag.String` → `FlagSource`.
    - (d) `os.Open` constant path → `FileSource`.
    - (e) goroutine outside the union closure → not recorded (negative).
    - (f) datasource credential env name → reaches Block C's Secret classification.
- [x] **B.7** Steal the Gemini idea: pull a real but small fixture from `evaluation/caddy/` for a boot-path unit test (one of caddy's existing config-loading paths) — better signal than synthetic-only fixtures.
- [x] **B.8** Round-trip `BootSpec` through reportv2 (extend schema additively; new section, not changing existing fields).
- [x] **B.gate-1** **Mattermost OOM probe.** Run boot-path against the SPRINT-0022 Mattermost overlay (Hub + WebConn roots). Budget: **30 min wall / 16 GiB RSS** (mirrors SPRINT-0022 C.gate-1). Capture metrics into `docs/research/runs/SPRINT-0023-bootpath-probe.md`. If exceeded, (C).
- [x] **B.9** SPRINT-0019/0020/0022 e2e byte-identical (boot-path runs but its output doesn't change emission for Call-surface regions yet).

**Block B gate:** boot-path produces deterministic `BootSpec` for toy fixtures; OOM probe under budget; legacy targets unchanged.

### Block C — `BootSpec` → Kubernetes manifest emission

Goal: `BootSpec` translates to manifest deltas the extracted-service Deployment can carry. **Secret classification is a hard within-block gate (Cliff 7).**

- [x] **C.1** In `pkg/compiler/transport/emit/manifest/` (shared between streamproxy and httpjson where useful): `RenderConfig(boot BootSpec) (envEntries []corev1.EnvVar, configMap *corev1.ConfigMap, secret *corev1.Secret, volumeMounts []corev1.VolumeMount, args []string)`.
- [x] **C.2** Per-source rendering rule per §Externalized config representation (env passthrough, ConfigMap/Secret heuristic, volumeMount for files, args for flags).
- [x] **C.3** Secret-vs-ConfigMap classification: substring match on `PASSWORD` / `SECRET` / `TOKEN` / `KEY` / `CREDENTIAL` (case-insensitive) plus an explicit Mattermost datasource name list including `MM_SQLSETTINGS_DATASOURCE`. Anything matched → Secret reference, never plaintext ConfigMap or env value.
- [x] **C.gate-1** Test that a `BootSpec` containing `EnvSource{Name: "MM_SQLSETTINGS_DATASOURCE"}` renders a Secret reference, not a plaintext value. If this assertion fails, **stop manifest emission** — do not proceed to Block D until classification is correct.
- [x] **C.4** Toy fixture: `BootSpec` with one of each source kind renders deterministic manifest deltas. Golden in `pkg/compiler/transport/emit/manifest/testdata/`.
- [x] **C.5** Round-trip stability: same `BootSpec` always renders byte-identical manifests.
- [x] **C.6** Static recursion-safety: extracted-service Deployment grep-clean for `MONOLIFT_LIFT_[A-Z_]+:` env keys (no compiler-emitted-Kubernetes-eats-its-own-tail).

**Block C gate:** toy `BootSpec` → manifest fixture passes; Secret classification correct; recursion-safety asserted; sort/key ordering deterministic.

### Block D — Stream-proxy emitter (toy session-surface fixture)

Goal: hijack-and-tunnel mechanism works on a toy session-surface fixture before touching Mattermost.

- [x] **D.1** Hypothesis doc at `docs/research/runs/SPRINT-0023-streamproxy-design.md`: host-stub template per §Stream-proxy mechanism, why not `httputil.ReverseProxy`, why not `gorilla/websocket`, how disconnects propagate, fail-closed/open semantics.
- [x] **D.2** Create `pkg/compiler/transport/emit/streamproxy/` package, sibling to `httpjson/`.
- [x] **D.3** Implement `Emitter.EmitHostStub(plan RegionPlan) ([]PatchSymbolRequest, error)`: per-entry-point host-side stub body (hijack + dial + io.Copy pair) as `PreludeSpec`. One `PatchSymbolRequest` per session-surface entry point.
- [x] **D.4** Implement `Emitter.EmitExtractedMain(plan RegionPlan) (GeneratedFile, error)`: produces `cmd/monolift-extracted-<region>/main.go` that (a) imports the union closure, (b) replays `BootSpec.DependencyInits` in classified order (skip disabled-by-minimal-config; substitute substitutable; require required), (c) binds the same routes as `RegionSurface.EntryPoints` on a configurable port, (d) replays `BootSpec.GoroutineLaunches` to spawn long-lived goroutines.
- [x] **D.5** Implement `Emitter.EmitOracleMain(plan RegionPlan) (GeneratedFile, error)` per ADR-0023.
- [x] **D.6** Implement `Emitter.EmitDeployment(plan RegionPlan) (corev1.Deployment, *corev1.ConfigMap, *corev1.Secret, error)`: Deployment with env/args/volumeMounts from Block C. Internal Service only (no external exposure).
- [x] **D.7** Static guard: extracted-service Service spec uses `ClusterIP`, never `LoadBalancer` or `NodePort`. Test asserts this.
- [x] **D.8** Toy session-surface fixture under `test/e2e/targets/streamproxy-toy/`: tiny Go program with one `http.Hijacker`-using handler (echo server over raw TCP after hijack). Pragma declares session-surface region. Emit + `go build` succeeds for host + extracted + oracle.
- [x] **D.9** Toy in-process e2e (no kind cluster): host stub forwards to extracted; client connects, exchanges N bytes, disconnects; extracted side records byte count; oracle records both sides match.
- [x] **D.10** **Byte-parity test** using `gorilla/websocket` only in test code: confirms the proxy does not rewrite frames. Frames sent by the test client arrive at the test extracted-service byte-for-byte.
- [x] **D.11** Connection-lifetime tests with `httptest.Server`: client close propagates to extracted; extracted close propagates to client; context cancellation stops both copy loops.
- [x] **D.12** Auth header preservation test: forward `Cookie`, `Authorization`, `X-Requested-With`, `X-Forwarded-*` verbatim. Test asserts exact header bytes round-trip.
- [x] **D.13** Fail-mode tests: `MONOLIFT_LIFT_FAILMODE=closed` (default) returns 503 when extracted unreachable; `=open` runs original in-host handler.
- [x] **D.gate-1** If hijack mechanism fundamentally doesn't compose with Go's `net/http` server (HTTP/2 conn unhijackable on current Go version, etc.), document and stop at (C).
- [x] **D.14** Register streamproxy with `SurfaceCategory: Session` per ADR-0025; surface-derivation pass dispatches sessions to streamproxy.

**Block D gate:** toy session-surface fixture lifts in-process; bytes flow end-to-end; byte-parity test passes; auth headers preserved; fail-modes work; recursion-safety + internal-Service guards pass.

### Block E — Toy multi-root session-surface integration

Goal: end-to-end pipeline on a toy multi-root session-surface fixture exercises boot-path + RegionPatchRequest + stream-proxy together. **Catches integration gaps before Mattermost.**

- [x] **E.1** Toy multi-root session-surface fixture under `test/e2e/targets/streamproxy-multiroot-toy/`: two receiver types, each with a hijack-using method, sharing a chan-field seam (SPRINT-0022 shape but smaller). Pragmas with shared `name=`. `main` reads two env vars and one flag.
- [x] **E.2** Run full pipeline: pragma → closure-union → seam → stateclass → admission → surface-derivation → boot-path → emission via `RegionPatchRequest` → streamproxy.
- [x] **E.3** Assert `BootSpec` non-empty with two `EnvSource` + one `FlagSource`; `RegionPatchRequest` carries two `PatchSymbolRequest` entries (one per root) with non-empty per-symbol preludes; `RegionPatchResult.Files` records both patched files; generated extracted-main, oracle-main, Deployment, ConfigMap all exist.
- [x] **E.4** `go build` succeeds on host + extracted + oracle binaries.
- [x] **E.5** In-process e2e: client connects to host, bytes flow through to extracted, region semantics preserved.

**Block E gate:** toy multi-root session-surface fixture builds and runs in-process.

### Block F — Mattermost attempt

Goal: drive the full pipeline against Mattermost Hub/WebConn. **Branch decision point.**

- [x] **F.1** Re-confirm SPRINT-0022 pragma overlay (`test/e2e/targets/mattermost/pragma_overlay.go`) and target metadata still apply.
- [x] **F.2** Run surface-derivation on Mattermost. Expected: `Session` (because `(*websocket.Upgrader).Upgrade()` reachable from `connectWebSocket`). Assert.
- [x] **F.3** Run boot-path against Mattermost. Capture `BootSpec` into `docs/research/runs/SPRINT-0023-mattermost-bootspec.md`. Expected sources: `EnvSource{MM_SQLSETTINGS_DATASOURCE}`, `EnvSource{MM_*}` (plural), `FlagSource{config}`, `FileSource{config.json}`. Expected dep-init chain: `app.New() → server.New() → platform.NewService() → HubsStart()`.
- [x] **F.4** Honest characterization: list every `BootSpec.Refusals` entry; classify fundamental vs tooling-immaturity per SPRINT-0021 framing.
- [x] **F.gate-1** **Branch decision.**
    - If `BootSpec.Refusals` empty AND surface-derivation produced `Session` AND toy multi-root fixture (Block E) passed → proceed to F.5 (S branch attempt).
    - If `BootSpec.Refusals` non-empty OR any other characterized gap (auth-context, FileSource collision, DBSource required, dependency minimization gap): write characterization in `docs/research/runs/SPRINT-0023-mattermost-attempt.md` with classifications + follow-up sketches. Skip F.5–F.7. Sprint lands (R).
- [x] **F.5** Drive `RegionPatchRequest` on Mattermost: emit host stubs for session-surface external entry points across both receiver types. Patch plan derived from `RegionSurface.EntryPoints` (do not over-include lifecycle/internal methods like `Hub.Stop` or `Hub.ProcessAsync` that aren't externally bound — let the surface pass be the source of truth). Skipped by F.gate-1 branch (R).
- [x] **F.6** Render extracted-service main + oracle main + Deployment + ConfigMap + Secret into `lifted/host-patch/server/` via `bin/e2e-compile`. Skipped by F.gate-1 branch (R).
- [x] **F.7** **Acceptance bar for (S):** `go build ./...` succeeds on host (with patches), extracted-service, oracle. `make verify-evaluation-untouched` passes. Capture build logs into `docs/research/runs/SPRINT-0023-mattermost-build.md`. Skipped by F.gate-1 branch (R).

**Block F gate:** either F.7 completes (S) or F.gate-1 routed to (R) with characterization.

### Block G — Regression + frozen-boundary verification

- [x] **G.1** `go test ./pkg/compiler/...` all green.
- [x] **G.2** `go test ./test/e2e/e2ecompile/...` serially, all green.
- [x] **G.3** Full-matrix regression: `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -timeout 60m -count=1`. Caddy + miniflux + pocketbase + pragma + multi-root machinery + new streamproxy-toy fixtures. SPRINT-0017/0019/0020/0022 byte-identical to pre-sprint.
- [x] **G.4** `make verify-evaluation-untouched`. `evaluation/mattermost/` byte-identical.
- [x] **G.5** Verify `cmd/main.go`, ADR-0018, `PatchSymbolBody` signature unchanged via `git diff` against sprint base.
- [x] **G.6** Property-lint: assert no new `liftability.PropertyID` constants.
- [x] **G.7** Admission-rule discipline check: if `pkg/compiler/transport/admission.go` changed, hypothesis doc exists and discipline checklist (a)-(e) is satisfied. Default expectation: zero changes.

### Block H — Documentation, ADR addenda, ledger

- [x] **H.1** Sprint closeout section in this file: which branch (S/R/C); what landed; boot-path metrics; Mattermost outcome.
- [x] **H.2** New ADR `docs/decisions/0026-bootpath-extraction.md`: pipeline placement (after admission+surface-derivation, before emission), `BootSpec` shape, config-source taxonomy, dependency classification rules, refusal-not-admission framing.
- [x] **H.3** New ADR `docs/decisions/0027-region-patch-request.md`: API additive shape, why `PatchSymbolBody` retained, routing rule, sentinel-uniqueness derivation.
- [x] **H.4** ADR-0025 additive amendment: stream-proxy is the v0 wire protocol for Session category; surface-derivation pass landed at `pkg/compiler/surface/`; AsyncProducerConsumer category refused with diagnostic per spec.
- [x] **H.5** ADR-0023 additive amendment **only on (S)**: cmd-inside-host emission for session-surface multi-root regions.
- [x] **H.6** No amendment to ADR-0018.
- [x] **H.7** Update `docs/evolution.md` narrative entry.
- [x] **H.8** Update `docs/evaluation/targets/03-mattermost.md` from SPRINT-0022: add this sprint's outcome.
- [x] **H.9** `docs/sprints/ledger.yaml`: status `done` (S/R) or `cliff-blocked` (C); record landing branch. Deferred by explicit operator instruction: do not modify the ledger.
- [x] **H.10** `test/e2e/targets/mattermost/target.go`: on (S), `SkipReason` updated to point at SPRINT-0024 (kind-cluster e2e); on (R), points at this sprint's characterization doc.

## Closeout

Branch: **R**.

Landed machinery:
- Surface derivation for Call/Session with async producer-consumer refusal.
- Additive `RegionPatchRequest`; legacy `PatchSymbolBody` retained.
- Boot-path extraction and report schema support.
- BootSpec-to-Kubernetes config rendering with Secret classification.
- Stream-proxy emitter package and toy session/multi-root fixtures.

Mattermost outcome:
- B.gate-1 completed under budget with `GOWORK=$PWD/.tmp/sprint-0021-a1-go.work`: 55.41s wall, 2,346,369,024 bytes max RSS.
- Mattermost branch decision is R. The compiler does not yet derive the websocket route as the external Session surface and does not reconstruct the expected Mattermost config/init chain. Characterization: `docs/research/runs/SPRINT-0023-mattermost-attempt.md`.

Housekeeping follow-up:
- `e2e-compile` should detect multi-module sources and set up `GOWORK` automatically for targets like Mattermost.

## Acceptance criteria

The sprint accepts iff one of (S)/(R)/(C) holds and all gates for that branch pass. **A sprint that lands none of these cleanly is a process failure.**

### (S) — Machinery lands; Mattermost emits buildable artifacts
- All Block 0–H gates pass; F.gate-1 routed to F.5.
- Surface-derivation classifies caddy/miniflux/pocketbase as Call (no behavioral change); Mattermost as Session.
- `RegionPatchRequest` accepts the Hub/WebConn region; toy multi-root fixture builds; Mattermost-emitted artifacts at `lifted/host-patch/server/` build with `go build ./...` for host + extracted-service + oracle.
- Boot-path produces non-empty `BootSpec` for Mattermost with expected env/flag/file sources and the PlatformService dep-init chain. No `Refusals`.
- Stream-proxy renders host stubs for all session-surface external entry points across both receiver types; toy fixture passes byte-parity, lifetime, auth-preservation, and fail-mode tests.
- Manifests classify secrets correctly (datasource credentials never plaintext); recursion-safety holds; extracted Service is internal-only.
- SPRINT-0017/0019/0020/0022 e2e byte-identical; `evaluation/mattermost/` byte-identical; `cmd/main.go` unchanged; ADR-0018 unchanged; `PatchSymbolBody` API unchanged.
- ADR-0026 + ADR-0027 + ADR-0023 amendment + ADR-0025 amendment landed.

### (R) — Machinery lands; Mattermost stops at a characterized gap
- Block 0, A, B, C, D, E pass. Block F fires F.gate-1 → R route.
- `docs/research/runs/SPRINT-0023-mattermost-attempt.md` exists with one entry per `BootSpec.Refusal` or stream-proxy gap (verdict, triggering code shape, distribution-feasibility analysis, fundamental-vs-tooling-immaturity classification, follow-up sketch). Synthesis paragraph names dominant gap shape.
- All toy fixtures (Block A patcher, Block B boot-path, Block C manifest, Block D streamproxy, Block E multi-root session-surface) build and pass.
- ADR-0026 + ADR-0027 landed; ADR-0023 amendment deferred (no working multi-root session-surface emission to ratify); ADR-0025 amendment landed.

### (C) — Hard cliff before machinery completes
- Stop at one of: A.gate-1 (RegionPatchRequest spills beyond `emit/`), B.gate-1 (boot-path OOM at Mattermost scale), D.gate-1 (hijack mechanism doesn't compose), or C.gate-1 (Secret classification can't be made safe).
- Cliff doc `docs/research/runs/SPRINT-0023-<cliff>.md` records reproduction steps + resource numbers + log excerpts.
- All other targets (caddy, miniflux, pocketbase, pragma, multi-root machinery) still pass.
- `test/e2e/targets/mattermost/target.go` `SkipReason` points to the cliff doc.
- Sprint ledger marked `cliff-blocked`.

## Resolved decisions (in-flight)

- **B.gate-1 GOWORK requirement** — *resolved 2026-04-27, mid-sprint.* Codex's probe failed to load `evaluation/mattermost/server` because it ran without a Go workspace, so `packages.Load` resolved `server/public` from the module cache instead of the local submodule tree, producing version skew (undefined `model.AccessControlPolicyVersionV0_3`, `mlog.MlvlRemoteClusterServiceWarn`, etc.). SPRINT-0021's successful probe used a `GOWORK` workspace pointing at both `evaluation/mattermost/server` and `evaluation/mattermost/server/public` — that file is still at `.tmp/sprint-0021-a1-go.work` and `GOWORK=$PWD/.tmp/sprint-0021-a1-go.work go list ./evaluation/mattermost/server/...` works. Resolution: codex resumes with `GOWORK` set in the probe environment. The `e2e-compile` driver gains a TODO to detect multi-module sources and set up a workspace automatically (not in scope this sprint; housekeeping follow-up). This is a tooling regression, not the SSA OOM cliff Cliff 2 was designed to catch — the boot-path walk has not yet been measured against Mattermost.
