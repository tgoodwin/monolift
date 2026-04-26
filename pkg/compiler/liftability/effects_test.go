package liftability

import "testing"

func TestEffectsDetectors(t *testing.T) {
	ctx, clean := testContextAndOperation(t, "ContextFirst")
	_, mutate := testContextAndOperation(t, "ParamMutate")
	_, escape := testContextAndOperation(t, "ParamEscape")
	_, globalWrite := testContextAndOperation(t, "GlobalWrite")
	_, globalRead := testContextAndOperation(t, "GlobalRead")
	_, callback := testContextAndOperation(t, "InterfaceCallback")
	_, reflectCall := testContextAndOperation(t, "ReflectCall")
	_, osWrite := testContextAndOperation(t, "OSWrite")

	assertDetectorVerdict(t, effectsNoParamHeapMutationDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoParamHeapMutationDetector{}, ctx, mutate, VerdictViolate)
	assertDetectorVerdict(t, effectsNoParamHeapMutationDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, effectsNoParamEscapeDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoParamEscapeDetector{}, ctx, escape, VerdictViolate)
	assertDetectorVerdict(t, effectsNoParamEscapeDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, effectsNoGlobalWritesDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoGlobalWritesDetector{}, ctx, globalWrite, VerdictViolate)
	assertDetectorVerdict(t, effectsNoGlobalWritesDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, effectsNoGlobalReadsDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoGlobalReadsDetector{}, ctx, globalRead, VerdictViolate)
	assertDetectorVerdict(t, effectsNoGlobalReadsDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, effectsNoParamInterfaceCallbacksDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoParamInterfaceCallbacksDetector{}, ctx, callback, VerdictViolate)
	assertDetectorVerdict(t, effectsNoParamInterfaceCallbacksDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, effectsNoReflectUnsafeDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoReflectUnsafeDetector{}, ctx, reflectCall, VerdictViolate)
	assertDetectorVerdict(t, effectsNoReflectUnsafeDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, effectsNoOSSideEffectsDetector{}, ctx, clean, VerdictHold)
	assertDetectorVerdict(t, effectsNoOSSideEffectsDetector{}, ctx, osWrite, VerdictViolate)
	assertDetectorVerdict(t, effectsNoOSSideEffectsDetector{}, ctx, Operation{}, VerdictUnknown)
}
