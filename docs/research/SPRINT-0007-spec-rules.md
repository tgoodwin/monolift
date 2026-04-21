# SPRINT-0007 Spec Rule Checklist

- `AS-FUNC-2` -> unsupported function roots refuse with `MLV2_SHAPE_UNSUPPORTED`.
- `AS-STRUCT-2` -> unfiltered struct surfaces with an unsupported method refuse with `MLV2_STRUCT_SURFACE_UNSUPPORTED`.
- `EC-PRUNE-3` -> bounded-pruning failure refuses with `MLV2_CLOSURE_TOO_LARGE`.
- `SS-CLASS-3` -> unsafe developer widening refuses with `MLV2_STATE_DECL_CONFLICT`.
- `SS-CLASS-4` -> correctness-relevant ambiguity refuses with `MLV2_STATE_UNKNOWN`.
- `SS-DISP-2` -> hidden shared mutation refuses with `MLV2_SHARED_MUTABLE_STATE`.
- `SS-DISP-2` + `SS-LIFT-6` -> embedded durable DB app roots refuse with `MLV2_EMBEDDED_DB_APP_ROOT`.
- `SS-LIFT-4` -> channel/goroutine state stays singleton-only, and channel crossing is refusal-relevant.
- `SS-LIFT-6` -> missing stable affinity for session or connection state refuses with `MLV2_SESSION_AFFINITY_UNAVAILABLE`.
- `SS-WALDO-2` -> remote failures without an error channel refuse with `MLV2_NO_ERROR_CHANNEL`.
- `TA-GRPC-1` -> `transport=grpc` may refuse with `MLV2_TRANSPORT_RESERVED`.
- `TA-HANDLER-1` -> `transport=handler` is only valid for `http-handler` roots; mismatches use `MLV2_SHAPE_UNSUPPORTED` with a rule override.
- `TA-REFUSE-1` -> unsupported canonical transport shapes refuse rather than silently degrade.
- `TA-SER-7` -> channels must not cross the remote boundary; boundary crossings refuse with `MLV2_CHANNEL_BOUNDARY`.
- `TA-SHAPE-1` -> classification order is normative and drives `MLV2_SHAPE_UNSUPPORTED`, `MLV2_NO_ERROR_CHANNEL`, and `MLV2_BUILDER_CHAIN_ROOT`.
