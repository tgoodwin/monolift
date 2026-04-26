// Package liftability classifies exposed operations by named liftability
// properties rather than by literal transport archetype.
//
// Gate 3.1 callgraph note:
// SPRINT-0009 compared reusing CHA with introducing an RTA-only path for
// reachable-from-root package checks. The implementation keeps CHA as the
// default callgraph because it is already built deterministically in the
// extraction pipeline, does not require a second root-selection pass for every
// operation, and is sufficient for the conservative detectors that land in
// this sprint. RTA remains available in the extraction package for later
// narrower closure work, but the liftability package uses CHA for predictable
// cost and stable output ordering.
package liftability
