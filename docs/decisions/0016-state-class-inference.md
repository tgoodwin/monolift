# ADR-0016: State-class inference

**Status:** accepted
**Date:** 2026-04-21
**Context docs:** `docs/sprints/SPRINT-0007.md`, `docs/specs/monolift-v2-contract.md` §State Semantics, `docs/decisions/0013-ssa-closure-precision.md`

## Context

SPRINT-0007 replaces hardcoded state classification with a general
state-inference pass. The design pressure comes from four constraints:

- state evidence is harvested from SSA closure reachability, mutation sites,
  sync witnesses, and `go/types`-resolved external client types
- inference follows the six-rule precedence committed in the sprint plan under
  ADR-0013's precision budget
- developer-declared `state=` must narrow safely without hiding unsafe
  mutations
- the SPRINT-0004/0006 synthetic `BaseApp.db` label is retired in favor of
  corpus-discovered per-field symbols on `*BaseApp` for the embedded-DB
  composite refusal

## Decision

- Add a dedicated `pkg/compiler/stateclass/` package that infers captured
  state from SSA closure evidence plus `go/types`-resolved field/global types.
- Run the pass from the live extraction flow through the same registration
  seam used by the shape pass, so `extract.Analyze` stays the orchestration
  point without importing semantic-pass packages directly.
- Infer state from harvested seed symbols rather than target-specific
  hardcoding:
  - receiver fields discovered from the root type, or from interface
    implementers discovered by `go/types`
  - package globals in the root package that are actually referenced or
    mutated
  - mutation sites, sync witnesses, channel-loop evidence, and external-client
    allowlist matches
- Preserve the sprint's precedence intent with a pragmatic six-rule evidence
  stack tried in strict order inside `inferClass`:
  1. `externalClientTypeRule` — external/remote client types resolved via
     `go/types` against the allowlist
  2. `sharedGlobalMutationRule` — package-global variables with observed
     store sites
  3. `syncPrimitiveRule` — sync-primitive witnesses (`sync.Mutex`,
     `sync.RWMutex`, `sync.Once`, etc.) guarding the seed
  4. `channelLoopRule` — mutation observed inside a channel-driven loop
  5. `mutationFreeReadRule` — captured state read without any store sites
  6. `stackLocalRule` — freevar captures with no store sites that do not
     escape to shared state
  The first rule that matches wins; the cascade is short-circuit. If all six
  fall through and the developer did not declare `state=`, `inferSeed` emits
  `MLV2_STATE_UNKNOWN` as a correctness-relevant ambiguity fallback —
  distinct from the rule stack, not a seventh rule. A separate post-pass,
  `applyCompositeEmbeddedDBRule`, then runs over the already-inferred seeds
  to detect the embedded-DB app-root pattern and is counted as a post-pass,
  not a precedence rule. (SPRINT-0007 closeout framing of "seven-rule
  precedence" counts the composite post-pass alongside the six stack rules;
  the ambiguity fallback is a separate mechanism.)
- Keep developer `state=` declarations narrowing-only. Safe declarations mark
  rows as developer-declared; `state=stateless` on obviously mutable global or
  singleton/session state refuses with `MLV2_STATE_DECL_CONFLICT`.
- Replace the Pocketbase shim with a general composite rule: when a large root
  exposes more than `embeddedDBAppRootMethodThreshold` operations and captured
  state includes embedded durable client fields, emit
  `MLV2_EMBEDDED_DB_APP_ROOT`, keep `MLV2_CLOSURE_TOO_LARGE`, and surface one
  refused state row per discovered field.
- Retire the synthetic `BaseApp.db` label. The report now uses the actual
  corpus field names discovered from `go/types`
  (`BaseApp.concurrentDB`, `BaseApp.nonconcurrentDB`,
  `BaseApp.auxConcurrentDB`, `BaseApp.auxNonconcurrentDB`).
- Coalesce identical non-refusal rows for readability. Caddy's many read-only
  config fields collapse to one root-level immutable-config row, while
  refusal-causing embedded-DB fields remain one row per field.

## Consequences

- State evidence now comes from the same SSA extraction seam as the rest of
  the semantic passes instead of from ad hoc target matchers.
- Pocketbase no longer needs any compiler code that names Pocketbase itself;
  the old shim file is deleted and the negative case is preserved by a generic
  rule.
- The pass is intentionally conservative about ambiguity, but the new harvest
  filters avoid spamming read-only third-party globals that are not part of
  the lifted root's meaningful captured state.
- Future precision work can improve ambiguous cases without changing the
  report contract or reintroducing parser coupling.
