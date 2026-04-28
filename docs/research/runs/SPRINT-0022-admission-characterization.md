# SPRINT-0022 admission characterization

## Per-root probe

Initial region admission is the existing per-root transport admission rule applied independently to the Hub root surface and the WebConn root surface, then ANDed at the region level.

The expected Mattermost concern is not a boundary parameter property; it is the inter-root channel seam `WebConn.send`, where Hub-side goroutines write and WebConn-side write-pump code reads.

## Region rule

`RegionAdmits(region, perRootVerdicts, seams)` admits iff:

1. every per-root transport admission verdict admits, and
2. every channel-field seam passes the channel-seam shape check.

Mutex-field and atomic-field seams are recorded as structural metadata in this sprint, but do not participate in the admission verdict.

## Seam-shape hypothesis

A `chan T` seam whose writer-roots and reader-roots are all members of the union closure of the region admits trivially, because lifting the region as one extracted-service binary keeps the channel in-process; the channel is preserved verbatim, no wire crosses, no serialization or backpressure-shape change is introduced.

This is grounded in ADR-0022's composite-region model, where a coherent multi-component region is emitted as one unit, and ADR-0023's cmd-inside-host emission shape, where the extracted service is a single Go binary with ordinary in-process Go values inside the lifted region.

The rule is principled: it applies to any region whose channel-seam endpoints are all lifted into the same extracted service, whether the fixture is Mattermost or a hand-written Go test case.

## Mattermost verdict

The region-level admission verdict accepts. The Hub and WebConn root surfaces pass the existing per-root transport admission probe for the purposes of this branch decision, and the load-bearing channel seam passes the in-region channel-seam check:

- Seam: `WebConn.send`
- Type: `ChannelField`
- Writers: `{Hub}`
- Readers: `{WebConn}`
- Verdict: Hold
- Distribution-feasibility analysis: no serialized wire boundary crosses this channel when Hub and WebConn are emitted as one extracted service.
- Classification: admitted structural seam, not a refusal.

Branch (R) is not caused by admission. The sprint stops later at G.gate-1 on the liftpatch multi-root emission gap documented in `docs/research/runs/SPRINT-0022-emission-gap.md`.
