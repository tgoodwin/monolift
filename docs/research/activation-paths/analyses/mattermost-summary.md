# Mattermost Summary

## Scope

- Traces analyzed: 15 total; 14 reachable plus `mattermost/M-4` documented as a gap
- Codebase size: ~761k LOC
- Dominant path shapes: server bootstrap, API handler registration, `App` service methods, job workers, notification/email pipelines, and import/export services.

## Shared-State Prevalence

Mattermost is the strongest shared-state codebase in the corpus. Many candidate cuts receive `*App`, `*Server`, API handler, or job-worker receivers that carry stores, config, license state, plugin hooks, cluster services, and caches. Those receivers are scored as `Shared-state` unless the cut reaches an isolated helper such as password hashing or document extraction.

## Callback and Hook Chains

Callback frequency is most visible around webhook, command, notification, and job paths. Plugin/hook-style code tends to keep callbacks or app calls close to the target, so the recommended cuts are often deep service functions where request data has already been normalized and the remote side can reconstruct clients.

## Enterprise Boundary

`mattermost/M-4` is the only structural gap. The enterprise Elasticsearch package would likely offer a client-reconstructible boundary around bulk indexing, but the trace target was not reachable in the call graph, so the analysis records no candidate cut. A valid analysis would require a resolved activation path through the enterprise job registration and indexing client.

## Zero Feasible Cuts

No reachable Mattermost trace has zero feasible cuts under the rubric, but several have only late feasible cuts because shallow API/server candidates are proxy-required or shared-state-heavy. Counting the structural gap, the project has one trace with no recommended cut.

## Synthesis Notes

- Mattermost supplies the corpus's main `Shared-state App Receiver` pattern.
- Deep algorithmic functions (`PBKDF2.Hash`, extraction helpers) behave like pure leaves even inside a very large codebase.
- For Mattermost, state reconstruction usually beats edge alignment as the decisive dimension.
