# Caddy Annotation Notes - gpt-5.4 run

Catalog links: [distribution-archetypes-v1](../distribution-archetypes-v1.md), [archetype-catalog-v1](../archetype-catalog-v1.md).

## Target Synthesis

Caddy is the control sample. The committed report already admits the reverse-proxy root under `immutable-captured-config`, and the refused stateful internals are exactly the sort of surfaces the sprint must not overclaim. The target contributes one important negative result:

- not every mutex, channel, or atomic is evidence for a remediable distribution archetype

Headline ADMITTED set:

- `modules/caddyhttp/reverseproxy.Handler`

Headline TERMINAL set:

- reverse-proxy hot-path coordination state such as `inFlightRequests` and stream-related handler internals

## Coverage Ledger

| Bundle | Status | Note |
|---|---|---|
| `modules/caddyhttp/reverseproxy` | findings | One admitted root plus terminal counterexample surfaces. |
| remainder of pragma-adjacent Caddy tree | no relevant archetype surface observed | No stronger currently-refused transform candidate than the reverse-proxy hot path inspected here. |

## Region Findings

### Region 1

- `subsystem`: reverse proxy handler root
- `owned directories`: `evaluation/caddy/modules/caddyhttp/reverseproxy`
- `region or operation identity`: `Handler`
- `admitted or refused`: already admitted
- `triage`: `ADMITTED`
- `proposed archetype`: existing replicated handler over immutable captured config
- `proposed candidate state class`: none; committed report assigns `immutable-captured-config`
- `proposed transform`: current handler-style transport and config capture are already sufficient
- `competing archetypes considered`: none
- `evidence signals seen`: committed golden report, admitted pragma root, replicated disposition
- `missing evidence`: none
- `file references`: `test/e2e/targets/caddy/golden/report.json`, `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:102`

### Region 2

- `subsystem`: reverse proxy hot path coordination
- `owned directories`: `evaluation/caddy/modules/caddyhttp/reverseproxy`
- `region or operation identity`: package-global `inFlightRequests` plus mutable stream and connection coordination inside `Handler`
- `admitted or refused`: refused today, and remains an honest v1 refusal
- `triage`: `TERMINAL`
- `proposed archetype`: none survived
- `proposed candidate state class`: none
- `proposed transform`: none; v1 should not pretend this is just a queue or singleton-owner
- `competing archetypes considered`: `serialized-singleton-owner`, `connection-hub-buffer`
- `evidence signals seen`: `sync.Map`, atomics, reflection dispatch, stream lifecycle, serialization-unsupported diagnostics in extract integration tests
- `missing evidence`: a proof that request/stream coordination can be externalized without changing proxy semantics
- `file references`: `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:50`, `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:51`, `pkg/compiler/extract_integration_test.go`
