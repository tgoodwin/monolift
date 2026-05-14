# SPRINT-0043 Retro: Cross-project activation-path coverage

**Date:** 2026-05-09
**Sprint:** SPRINT-0043
**Context:** Reverse-import scoping and 6-project activation-path Kind e2e coverage

## Summary

SPRINT-0043 planted a stake in the ground for every evaluation project. The important result is not that Monolift can lift arbitrary code from each project yet; it is that the activation-path strategy now has a working representative e2e lift across all six corpus projects:

- miniflux
- caddy
- gitea
- listmonk
- PocketBase
- Mattermost

That establishes a credible generalization baseline. Each project now has at least one focused package-level function lift that can be compiled, containerized, deployed into a local Kind cluster, exercised through a real application workload, compared against baseline behavior, and verified through env-off and fail-mode harness checks.

The sprint also exposed the next layer of work. The core strategy generalizes, but the developer loop is still too slow and too rough around the edges. Before expanding aggressively into more lift functions within each project, the next sprint should smooth performance, stability, repeatability, and diagnostics.

## What Worked

Reverse-import scoping was the most important compiler-side improvement. Replacing broad `./...` package loading with scoped transitive importer loading turned large-project activation analysis from a timeout-prone operation into a practical one. Gitea and Mattermost were the strongest proof points.

The Kind e2e harness remained a good forcing function. It caught real packaging, bootstrap, readiness, and workload-path problems that compile-only validation would have missed.

Focused target-by-target debugging worked better than long opaque test runs. Most failures became straightforward once they were classified precisely:

- image pull and registry problems
- app runtime bootstrap mismatches
- missing runtime assets
- readiness and rollout races
- workload paths that did not actually reach the lifted function
- transcript instability from irrelevant dynamic content
- Go workspace and package `TestMain` side effects

Project-specific runtime fixes were more effective than trying to force every app through one generic container shape. Gitea needed its official entrypoint behavior. Listmonk needed asset embedding. PocketBase was more reproducible as a locally built image. Mattermost needed workspace-aware builds, copied runtime assets, valid public-link config, and a real file-link workload.

## Challenges

The original blockers were mostly not conceptual compiler blockers. They were devops and harness fidelity issues. That was good news, but it also means the activation e2e pipeline is now sensitive to app-specific runtime assumptions.

Gitea was the clearest example. Replacing the binary was not enough; the official image's filesystem layout and entrypoint scripts mattered. The successful path preserved that bootstrap model and swapped only the binary path used by the entrypoint.

Listmonk showed that application assets are part of the deploy contract. A plain Go binary was insufficient because the production binary expects embedded config, SQL, query, static, email-template, and i18n assets.

PocketBase exposed external registry fragility. Depending on an anonymously pulled GHCR image made the test nondeterministic. Building the local e2e image from checked-in source is slower than pulling an image, but much more reliable.

Mattermost combined several concerns at once: multi-module workspace behavior, expensive build/analysis phases, package `TestMain` trying to reach local Postgres, missing runtime assets, invalid public-link defaults, and the need for a workload that exercises `GeneratePublicLinkHash` through a real API route.

## Takeaways

The cross-project strategy is viable for simple package-level stateless functions. That is now demonstrated across six materially different codebases.

The next bottleneck is not "can it ever work?" It is "can we make it fast, stable, diagnosable, and cheap enough to use repeatedly?"

The e2e harness is valuable, but it needs to become more operationally predictable. The full combined activation sweep is still pending and may uncover ordering, resource pressure, image-cache, or cleanup issues that focused runs do not show.

The compiler likely has more efficiency wins available. The `./...` import-load fix was impactful because it removed unnecessary work before expensive analysis began. Similar opportunities may exist in later passes, especially around activation analysis, augmentation, package verification, repeated builds, and per-target artifact regeneration.

The right sequencing is to harden the foundation before expanding the target matrix. More lift functions per project will be useful only if the loop is stable enough to make failures actionable.

## Newly Uncovered Risks

Mattermost remains expensive. Even with scoping, the focused e2e run is long enough that it can slow iteration and obscure where time is spent.

Docker and Kind work are now core parts of the verification story. Local sandbox assumptions, image cache state, registry access, and namespace cleanup all affect test reliability.

The deploy contract is still informal in places. Runtime assets, entrypoint behavior, writable data directories, and project-specific setup are currently expressed through target-specific options rather than a more structured model.

Transcript comparison needs careful normalization. Dynamic HTML, cache-buster hashes, generated IDs, and timestamps can produce false negatives unless workloads record behaviorally meaningful fields.

The current proof is still limited to package-level functions with simple serialization boundaries. Receiver methods, stateful functions, callback-heavy paths, and multi-cut lifts remain future work.

## Recommended Follow-up Sprint

The next sprint should focus on performance, stability, and operational polish for the activation-path pipeline.

Suggested intent:

> Make the 6-project activation-path e2e baseline fast, repeatable, and diagnosable enough to support broader lift exploration within each project.

High-value work:

- Run and stabilize the full combined activation sweep.
- Add per-stage timing and artifact timing summaries for the e2e harness.
- Investigate expensive compiler phases after package loading, especially augmentation and patched package verification.
- Cache or reuse work where safe: scoped package maps, Docker layers, local images, generated artifacts, and build contexts.
- Make deployment assumptions more explicit: runtime image, entrypoint, assets, writable dirs, bootstrap commands, and readiness behavior.
- Add better failure snapshots: pod status, events, logs, endpoints, image state, and last workload response.
- Validate generated manifests and dormant extracted deployments as explicit closeout checks.
- Keep focused target runs as the fast path, but make the combined sweep reliable enough for overnight validation.

## Research Questions

Compiler efficiency:

- After reverse-import scoping, which pass dominates elapsed time for each project?
- Is Mattermost slow because of augmentation, package verification, Docker build, app startup, or workload setup?
- Can activation-path analysis reuse loaded package facts across nearby targets in the same project?
- Can patched package verification avoid heavyweight package side effects more systematically than `go test -exec=true`?
- Are there other broad scans equivalent to the old `./...` inefficiency?

Harness stability:

- Does the combined sweep pass on a fresh Kind cluster and on a reused Kind cluster?
- Which targets depend on preexisting Docker cache state?
- Which namespaces, images, or port-forwards survive failed runs?
- Can failure-mode checks be made faster without losing coverage?

Expansion readiness:

- Which additional functions per project are closest to the current proven envelope?
- Which projects have the best next targets for nontrivial parameter shapes?
- Which targets force receiver-method, state, or callback support?
- Where does the package-level stateless strategy stop being representative?

## Proposed Next Milestone

A strong next milestone would be:

1. The full activation sweep passes reliably overnight.
2. Per-target timing is recorded and explains where time is spent.
3. The slowest target has at least one concrete performance improvement or documented bottleneck.
4. Harness failures produce enough Kubernetes and workload context to diagnose without manual rediscovery.
5. The sprint ends with a ranked list of candidate second lifts for each corpus project.

That would turn SPRINT-0043's cross-project proof into a stable foundation for exploring broader lift coverage.
