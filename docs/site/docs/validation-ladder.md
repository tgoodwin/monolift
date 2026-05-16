# Stages of evidence

## Research question and result

A research compiler can claim that a function is "liftable" at many
different levels of confidence. The activation analyzer found a path.
The planner produced a verdict. The renderer produced code that
compiles. An image built. A pod came up. A real workload reached the
extracted service. Two transcripts agreed under cleanup. These are
not the same claim, and conflating them is the easiest way to
overstate a research result.

Monolift's validation pipeline addresses this by treating each rung
as a named stage with a specific assertion attached. A target's
reported result is the highest stage it cleared, plus the reason it
stopped. The same numbering is used in the e2e harness, in the
per-target manifest, and in the per-sprint coverage reports, so a
trace's status survives intact across documents.

## The ladder

| Stage | Assertion | Common blockers |
|---|---|---|
| 0 | Cluster, namespace, and baseline image exist. | Kind plumbing, Docker build, fixture ordering. |
| 1 | Baseline (unlifted) deployment is Ready. | Auth setup, migrations, readiness probe shape. |
| 2 | Baseline workload completes against the host. | The host app can actually serve the chosen request. |
| 3 | Activation analysis recovered a path and recommended a cut. | Reverse-import scope, augmentation cost, missing dispatch evidence. |
| 4 | Admission accepted the recommended cut, a plan was built, and the rendered code compiles. | Receiver class, codec classification, missing reconstructor, parent-not-leaf selection. |
| 5 | The extracted artifact builds as a standalone Go binary. | Driver linking, module resolution, generated `main` linkage. |
| 6 | The extracted image loads into the cluster. | Docker/Kind image plumbing. |
| 7 | The lifted Deployment is Ready under the reconstructor startup contract. | Env propagation, database reachability, mount shape, startup probe (`PingContext`). |
| 8 | A real workload through the host's public API reaches the extracted service. | The host workload does not exercise the lifted symbol; result envelope mismatch on `/invoke`. |
| 9 | Env-off and fail-mode behavior matches the declared client policy. | Fail-open/fail-closed mismatch; state from a prior run leaks into the env-off pass. |
| 10 | Transcript comparison passes against fresh resources, or a declared behavioral invariant plus normalizer/substitution holds. | Timestamps, IDs, random salts, side-effect ordering, missing normalizer. |

The ladder splits into four groups.

- **Stages 0-2** are evidence about the baseline, meaning the
  unmodified host application.
- **Stages 3-4** are evidence about the compiler, meaning the
  analyzer and extractor agree the function can be lifted.
- **Stages 5-7** are evidence about deployment, meaning the artifact
  builds, loads, and comes up with a working dependency.
- **Stages 8-10** are evidence about behavior, meaning the lifted
  system, exercised through the host, returns the same answer as the
  unlifted system.

## Why each rung is its own claim

The pattern that motivated the ladder is the gap between "deployed"
and "works". A target that reaches stage 7 has a Ready Deployment
and a `PingContext`-validated database connection. That is a real
milestone. The runtime contract held, the env was propagated
correctly, the image found its dependencies. None of it proves the
lifted code is reached, much less that it returns the same answer as
the host's local execution would.

That distinction is load-bearing. The SPRINT-0049 cohort surfaced
two illustrative cases.

- A target deployed cleanly (stage 7), but the host workload did not
  actually invoke the lifted symbol. It required a configuration knob
  (`GITEA__security__PASSWORD_HASH_ALGO=argon2`) that the fixture did
  not set. Without stage 8 as a distinct rung, the deployment would
  have been counted as proof of a lift that the workload never
  touched.
- A separate target reached stage 7, but the result envelope returned
  by `/invoke` did not match the host's local return shape (a
  nullable `*locale.LocalizedErrorWrapper`). The deploy succeeded,
  the workload ran, but transcript comparison was not yet meaningful.
  Stage 8 is what forces that mismatch into view.

The ladder is therefore not a checklist of operational tasks. It is
a sequence of epistemic gates. Each rung asserts something the prior
rungs did not.

## What stage 10 means

The strongest claim the harness makes is that the lifted and
unlifted systems produce equivalent observable behavior on fresh
resources. The default form of that claim is **transcript
comparison**. The host is exercised through its public API in both
configurations (extracted on and extracted off), and the recorded
request and response transcripts are compared.

Some lifts inherently break byte-identical transcript comparison:
random salts, generated IDs, server-side timestamps. Stage 10
accommodates these through two declared substitutions, both gated by
target metadata.

- **Normalized transcript comparison.** The target declares a
  normalizer (e.g. "redact timestamps, sort by stable key") and the
  comparison runs against the normalized transcripts.
- **Behavioral invariant.** The target declares a property (e.g.
  "feed entries exist for this feed ID", "thumbnail file exists at
  the expected path") and the harness asserts the property in both
  configurations.

Neither substitution is a generic skip. Both must be recorded in the
target's metadata and justified in the stage-binding document for
the sprint. A target that cannot produce one of these signals stops
at stage 9 and reports stage 9. Not stage 10 with a footnote.

## The dormant invariant

One assertion runs orthogonally to the ladder: the **dormant
invariant**. When the lift is configured off, the extracted
Deployment must record no calls and produce no side effects. This is
checked at stage 9 and validated explicitly at sprint closeout:
extracted Deployments do not carry `MONOLIFT_LIFT_*` env vars, and
their `/calls` counter is zero in the env-off pass.

The invariant exists because the failure it guards against is
silent. A stale extracted pod that still receives traffic from a
previous run could agree with the host transcript for the wrong
reason. The dormant check is the cheapest way to keep that failure
mode out of the evidence.

## How the ladder shows up in code

The harness maintains a `StageTracker` that records the active stage
for each running target. The labels mirror the table above:

```
Enter(0, "cluster-ensure")          Enter(5, "build-lifted-images")
Enter(0, "create-namespaces")       Enter(6, "load-lifted-images")
Enter(0, "build-baseline-image")    Enter(7, "lifted-deploy")
Enter(1, "baseline-deploy")         Enter(8, "lifted-workload")
Enter(2, "baseline-workload")       Enter(9, "transcript-compare")
Enter(3, "compile")                 Enter(9, "env-off-fail-modes")
```

A target struct in `test/e2e/targets/<name>/target.go` sets
`StopAtStage` to the highest rung the target is expected to clear.
As a target matures, the field is raised monotonically. Never
skipped forward, never raised in the same change that introduces the
underlying capability. The corpus manifest
(`test/e2e/activation_corpus_traces.yaml`) records the latest stage
each trace cleared, plus the reason it stopped if it stopped early.

## Design principles

**One stage, one assertion.** Each rung asserts something specific.
The phrasing on this page is the same phrasing used in the harness
labels, the per-target struct, and the per-sprint coverage report, so
a trace's status is interpretable without context.

**Monotonic, target by target.** A target is raised one stage at a
time, with a single focused harness run per change. Jumping from
stage 4 to stage 10 in one commit destroys the ability to attribute a
regression to a specific gate.

**Substitution is declared, not implicit.** Stage 10 can be reached
through normalized comparison or a behavioral invariant, but only
when the target's metadata declares the substitution. Untyped skips
are not allowed to wear stage-10 colors.

**No bundled sweeps as proof.** The full corpus is exercised through
focused per-target invocations, not a single regex that runs every
target in one process. Bundled runs have historically masked
target-specific failures and corrupted shared state between targets.
The ladder is enforceable only when each target's evidence is
collected in isolation.
