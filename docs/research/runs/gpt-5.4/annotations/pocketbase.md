# PocketBase Annotation Notes - gpt-5.4 run

Catalog links: [scheduled-reconciler](../archetype-catalog-v1.md#scheduled-reconciler), [connection-hub-buffer](../archetype-catalog-v1.md#connection-hub-buffer).

## Target Synthesis

PocketBase contributes the sharpest boundary evidence in the corpus. The committed root is terminal not because the codebase lacks interesting distribution shapes, but because the selected root is the wrong granularity: the `core.App` interface is an embedded DB app root with hooks, cron, subscriptions, and multiple DB handles fused together.

Headline TERMINAL set:

- `core.App` root from the committed golden report

Headline narrower AUTO set if lifted at the right cut:

- `tools/cron.Cron`
- `tools/subscriptions.Broker`

Headline SUGGEST set:

- large hook registries under `core.BaseApp`, because they are real singleton-owned registries but the evidence boundary is dominated by root over-selection

## Coverage Ledger

| Bundle | Status | Note |
|---|---|---|
| `core/app.go` and `core/base.go` | findings | Root-level terminal evidence and hook-registry evidence. |
| `tools/cron` | findings | Strong scheduled-reconciler example. |
| `tools/subscriptions` | findings | Narrow connection/fanout broker hidden inside the terminal app root. |
| `apis` and `tools/router` | no relevant archetype surface observed | Mostly request-time handler scaffolding and hook wrappers rather than standalone transforms. |

## Region Findings

### Region 1

- `subsystem`: app root
- `owned directories`: `evaluation/pocketbase/core`
- `region or operation identity`: `App` interface pragma root
- `admitted or refused`: refused today, correctly
- `triage`: `TERMINAL`
- `proposed archetype`: none survived at this granularity
- `proposed candidate state class`: none
- `proposed transform`: none; lift a narrower region instead of the embedded DB app root
- `competing archetypes considered`: `serialized-singleton-owner`, `scheduled-reconciler`, `connection-hub-buffer`
- `evidence signals seen`: committed diagnostics `MLV2_CLOSURE_TOO_LARGE` and `MLV2_EMBEDDED_DB_APP_ROOT`, DB builders on the selected root, root mixes cron, broker, hooks, DB, and filesystem APIs
- `missing evidence`: a narrower selected root
- `file references`: `test/e2e/targets/pocketbase/golden/report.json`, `evaluation/pocketbase/core/app.go:29`

### Region 2

- `subsystem`: cron service
- `owned directories`: `evaluation/pocketbase/tools/cron`
- `region or operation identity`: `Cron`
- `admitted or refused`: currently hidden by the app-root choice, but independently remediable
- `triage`: `AUTO`
- `proposed archetype`: `scheduled-reconciler`
- `proposed candidate state class`: `scheduled-reconciler`
- `proposed transform`: lift the cron loop as a scheduler-owned invocation surface and keep individual jobs as callbacks or enqueued work
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: ticker ownership, job registry, explicit `Start` and `Stop`, time-based triggering clearly separated from work bodies
- `missing evidence`: none once the root is narrowed
- `file references`: `evaluation/pocketbase/tools/cron/cron.go:20`

### Region 3

- `subsystem`: realtime subscriptions
- `owned directories`: `evaluation/pocketbase/tools/subscriptions`
- `region or operation identity`: `Broker`
- `admitted or refused`: currently hidden by the app-root choice, but independently remediable
- `triage`: `AUTO`
- `proposed archetype`: `connection-hub-buffer`
- `proposed candidate state class`: `connection-hub-buffer`
- `proposed transform`: lift the broker as a connection-aware fanout service with explicit client register/unregister and bounded chunk delivery
- `competing archetypes considered`: `serialized-singleton-owner`
- `evidence signals seen`: dedicated client registry, explicit register/unregister operations, broadcast-oriented API surface
- `missing evidence`: none once selected as its own root
- `file references`: `evaluation/pocketbase/tools/subscriptions/broker.go:11`

### Region 4

- `subsystem`: hook registries
- `owned directories`: `evaluation/pocketbase/core`
- `region or operation identity`: `BaseApp` hook fields and registry initialization
- `admitted or refused`: refused today because hook registries are fused to the terminal app root
- `triage`: `SUGGEST`
- `proposed archetype`: `serialized-singleton-owner`
- `proposed candidate state class`: `owned-mutable-singleton`
- `proposed transform`: split hook-host ownership from embedded DB ownership so registry updates and trigger dispatch can be serialized independently
- `competing archetypes considered`: `connection-hub-buffer`
- `evidence signals seen`: one owner struct carries many hook registries; mutations are controlled through initialization and trigger surfaces
- `missing evidence`: an honest narrower root that excludes the embedded DB app boundary
- `file references`: `evaluation/pocketbase/core/base.go:67`, `evaluation/pocketbase/core/base.go:87`
