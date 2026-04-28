# SPRINT-0031 Bridge Index Priority

Date: 2026-04-28

## Baseline

SPRINT-0030 moved the loss from bridge owner discovery to bridge owner indexing.
The default validation selected, scheduled, scanned, and seeded the target
owners, including the boundary owners needed for the known chain. It admitted
1,675 bridge owners and 93 bridge boundary owners, but the function-ref index
scanned 0 bridge owners and skipped all 1,675 under the default validation
budget. The oracle nodes stayed absent from final classification because their
first missing phase was `function_ref_index`.

The `index180` control preserved the same discovery shape but let the
function-ref index scan 1,670 bridge owners, including the target owners. That
run recovered final classification for the three target nodes and recovered the
main `connect-to-api-handler` and `connect-registered-at-init` relationships;
`init-has-http-boundary` still had boundary evidence but no final relationship
record.

The preserved conclusion is that the target owners now enter the bridge seed
set. SPRINT-0031 should prioritize admitted bridge owners before indexing so
high-value bridge and boundary owners are scanned before lower-value owners
consume the default budget.

## Selected Policy

Use one generic bridge-index priority tuple for admitted bridge owners:

1. Boundary bridge owners: bridge owners with boundary seed evidence.
2. Touchpoint-ref bridge owners: bridge owners in a selected touchpoint package
   with direct references to selected touchpoints.
3. Selected-package bridge owners: remaining bridge owners whose package was
   selected from reverse-BFS touchpoints.
4. Other bridge owners.

Within each class, sort by boundary evidence count descending, direct
touchpoint-reference count descending, then package path, object name, function
string, and seed reasons. The policy uses only generic bridge discovery
evidence already collected by the probe: seed reasons, boundary evidence,
selected touchpoint package membership, and direct touchpoint references.

Rejected alternatives:

- Oracle-target priority was rejected because production scheduling must not
  depend on oracle IDs or target names.
- Package-name or route-name priority was rejected because it would encode
  application or framework details.
- Broadly increasing bridge discovery budgets was rejected as the primary fix
  because SPRINT-0030 already admitted the target owners; the loss moved to
  index scheduling.
