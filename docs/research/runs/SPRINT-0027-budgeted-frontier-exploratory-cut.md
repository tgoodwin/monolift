# SPRINT-0027 Exploratory Row Cut

Date: 2026-04-28

The exploratory row was not run.

Planned exploratory budget:

- reverse owners: 5,000
- adjacent owners: 10,000
- boundary candidates: 20,000
- depth: 3
- boundary duration: 120s

Reason for cut:

- The small, medium, and large rows all produced nonzero adjacent owners.
- Boundary evidence and generic registration-site counts increased at larger
  budgets.
- Target-specific closeness did not move: no row recovered `connectWebSocket`
  as an external surface, no row found `APIHandlerTrustRequester`, and no row
  found the target registration owner.
- The large row already peaked at 10,570,637,736 bytes RSS and spent 81.590s in
  boundary-frontier work after callgraph, so the exploratory row would be a
  diagnostic-only stress run without evidence that depth 3 is aimed at the
  missing target edge.

Conclusion: the exploratory row is not useful under the plan's condition that
prior rows show target movement.
