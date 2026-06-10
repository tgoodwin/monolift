# Evaluation ideas — evaluating the compiler itself

> **NOTE: This doc is not very useful. The ideas here are pretty obvious and hard to articulate in a paper.**
> (Tim's opinion)


Working notes for the paper's evaluation. Scope here is **the compiler as a
decision-maker and translator**, not the runtime performance of the lifted
services (that is tracked separately). Performance ideas live elsewhere.

Organizing principle: a compiler makes claims, and an evaluation exists to
substantiate each claim. Each section below states a claim Monolift implicitly
makes, then a concrete experiment, a metric, and a baseline.

Status: brainstorm. Nothing here is committed; refine and cut from here.

---

## 1. "The distributed program behaves like the monolith." (soundness / equivalence)

The most important and most distinctive claim. Most monolith-decomposition tools
only *suggest* boundaries and never guarantee behavior. Monolift actually
compiles, so we can check.

- **Differential equivalence at scale.** Stage 10 already does an oracle compare
  in miniature. Scale it: for every admitted lift, run the monolith and the
  lifted topology on the same inputs and assert identical observable outputs.
  Metric: fraction of admitted lifts that pass under a real input distribution,
  not a single fixed input.
- **Standout idea — use each project's own test suite as the oracle.** listmonk,
  miniflux, and pocketbase ship real test suites written by their authors. After
  lifting, run the *original project's* tests against the lifted topology. Tests
  that pass are equivalence evidence we did not author, which is far more
  credible than our own fixtures. Cheap (tests exist), credible (third-party),
  and it directly attacks the adapter validity gap: if the adapter's
  drain-and-reconstruct corrupts a value, a real test catches it.
- **Fuzz the cut point.** Generate random inputs at the boundary, compare
  monolith vs lifted. Especially pointed for the adapter: does reconstruction
  preserve the value across the full input space, or only on the one PNG fixture?
- **Metamorphic properties.** Determinism, idempotence — things that must hold
  regardless of where the boundary sits.

Threat to validity to state up front: observational equivalence over a test
distribution is not a proof. Acceptable for a systems venue — be honest that it
is empirical, and leave formal semantics to future work.

## 2. "It applies to real code." (coverage)

- For each app, enumerate candidate cut points (functions and interfaces
  reachable from `main` on a request path) and produce a histogram of outcomes:
  admitted directly / recovered via adapter / refused, broken down by refusal
  reason. The honest applicability picture, long tail included.
- Metric is coverage, but the interesting output is the **refusal taxonomy** — it
  tells the reader exactly which shapes the approach cannot handle yet and why.
- Corpus expansion is itself a contribution. Picking a defensible, representative
  set of real Go services and characterizing them is publishable on its own, and
  it addresses the "only 6 apps" external-validity concern directly.

## 3. "When it can't, it refuses honestly rather than lying." (refusal soundness)

The other half of the soundness story, and a genuine differentiator. Pitch:
Monolift never silently emits a wrong distributed program — it either emits an
equivalent one or refuses with a reason.

- **Precision: every admission must survive equivalence.** A single admitted lift
  that fails differential testing is a soundness bug. Zero such cases across the
  corpus is a strong headline result.
- **Recall / conservatism: how many refusals are fundamental vs engineering
  gaps?** For a sample of refusals, have an expert attempt the lift by hand and
  check whether it passes equivalence. A refusal we *could* have lifted is a
  false negative. We won't (and shouldn't) drive these to zero — but
  characterizing them and arguing each is sound-but-conservative is the move.

## 4. "It costs less than doing it by hand." (developer effort)

- Lines of annotation and fraction of code touched per lift, versus the diff for
  a hand-written extraction of the same boundary. The "distribution as a compiler
  pass" thesis lives or dies on annotation cost being far below manual
  refactoring cost.
- A case study (one app: annotation vs the equivalent manual microservice diff)
  is lower-risk than a user study and usually enough for a systems venue.

## 5. "The design choices are justified." (ablations)

The cut-placement analyzer has six scoring dimensions, and `evolution.md`
already records that ordering them wrong (callbacks before surface area)
produced visibly bad cuts. That is an ablation waiting to be written up: disable
or reorder each dimension and measure how often the analyzer still picks the cut
an expert would. Validates the design empirically instead of asserting it.

## 6. "It scales to real codebases." (compile cost)

The gitea 10-minute timeout is a result, not an embarrassment. Characterize how
analysis time grows with codebase size, call-graph depth, and SSA size, and show
where it falls over. Bounds applicability honestly and motivates future work.

## Bonus (a little novel) — compiler stability under behavior-preserving refactoring

Apply renames, reorderings, and extract-function refactors that do not change
semantics, and check that the cut decision does not change. If renaming a
variable flips a lift from admitted to refused, that is a robustness bug in the
classifier. A metamorphic test of the *compiler itself* (not the program) is an
unusual and compelling thing to report for a decision-making compiler.

---

## Baselines

The relevant prior art is automated monolith-to-microservices decomposition
(graph-clustering and ML-based tools — Mono2Micro, CARGO, and similar). Sharp
contrast to draw: those tools *recommend* boundaries and stop; Monolift compiles
the boundary and checks it for behavioral equivalence. A head-to-head on
**decision quality** — do their recommended boundaries even survive a liftability
check? — could be a strong story. A naive in-house baseline (cut at the annotated
function regardless of feasibility) shows the analyzer's value.

---

## What to anchor on

If forced to pick two to anchor the paper: **differential equivalence using the
projects' own test suites** (claim 1) paired with **zero unsound admissions
across the corpus** (claim 3). Together they make the soundness argument that
separates Monolift from the decomposition-suggestion literature, and most of the
infrastructure already exists.

Open question driving the rest: **which central claim does the paper lead with —
soundness, applicability to real code, or low developer effort?** The evaluation
should be built backward from that.
