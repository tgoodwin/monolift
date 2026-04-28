# Distribution Archetype Research Follow-ups

## Compiler / Classifier Tasks
- **ADR-0016 Implementation**: Add `singleton-mutable-mutex`, `singleton-mutable-sharded`, and `worker-pool-channel` state classes to `pkg/compiler/stateclass/`.
- **Auto-Injection of Context**: For Singleton Actor candidates that violate `boundary.context-first`, implement a transform that auto-injects a background context or propagates it from the caller.
- **Join Analysis for Sharding**: Implement an SSA pass to detect cross-key state interactions, enabling safer promotion of Singleton Actors to Sharded Stateful Services.

## Evidence Signal Refinement
- **Registry Pattern Detection**: Add a signal to detect structs that are primarily keyed maps, justifying the "Sharded" archetype.
- **Closure Serializability**: Improve detection of non-serializable captures in background goroutine closures (critical for Worker Pool auto-lift).

## Target-specific Follow-ups
- **Mattermost God-Object Decomposition**: Research sub-domain boundaries within `server/channels/app` to move it from SUGGEST -> AUTO for sharding.
- **PocketBase Collection Isolation**: Validate if collections in PocketBase are truly isolated enough to allow independent sharded replicas of the `App` component.

## Tooling
- **Extract-Report CLI**: The scratch tool developed for this sprint (`cmd/extract-report`) should be formalized and moved to `pkg/compiler/diag` for future research use.
