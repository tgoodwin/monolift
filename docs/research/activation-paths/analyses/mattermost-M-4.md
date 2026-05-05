# mattermost/M-4 - Elasticsearch bulk indexing (`BulkIndexPosts`)

## Header

- Trace ID: `mattermost/M-4`
- Project: `mattermost`
- Region root: `server/enterprise/elasticsearch/common/indexing_job.go:412`
- Path length: 11
- Source trace: `projects/mattermost/traces/M-4.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| - | Gap | target-not-found | - | - | - | - | - | - | Infeasible |

## Recommended Cut

No cut is recommended for `mattermost/M-4` because the trace target is documented as not found in the call graph. A valid cut analysis would require a reachable activation path to `BulkIndexPosts` and source inspection at each concrete dispatch point.

## Tension Notes

The blocker is structural rather than multi-objective: without a resolved path, every candidate cut would be speculative and would not satisfy the sprint's source-backed scoring rule.

## Observations

- The likely analysis would need to distinguish enterprise Elasticsearch indexing job state, app/database access, and bulk indexer client reconstruction.
- This file records the accepted corpus gap and does not attempt to infer a boundary from unreachable graph data.
