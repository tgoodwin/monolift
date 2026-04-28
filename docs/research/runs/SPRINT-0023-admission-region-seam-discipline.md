# SPRINT-0023 admission region-seam discipline note

## Hypothesis

Multi-root regions need an admission combiner that preserves each root's existing
ADR-0017 verdict and additionally verifies that channel seams stay inside the
selected region. This is not a new Layer-1 property and does not relax any
single-root admission rule.

## Discipline checklist

(a) Boundary moved: only the region-level combiner in
`pkg/compiler/transport/admission.go`.

(b) Observed gap: SPRINT-0022 already accepted Hub/WebConn only when the channel
field seam endpoints are both within the composite region; the previous
single-root admission helper had no representation for that region-level check.

(c) Smallest rule: per-root refusals are preserved verbatim; the new check only
adds refusal reasons for empty or escaping `ChannelField` seam endpoints.

(d) Regression coverage: `TestRegionAdmitsInRegionChannelSeam` and
`TestRegionAdmitsRejectsEscapingChannelSeam` cover the admitted and refused
region seam cases; legacy `Admit` tests still pass.

(e) Exit criterion: no new `liftability.PropertyID` constants and no change to
ADR-0018 Layer-1 properties. Verified in G.6.
