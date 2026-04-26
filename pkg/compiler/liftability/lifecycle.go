package liftability

import (
	"go/types"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
)

type lifecycleNoAsyncForkDetector struct{}

func (lifecycleNoAsyncForkDetector) ID() PropertyID { return PropertyLifecycleNoAsyncFork }

func (lifecycleNoAsyncForkDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyLifecycleNoAsyncFork, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if !facts.hasGo {
		return VerdictHold, []Evidence{bodyEvidence(PropertyLifecycleNoAsyncFork, VerdictHold, SourceSSA, "body does not spawn goroutines")}, nil
	}
	return VerdictViolate, []Evidence{bodyEvidence(PropertyLifecycleNoAsyncFork, VerdictViolate, SourceSSA, "body spawns goroutine")}, nil
}

type lifecycleLongRunningLoopDetector struct{}

func (lifecycleLongRunningLoopDetector) ID() PropertyID { return PropertyLifecycleLongRunningLoop }

func (lifecycleLongRunningLoopDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyLifecycleLongRunningLoop, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if facts.hasLoopBackEdge && facts.hasReceive {
		return VerdictHold, []Evidence{bodyEvidence(PropertyLifecycleLongRunningLoop, VerdictHold, SourceSSA, "loop back-edge with receive/select observed")}, nil
	}
	return VerdictViolate, []Evidence{bodyEvidence(PropertyLifecycleLongRunningLoop, VerdictViolate, SourceSSA, "no long-running receive loop observed")}, nil
}

type lifecycleExecutionProfileDetector struct{}

func (lifecycleExecutionProfileDetector) ID() PropertyID { return PropertyLifecycleExecutionProfile }

func (lifecycleExecutionProfileDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyLifecycleExecutionProfile, VerdictUnknown, SourceSSA, "detail=unknown")}, nil
	}
	if facts.hasLoopBackEdge && facts.hasReceive {
		return VerdictViolate, []Evidence{bodyEvidence(PropertyLifecycleExecutionProfile, VerdictViolate, SourceSSA, "detail=long-running")}, nil
	}
	if !facts.hasGo {
		return VerdictHold, []Evidence{bodyEvidence(PropertyLifecycleExecutionProfile, VerdictHold, SourceSSA, "detail=sync-short")}, nil
	}
	return VerdictUnknown, []Evidence{bodyEvidence(PropertyLifecycleExecutionProfile, VerdictUnknown, SourceSSA, "detail=unknown")}, nil
}

type transportHandlerBoundaryDetector struct{}

func (transportHandlerBoundaryDetector) ID() PropertyID { return PropertyTransportHandlerBoundary }

func (transportHandlerBoundaryDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyTransportHandlerBoundary, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	if matchesNetHTTPHandler(ctx.Loaded, op.Signature) {
		return VerdictHold, []Evidence{bodyEvidence(PropertyTransportHandlerBoundary, VerdictHold, SourceTypes, "signature matches net/http handler")}, nil
	}
	if matchesCaddyMiddlewareHandler(ctx.Loaded, op.Signature) {
		return VerdictHold, []Evidence{bodyEvidence(PropertyTransportHandlerBoundary, VerdictHold, SourceTypes, "signature matches caddyhttp.MiddlewareHandler")}, nil
	}
	return VerdictViolate, []Evidence{bodyEvidence(PropertyTransportHandlerBoundary, VerdictViolate, SourceTypes, "signature does not match known handler boundary")}, nil
}

func matchesNetHTTPHandler(loaded *extract.LoadedModule, signature *types.Signature) bool {
	if signature == nil || signature.Params().Len() != 2 || signature.Results().Len() != 0 {
		return false
	}
	responseWriterType, requestPtrType, ok := netHTTPTypes(loaded)
	if !ok {
		return false
	}
	return types.Identical(signature.Params().At(0).Type(), responseWriterType) &&
		types.Identical(signature.Params().At(1).Type(), requestPtrType)
}

func matchesCaddyMiddlewareHandler(loaded *extract.LoadedModule, signature *types.Signature) bool {
	caddySignature, ok := caddyServeHTTPSignature(loaded)
	if !ok {
		return false
	}
	return types.Identical(signature, caddySignature)
}
