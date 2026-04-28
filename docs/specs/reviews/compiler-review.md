# Monolift v2 Contract — Compiler-Implementer Review

**Target:** `docs/specs/monolift-v2-contract.md` v0.1-draft (2026-04-19)
**Reviewers (merged):** codex / gemini / claude (via `sprint-planner`-style fan-out; critiques cross-cut)
**Merge-author:** Opus 4.7 (this session)
**Perspective:** what it takes to implement this spec with `go/ast`, `go/packages`, `go/types`, `go/ssa`

---

## Executive summary

**Verdict: accept with revisions.** The spec identifies the right v1 failures and
retires the worst ones (interface-only pragmas, `main`-walk extraction, stateless-only
lifts, the `clientgen.go:110` panic). But roughly a third of the normative rules
describe compiler-hard problems as if naming a diagnostic made them implementable.
The closure report — explicitly named as the spec/implementation interface — is
under-specified. Two conforming compilers could produce incomparable reports and
refuse different programs today.

**Top three concerns:**
1. **Closure-report schema and symbol-identity are unspecified.** The document's
   named interface to SPRINT-0005 is prose, not a schema.
2. **Call-graph algorithm baseline is not named.** Every "resolved interface call",
   "bounded dispatch set", and "unresolved reflection" rule depends on the choice
   of CHA / RTA / VTA / points-to, and the spec is silent.
3. **Several MUST-level predicates are not mechanizable as written.** "Wrapping
   adapter", "unrelated application subsystem", "wiring cannot be represented
   deterministically", "cache divergence is not correctness-observable",
   "reproducible cgo" — each requires a predicate the spec doesn't give.

Fixable with ~15 concrete edits below. Without them, SPRINT-0005 will either
ship a much narrower compiler than the spec claims, or produce non-reproducible
refusal behavior.

---

## Blocking issues

### B1. Closure-report schema under-specified (L334–L350)

EC-REPORT-1 lists contents but leaves format "implementation-defined". Since the
report is the named spec/implementation interface, this is the single most
important defect. Two implementers will produce incomparable reports, and tests
cannot assert rule IDs stably.

**Required:** JSON schema (in the spec, even as an appendix) with:
- `schemaVersion`
- Symbol identity as `{module_path, package_path, object_name, kind, instantiation?}` — not `types.Object.String()` (unstable across Go versions)
- Source spans as `{file_relative_path, byte_offset_start, byte_offset_end, line_start, line_end}` (Go positions are byte offsets)
- Stable lexicographic ordering over symbol identity (reproducible diffs)
- Build-config fields: `GOOS`, `GOARCH`, `CGO_ENABLED`, build tags, module root, workspace mode, `Tests` flag

### B2. Call-graph algorithm baseline not named (L294–L296, EC-CLOSURE-3)

"Call-graph precision is insufficient" is the load-bearing phrase for nearly
every dynamic-dispatch rule in the spec (impl ambiguity, reflection resolution,
dispatch-set boundedness, wiring). `go/ssa` gives IR; it does not answer dynamic
call targets by itself.

**Required:** name the analysis *requirement* (conservative call-graph with named
precision triggers for refusal), not a specific algorithm. State that the
selected algorithm MUST be recorded in the closure report, and that two runs
with the same package graph under the same algorithm produce identical
verdicts. Leave choice between CHA / RTA / VTA / pointer open but require disclosure.

### B3. "Closure too large" has no metric (L352–L358, EC-PRUNE-2)

"Unrelated application subsystem", "persistence plus routing without bounded
adapter", "broader public surface" are not compile-time predicates.
`MLV2_CLOSURE_TOO_LARGE` as written is "the compiler said so".

**Required:** concept is blocking; exact thresholds are SPRINT-0005 calibration
against the six-target corpus. Spec should commit to a *predicate form* (e.g.,
"closure is bounded when its external-edge frontier is finite under EC-TERM-*
and no refusal condition in §3–§5 applies") and include a recommended-defaults
section with calibration notes (indicative examples: ≥10 external-module
packages, ≥20 exposed methods on root surface — both non-normative).

### B4. Wiring-pattern whitelist missing (L326–L332, EC-WIRE-4)

"Initialization/value graph cannot be represented deterministically" has no
decision procedure. For arbitrary Go this is program evaluation (Rice's
theorem).

**Required:** enumerate supported patterns:
- Top-level `var x = expr` where `expr` is a constant, composite literal, or call
  whose callee has no writes to package globals
- `init()` functions containing only package-global writes and registry calls
- Blank-import `_ "pkg"` with `pkg.init()` matching the above
- Framework registries via declared adapter metadata (see B5)

Everything else refuses with `MLV2_WIRING_UNRESOLVED`.

### B5. Adapter metadata as a first-class spec construct (cross-cutting)

The spec implies adapters (handler adapters, registry adapters, serialization
adapters, context-value adapters, external-dependency adapters, generic
type-substitution adapters) in many places but doesn't define the metadata
format or conformance rules. This is the single highest-leverage edit: it
unifies the registry-tracing gap (L614–L620), handler adapter contract
(L488–L492), cgo/reflection allowlists, and framework context propagation.

**Required:** a "Adapter Metadata" section defining adapter kinds, matched
types/functions, accepted canonical shapes, state effects, required report
fields, and a conformance schema.

### B6. Wrapping-adapter detection non-mechanical (L622–L630, MI-WRAP-2)

"Stores another implementation of the same interface and forwards calls" is
not a syntactic predicate. Without one, `MLV2_IMPL_WRAPPER_AMBIGUOUS` fires
based on implementer-defined heuristics.

**Required:** syntactic predicate as default: "Type `W` is a wrapper of
interface `I` iff `W` has one field (direct or promoted) of a type assignable
to `I`, and every method of `W` implementing `I` makes at least one call on
that field with a matching method name." Otherwise fall through to
`MLV2_IMPL_WRAPPER_AMBIGUOUS`.

### B7. Generics contradiction (AS-FUNC-1 at L126 vs TA-SER-5 at L502)

AS-FUNC-1 accepts "named package-level function declaration" without
addressing generics. TA-SER-5 defers generic instantiation expressions. But a
generic function declaration *without* a selected type argument set has no
monomorphized adapter shape. Spec accepts a root whose adapter cannot be
generated.

**Required:** either refuse generic declarations in v2 (with `MLV2_SURFACE_DEFERRED_GENERIC_DECL`)
unless every instantiation reachable under the selected build is enumerable, or
defer them entirely alongside instantiation expressions.

### B8. `mode=singleton` vs `state=singleton` unresolved (L559–L568, L694–L705)

DP-MODE-1 lists `singleton` as a dispatch mode and placement constraint; the
keys table lists `state=singleton` as a disposition. An implementer cannot
tell whether `mode=singleton state=singleton` is redundant, incompatible, or
additive.

**Required:** pick one. Recommend: remove `singleton` from `mode`; let the
state disposition drive placement. Update L567, L696, L702.

### B9. `x-<vendor>` extension-key reservation ambiguous (L687, PS-GRAMMAR-2)

"Unknown keys MUST be parseable but validation-defined. A compiler MAY reserve
`x-<vendor>` keys." If each compiler decides independently, portable pragmas
are impossible.

**Required:** reserve `x-` globally across all conforming compilers;
`MLV2_PRAGMA_UNKNOWN_KEY` MUST NOT fire on `x-*`.

### B10. Pragma attachment unspecified (cross-cutting, EBNF L672–L685)

`go/ast` distinguishes `Decl.Doc` (immediately-preceding, no blank line) from
`Decl.Comment` (trailing). The spec doesn't pick. Ambiguous attachment →
ambiguous refusals.

**Required:** pragmas MUST appear as `Doc` comments on the annotated
declaration. Trailing/line-end comments MUST refuse with `MLV2_PRAGMA_MISATTACHED`.
Multiple `//monolift:lift` on one declaration: `MLV2_PRAGMA_DUPLICATE`.
Unknown verbs (`//monolift:retire` etc.): `MLV2_PRAGMA_UNKNOWN_VERB`.

### B11. v1 migration is optional (L782–L790, PS-MIGRATE-1..3)

`MAY` recognize + `SHOULD` diagnose. A conforming v2 compiler can silently
ignore `// @monolift` and `//monolift:offload`, which is not a coherent
migration from the current codebase (see `codebase-state-2026-04-19.md`).

**Required:** v2 compiler MUST scan comments for known v1 prefixes and emit
`MLV2_PRAGMA_V1_DEPRECATED` (warning). It MUST NOT generate a v2 lift from
v1 syntax. Suggested rewrites may use `name=<required>` placeholder.

---

## Serious concerns (non-blocking but should land before v1.0)

### Go-semantic precision

- **S1. Constraint interfaces (Go 1.18 type sets).** AS-IFACE-1 (L109) doesn't
  distinguish method-set interfaces from constraint interfaces with `~T` or
  unions. Use `types.Interface.IsMethodSet()` as the predicate; refuse non-method-set
  interfaces as lift roots.
- **S2. Pointer/value receiver in `impl=`.** `T` and `*T` have different method
  sets. Grammar (L677–L681) doesn't allow `*pkg.T`. Define `impl=` as
  `types.Type`-plus-addressability, or canonical-form syntax `impl="example.com/mod/pkg.(*T)"`.
- **S3. Named functions cannot capture closure variables (L396 factual error).**
  The spec language treats "closure variables captured by accepted named functions"
  as receiver-field-like state. Named package-level Go functions have no
  lexical environment. Rewrite to cover function literals reachable in the closure.
- **S4. Method value vs method expression.** `fn := svc.CreateUser` (binds
  receiver) vs `UserService.CreateUser(s, ...)` (receiver as first arg).
  Dispatch rewrite rules don't cover these. Add explicit handling or refuse.
- **S5. Call-argument evaluation order under rewriting.** Go guarantees
  left-to-right evaluation of receiver/arguments with side effects. A rewrite
  that hoists into RPC marshaling can silently reorder. CP-3 (L103) should
  commit normatively to preserving evaluation order.
- **S6. `error`-as-interface carve-out.** TA-SER-4 (L500) refuses interface
  parameters with unbounded type sets; `error` is technically such an interface.
  Explicitly exempt `error` via TA-SER-2's envelope.
- **S7. Empty interface / `any`.** TA-SER-4 would refuse all `any` parameters.
  State explicitly whether `any` is refused, deferred, or requires adapter metadata.
- **S8. Method-set normalization for struct pragmas (AS-STRUCT-2, L163–L165).**
  `T` and `*T` method sets differ. Pick pointer method set as default; state
  whether promoted embedded methods are included unless excluded by `methods=`.
- **S9. Methods on non-struct defined types.** Go allows `type Count int;
  func (c Count) Inc() Count`. Spec's struct-centric state rules don't generalize.
  Either restrict AS-METHOD to struct receivers explicitly, or generalize
  state rules.

### Refusal defaults for hazardous constructs

- **S10. cgo default-refuse-with-opt-in.** EC-TERM-3 (L314) "reproducibility"
  is undecidable. Refuse cgo in the closure by default; require `cgo=allow`
  key for opt-in with adapter metadata.
- **S11. Reflection default-refuse-with-opt-in for dispatch.** EC-TERM-4
  (L316): distinguish data-encoding reflection (`encoding/json`, ORMs)
  from dispatch reflection (`reflect.Value.Call`, `MakeFunc`,
  `MethodByName`). Refuse application-controlled dispatch reflection by
  default; allow known library reflection via adapter allowlist.
- **S12. `//go:linkname`, unsafe, assembly bodies.** Add diagnostics
  `MLV2_LINKNAME_ROOT`, `MLV2_UNSAFE_CLOSURE`, `MLV2_ASM_BODY`. Refuse
  unless opt-in.
- **S13. Variadic functions.** Canonical shapes (L468–L480) say "variadic
  unserializable args" are unsupported. Be explicit: variadic with
  serializable element type is accepted via slice encoding.

### Canonical-shape classification

- **S14. Type-level predicates per shape.** Each shape in the taxonomy
  (L470–L480) needs a `types.Type`-based predicate, not prose. E.g., HTTP
  handler = "parameters are exactly `(http.ResponseWriter, *http.Request)`
  or receiver `T` satisfies `http.Handler`". Channel-consumer is a
  *behavioral* pattern, not a signature — promote it to an explicit
  `shape=worker` key or refuse it as a signature-shape category.
- **S15. Shape priority order needs a tie-break rule (L470).** `func(ctx,
  <-chan T) error` matches both channel-consumer and ctx-req-resp. State
  the tie-break.
- **S16. Shape-preserving HTTP handler requires ProxyResponseWriter codegen.**
  TA-HANDLER-1 (L482–L492) forwards the HTTP request to a remote handler;
  the response returns through the original framework path. To preserve
  `http.ResponseWriter`, the compiler must generate a proxy implementation
  that captures status/headers/body and returns them across the transport.
  State this as a normative codegen requirement and add
  `MLV2_HANDLER_CAPABILITY_UNSUPPORTED` for features the proxy cannot
  preserve (streaming, hijack, WebSocket upgrade, HTTP/2 push, trailers).

### Serialization

- **S17. `time.Time`, `json.Marshaler`, struct tags, unexported fields, custom
  text/binary marshalers** — TA-SER-1 (L494) says "named types whose exported
  representation is serializable"; commit explicitly to `encoding/json`
  semantics for the selected Go version, or enumerate.
- **S18. Recursive types vs runtime cycles.** TA-SER-3 (L498) demands acyclic
  pointer graphs. Cycle detection at runtime serialization is encoder-level;
  compile-time-decidable check is "type graph is non-recursive", which
  over-refuses common linked structures. Clarify: type-level check or
  runtime check; if runtime, state that some failures become runtime errors.
- **S19. Pointer/reference mutation.** `TA-SER-3` allows aliasing within one
  request envelope but is silent on the callee mutating a pointer argument
  or receiver field that aliases caller-visible state. Refuse mutable
  pointer arguments that require caller-visible alias preservation, or
  define copy-in/copy-out + alias restoration.

### Context and state

- **S20. `context.Context.Value` dropping.** TA-SER-6 (L504) says drops are
  reported. Make this syntactic: "Any call to `(context.Context).Value`
  reachable in the closure MUST be listed in the closure report as a
  potential dropped context value."
- **S21. State-inference evidence catalog.** SS-CLASS-1 (L406) lists evidence
  categories but ships no list of "known session types", "known cache types",
  "known external client types". Without this, every conforming compiler
  will disagree on state classes. Add an appendix or reference-impl list.
- **S22. Generated-mock detector.** MI-FILTER-2 (L634–L636) is normative but
  undefined. Commit to the community convention regex
  `^// Code generated .* DO NOT EDIT\.$` (somewhere in file prelude) as
  the baseline; allow additional patterns via adapter metadata; require
  the matched rule to appear in the closure report.

### Diagnostics and migration

- **S23. Diagnostic priority ordering.** PocketBase at L911–L927 fires at
  least three diagnostics (`MLV2_EMBEDDED_DB_APP_ROOT`,
  `MLV2_CLOSURE_TOO_LARGE`, `MLV2_SHARED_MUTABLE_STATE`). Spec should define
  priority order for cascaded refusals. Proposed: parse > surface > root >
  closure-size > wiring > state > shape > serialization > policy > transport.
  Primary diagnostic wins; others appear as related-info.
- **S24. Multi-module workspaces / replace directives / vendoring.** EC-TERM-2
  diverges between main-module and everything-else. With `go.work` and
  `replace`, this is ambiguous. State which rule applies and how the selected
  build config disambiguates.
- **S25. Blank-import preservation.** Lifted deployables must retain blank
  imports that trigger registry `init()` side effects. Report MUST list
  init-edge imports retained solely for side effects.

---

## Suggested spec edits (concrete)

In decreasing priority. Line numbers refer to the reviewed draft.

1. **Replace EC-REPORT-1/2 (L336–L350) with a JSON Schema appendix.** Minimum
   fields: `schemaVersion`, build config, root identity, exposed operations
   with canonical signatures and type args, included symbols with canonical
   IDs and inclusion edges, state items with field paths and evidence,
   adapters by stable ID, external deps with adapter IDs and config evidence,
   pruning decisions with edge IDs, diagnostics with code/severity/span/remediation.
2. **Add §3.x "Analysis Baseline" between L294 and L296.** State the
   call-graph analysis requirement as a *specification* property (conservative,
   precision-triggered refusals); require algorithm disclosure in report;
   leave algorithm choice open. Cite `golang.org/x/tools/go/callgraph/{cha,rta}`
   as candidate substrates.
3. **Add §3.y "Adapter Metadata" as a first-class construct** (B5). Covers
   handler, registry, serialization, context-value, external-dependency,
   cgo, reflection, generic-substitution adapters.
4. **Replace EC-PRUNE-2 with a predicate.** "A closure is bounded when the
   reachable external-edge frontier is finite under EC-TERM-* and no refusal
   condition in §3–§5 applies." Keep the smells list as non-normative
   implementer guidance. Add a recommended-defaults section indicating
   numeric hints for SPRINT-0005 calibration.
5. **Tighten EC-WIRE-4 with a supported-patterns list** (B4).
6. **Replace MI-WRAP-2 (L626) with a syntactic predicate** (B6).
7. **Add a Generic Declarations subsection under §2** (B7).
8. **Reconcile `mode=singleton` vs `state=singleton`** (B8).
9. **Reserve `x-` globally** (B9).
10. **Require `Doc`-comment pragma attachment** (B10); add `MLV2_PRAGMA_MISATTACHED`,
    `MLV2_PRAGMA_DUPLICATE`, `MLV2_PRAGMA_UNKNOWN_VERB`.
11. **Strengthen v1 migration to MUST-warn** (B11); known v1 prefixes raise
    `MLV2_PRAGMA_V1_DEPRECATED`.
12. **Add diagnostic-priority ordering** (S23).
13. **Enumerate canonical-shape type predicates** (S14), with tie-break rule (S15).
14. **Add ProxyResponseWriter codegen requirement + handler-capability
    diagnostic family** (S16).
15. **Add constraint-interface refusal via `Interface.IsMethodSet()`** (S1).
16. **Add `error`-as-interface carve-out from TA-SER-4** (S6).
17. **Default-refuse-with-opt-in for cgo, dispatch reflection, unsafe, linkname,
    assembly bodies** (S10–S12).
18. **Ship evidence catalogs** for state inference, generated-mock detector,
    context-value reporting (S20–S22).

---

## Points of reviewer disagreement + merge resolutions

| Disagreement | Positions | Merge decision |
|---|---|---|
| Call-graph algorithm prescription | CLAUDE: hard-prescribe RTA; CODEX: leave open | Name the *requirement* and refusal contract; leave algorithm open with disclosure |
| Closure-too-large metric specifics | CLAUDE: ≥10 pkgs / ≥20 methods; GEMINI: 20% symbols; CODEX: "measurable cut" | Predicate is blocking; numeric thresholds are non-normative defaults, SPRINT-0005 calibrates |
| Data-race complexity framing | GEMINI: NP-hard; CLAUDE: undecidable; CODEX: no claim | Drop complexity-class framing; state as "not mechanizable as soundness property"; require developer declaration fallback |
| Reflection refusal breadth | CLAUDE: blanket refuse application `reflect.Call`; CODEX: distinguish encoding from dispatch | CODEX's nuance wins — default-refuse application dispatch reflection; exempt known serialization reflection via adapter |
| v1 migration (MAY vs MUST) | CLAUDE: coherent; CODEX/critiques: too weak | MUST-warn on known v1 prefixes |
| ProxyResponseWriter severity | GEMINI: top blocker; CLAUDE: codegen detail | Serious but not top blocker; keep concrete codegen requirement |
| Hard error-checking refusal | GEMINI: refuse lifts where error ignored; CLAUDE/CODEX: over-strict | Warning-level `MLV2_LIFT_ERROR_IGNORED`, not refusal |

---

## What the spec gets right (do not unwind)

- Compile-time refusal discipline replacing `clientgen.go:110` panic.
- Declaration-root focus (refusing anonymous functions, function-valued vars,
  generic instantiation expressions) sidesteps a class of analysis work.
- Call/value closure replacing `main`-walk extraction.
- Shape-preserving handler transport (modulo capability gaps in S16).
- Closure report as a required artifact — the structure is right, the
  schema discipline needs tightening.
- v1 migration stance of warn-not-rewrite is correct (only the MAY/MUST
  level needs strengthening).

---

## One-paragraph verdict

Ready for Phase 0 alignment but not ready to hand to an SPRINT-0005 implementer.
The closure-report schema, call-graph baseline, wiring-pattern whitelist, and
wrapper predicate are the load-bearing gaps; everything else in the blocking
list is ~10 lines of spec text each. With the 18 suggested edits, this becomes
a buildable contract. Without them, SPRINT-0005 will produce a compiler whose
acceptance/refusal verdicts diverge from other conforming implementations, and
the spec's interoperability claim (CP-3) will not hold. Estimated effort for
the full edit pass: 1–2 focused revision sessions plus a re-run of Phase 9
target validation against the revised rules.
