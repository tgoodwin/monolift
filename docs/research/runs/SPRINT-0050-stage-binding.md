# SPRINT-0050 Stage Binding

**Date:** 2026-05-16
**Sprint:** SPRINT-0050
**Remote run root:** `/local/repository-codex-sprint0050/.moab/runs/`

This note binds the activation stages for SPRINT-0050 before implementation.
It also records the policy answer to the test-harness question: research is not
driven by encoding every question as a Go test. Go tests are regression gates
for stable invariants. Exploratory target research should be run as scoped
CLI/script probes that write artifacts under `.moab/runs/...`, including the
source root, target, package scope, selected cut, verdict, timings, and logs.

## Evidence Inputs

- Admission-only baseline:
  `/local/repository-codex-sprint0050/.moab/runs/sprint-0050-admission-baseline/`
  recorded `5 pass / 12 admission-skip / 55 manifest-skip / 0 timeout-skip`.
  This run is a coarse manifest-drift measurement only.
- Focused scoped admission:
  `/local/repository-codex-sprint0050/.moab/runs/sprint-0050-focused-admission-scoped/`.
- Source-module-local lift probe:
  `/local/repository-codex-sprint0050/.moab/runs/sprint-0050-focused-lift-source-local/`.

The first admission attempt was preserved at
`.moab/runs/sprint-0050-admission-baseline-path-failure/` because non-login SSH
did not put Go on `PATH`. It is an infrastructure finding, not candidate
evidence.

## Phase 0 Rulings

Direct-invoke policy:
`nullable-localized-error` is a typed expectation. For functions that return a
localized-error wrapper, a nil direct result is a valid success shape when the
workload predicate and extracted-service `/calls` delta also pass. This is not
a generic opt-out from stage 8-10; targets must declare the expectation and the
side-effect predicate.

Parent-over-leaf admission:
A parent cut does not prove the intended durable-resource leaf. SPRINT-0050
should not change the default ranking just because a parent admits. If deeper
admissible exploration is needed, make it opt-in and test it separately. Corpus
rows and coverage reports must record the selected cut, not just the original
trace function.

Workload fitness:
Required config that makes the workload hit the lifted symbol belongs in target
metadata. The `gitea/M-16` lesson is binding: `GITEA__security__PASSWORD_HASH_ALGO=argon2`
must be part of the target setup, not a remembered run command. For activation
targets, the harness validates declared workload requirement env vars against
the host deployment options so metadata cannot drift from setup. It also checks
the baseline manifests for the same env var/value before running the baseline
workload, which catches cheap workload-fitness failures before lifted deployment
and `/calls`-delta checks.

Admission scope:
Focused candidate probes must use reverse-import scope or an explicit
target/importer package set. Whole-repository `./...` admission is invalid for
candidate rejection. Timeouts from broad package loading are scope failures, not
durable-resource evidence.

## Stage 3 - Activation And Cut

Assertion:
The activation path reaches the intended target and produces a ranked cut
candidate inside the project module.

Generated artifact:
Activation path output, cut report, phase timings, and candidate ranking under
the run directory.

Target toggles:
Trace target, source root, augmentation mode, and package scope. The default for
focused admission is reverse-import scope.

Allowed substitutions:
None. A parent cut may be recorded as a selected cut, but it cannot be counted
as proof for the leaf.

Open questions:
Whether deepest-admissible exploration should be a separate CLI mode after
SPRINT-0050.

## Stage 4 - Admission And Compile-Clean Plan

Assertion:
The selected cut admits, `BuildPlan` succeeds, and generated output is
compile-clean enough to become a harness artifact.

Generated artifact:
Admission verdict, refusal code, demotion chain when available, generated files,
and `monolift_lift_manifest.json`.

Target toggles:
Reconstructor registry entries, target metadata expectations, output directory
inside the source module, and package scope.

Allowed substitutions:
None for admission. If the intended leaf refuses and a parent admits, report
that as parent-cut-only evidence.

Open questions:
`pocketbase/M-1` should admit after a local `*filesystem.System` reconstructor
is registered. If it requires `core.App` or broader app-owned state, fall back
to `pocketbase/M-4`.

## Stage 5 - Artifact Build

Assertion:
The generated extracted service and host patch build from the source module
dependency graph.

Generated artifact:
Build logs, Dockerfile paths, and generated Go package paths.

Target toggles:
Module-local output path, dependency availability in the source `go.mod`, and
generator version.

Allowed substitutions:
None.

Open questions:
Whether the focused research CLI should run a build-only check without entering
the Kind harness.

## Stage 6 - Image Load

Assertion:
The host and extracted images build and load into the Kind cluster for the exact
target.

Generated artifact:
Docker build logs, Kind image-load logs, and image tags.

Target toggles:
Image names, target namespace, and build context.

Allowed substitutions:
None.

Open questions:
None for Phase 0.

## Stage 7 - Lifted Deployment Ready

Assertion:
The extracted deployment becomes Ready under the declared startup contract.
For persistence targets, the extracted service must receive the resource env
vars it needs, and it must not receive `MONOLIFT_LIFT_*`.

Generated artifact:
Rendered Kubernetes manifests, pod status, extracted pod logs, readiness probe
output, and dormant-env audit.

Target toggles:
DB URL, local filesystem root, shared volume mount, startup probe, and close
hooks.

Allowed substitutions:
None. A deploy that uses per-pod ephemeral state does not count as an external
persistence proof.

Open questions:
The filesystem target needs a shared durable local root between host and
extracted pods.

## Stage 8 - Lifted Workload Reaches Extracted Service

Assertion:
The real host workload reaches the extracted service. A direct `/invoke` check
is required unless the target declares an approved expectation/substitution.

Generated artifact:
Workload transcript, direct-invoke transcript, `/calls` before/after delta, and
host/extracted logs.

Target toggles:
Direct-invoke expectation, root-relative payload policy, workload predicate, and
normalizers.

Allowed substitutions:
- `nullable-localized-error`: nil result allowed when the workload predicate and
  `/calls` delta pass.
- `status-only`: allowed for explicitly declared status checks.
- `workload-calls-delta`: allowed only with a target predicate such as "feed
  entries exist" or "thumbnail exists".

Open questions:
The Miniflux localized-error wrapper needs the harness expectation change before
`miniflux/M-1` can move beyond stage 7.

### Direct `/invoke` Envelope: `miniflux/M-1`

Generated extracted server:

- `POST /invoke` decodes an `invokeRequest` with `user_id`, `feed_id`, and
  `force_refresh`.
- It calls `handler.MonoliftInvokeRefreshFeed(state.Store, req.UserID, req.FeedID, req.ForceRefresh)`.
- The generated `invokeResponse` has only `error,omitempty` for this
  localized-error wrapper shape.
- If the localized error result is nil, the server returns HTTP 200 with an
  empty JSON object (`{}`); there is no `result` field.
- If the localized error result is non-nil, the server returns HTTP 200 with
  `{"error":{"error":"...","message":"..."}}`.
- `/calls` increments for every direct invocation and `/invocations` stores the
  request plus that response envelope.

Harness decode path:

- `postInvoke` requires HTTP 200, decodes the response into `map[string]any`,
  and returns `out["result"]` unless the special `reading_time` field exists.
- For the nil localized-error success envelope (`{}`), `postInvoke` therefore
  returns nil.

Policy consequence:

`miniflux/M-1` must use `nullable-localized-error`: nil direct result is allowed
only with the declared feed-entry behavioral predicate and extracted-service
`/calls` delta. It must not be treated as a generic successful oracle compare.

## Stage 9 - Env-Off And Fail Modes

Assertion:
Env-off sends traffic through the original monolith path and records no
extracted-service calls. Fail-open/fail-closed behavior matches the declared
client policy.

Generated artifact:
Env-off transcript, fail-open transcript, fail-closed transcript, `/calls`
deltas, and service logs.

Target toggles:
Client policy, fresh-resource setup, and fault-injection target.

Allowed substitutions:
None unless the stage-binding doc names the behavioral invariant and why direct
comparison is impossible.

Fresh-resource policy:
Stateful DB and filesystem targets must declare a target-level fresh-resource
policy before they can rely on env-off/fail-mode evidence. The policy records
the resource kind, isolation scope, and concrete mechanism that prevents env-on
side effects from satisfying later checks. For DB rows, the minimum acceptable
scope is a resource created per workload setup call; for filesystem/object-store
targets, the minimum acceptable scope is a fresh root or root-relative object
prefix for each env-on, env-off, fail-mode, and restored-service check.

`miniflux/M-1` uses `postgres-feed-row` with `per workload Setup call` scope:
each setup creates a feed using a unique RSS URL, so env-off and fail-mode
refresh checks cannot pass only because the env-on refresh already created
entries.

Open questions:
The filesystem target still needs the exact root/prefix policy once the
reconstructor and workload are implemented.

## Stage 10 - Cleanup And Behavioral Equivalence

Assertion:
The full proof path completes with transcript comparison or a declared
behavioral invariant plus normalizer.

Generated artifact:
Baseline and lifted transcripts, normalizer output, cleanup logs, and coverage
report entry.

Target toggles:
Transcript normalizers, behavioral predicates, fresh resource policy, and
cleanup hooks.

Allowed substitutions:
Declared behavioral invariants are allowed for side-effect-heavy persistence
targets when direct response comparison is not the correctness signal. They must
be target-specific and must preserve the stage 8 `/calls` evidence.

Open questions:
Miniflux likely needs ID/timestamp normalization plus the localized-error
expectation. PocketBase thumbnail work likely needs path and image metadata
normalization.
