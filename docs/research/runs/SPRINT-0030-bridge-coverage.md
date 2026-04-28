# SPRINT-0030 Bridge Coverage

Date: 2026-04-28

## Baseline Failure

SPRINT-0029 generic bridge mode selected `connectWebSocket` from reverse-BFS
touchpoints and included it in the function-ref index, but it did not recover
the target Mattermost chain.

Target node status from the SPRINT-0029 validation:

| Target | SPRINT-0029 status |
|---|---|
| `connectWebSocket` | present in SSA, reverse-BFS touchpoints, bridge seed set, and function-ref index; absent from final classification |
| `APIHandlerTrustRequester` | present in SSA only; absent from bridge seed set, function-ref index, and final classification |
| `InitWebSocket` | present in SSA only; absent from bridge seed set, function-ref index, boundary evidence, and final classification |

The preserved conclusion was that the first missing phase was bridge local owner
discovery after start selection.

## Selected Fix Bucket

Boundary-owner discovery fix.

## Evidence

The first SPRINT-0030 diagnostic run confirmed that the target package was
selected but not scanned before bridge discovery stopped. The bridge selected
1,000 starts across 113 packages, scanned 46 packages, scanned 4,685 functions
and 250,000 instructions, admitted 1,075 bridge owners, admitted no boundary
owners, and indexed 1,075 bridge owners.

Target status in that diagnostic:

| Target | Package selected | Package scanned | Owner scanned | Produced bridge seed | Main skip/stop causes |
|---|---:|---:|---:|---:|---|
| `connectWebSocket` | yes | no | no | yes | boundary-owner budget, instruction budget, start budget |
| `APIHandlerTrustRequester` | yes | no | no | no | boundary-owner budget, instruction budget, start budget |
| `InitWebSocket` | yes | no | no | no | boundary-owner budget, instruction budget, start budget |

The ref-match audit showed that the missed owners contained general boundary
evidence, not a Mattermost-only signal:

| Target | Direct touchpoint refs | Boundary evidence | Diagnostic seed result |
|---|---:|---|---|
| `connectWebSocket` | 0 | no | bridge seed |
| `APIHandlerTrustRequester` | 0 | yes, handler-shaped `net/http` evidence | not seeded |
| `InitWebSocket` | 1 | yes, handler-shaped `net/http` evidence | not seeded |

That evidence supports admitting local owners with generic boundary predicate
evidence while scanning selected touchpoint packages. It does not support a
Mattermost-specific recognizer or broader bridge-budget tuning as the primary
fix.

## Algorithm Sketch

The bridge algorithm after this sprint:

1. Derive bridge starts from reverse-BFS touchpoints.
2. Map selected starts to local packages.
3. Scan bounded local owner sets in selected packages.
4. Admit owners that either reference touchpoint functions or carry generic
   boundary predicate evidence.
5. Build the bridge function-ref index from the admitted owner set.
6. Run existing function-value flow and classification over that index.

SPRINT-0030 changed step 4. It did not change the source of starts, introduce
Mattermost-specific recognizers, or broaden bridge mode outside its explicit
mode flag.

## Validation Evidence

The default validation run moved the loss forward. The target package was
selected, scheduled, scanned, and completed for all three oracle targets. All
three target owners were scanned and produced bridge seeds.

| Target | Package scanned | Owner scanned | Produced bridge seed | Function-ref indexed |
|---|---:|---:|---:|---:|
| `connectWebSocket` | yes | yes | yes | no |
| `APIHandlerTrustRequester` | yes | yes | yes | no |
| `InitWebSocket` | yes | yes | yes | no |

The default validation admitted 1,675 bridge owners, including 93 bridge
boundary owners, but indexed 0 of them because the function-ref index budget was
exhausted before scanning the admitted bridge owners.

The `index180` validation then indexed 1,670 bridge owners, including the three
oracle target owners. It recovered final classifications for
`connectWebSocket`, `APIHandlerTrustRequester`, and `InitWebSocket`, and
recovered the main function-value and registration relationships. This confirms
that the boundary-owner discovery fix puts the right owners into the seed set;
the remaining default-budget failure is bridge-index scheduling.

## Next Loss

The next sprint should target bridge-index priority scheduling. The seed set
now contains the known target owners, and a larger index budget proves those
owners are sufficient. The remaining question is how to prioritize admitted
bridge owners so the function-ref index scans high-value bridge and boundary
owners before lower-value owners consume the default budget.
