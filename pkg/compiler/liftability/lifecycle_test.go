package liftability

import "testing"

func TestLifecycleAndTransportDetectors(t *testing.T) {
	ctx, syncShort := testContextAndOperation(t, "ContextFirst")
	_, asyncFork := testContextAndOperation(t, "AsyncFork")
	_, longRunning := testContextAndOperation(t, "LongRunning")
	_, handler := testContextAndOperation(t, "Handler")

	assertDetectorVerdict(t, lifecycleNoAsyncForkDetector{}, ctx, syncShort, VerdictHold)
	assertDetectorVerdict(t, lifecycleNoAsyncForkDetector{}, ctx, asyncFork, VerdictViolate)
	assertDetectorVerdict(t, lifecycleNoAsyncForkDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, lifecycleLongRunningLoopDetector{}, ctx, longRunning, VerdictHold)
	assertDetectorVerdict(t, lifecycleLongRunningLoopDetector{}, ctx, syncShort, VerdictViolate)
	assertDetectorVerdict(t, lifecycleLongRunningLoopDetector{}, ctx, Operation{}, VerdictUnknown)

	assertDetectorVerdict(t, lifecycleExecutionProfileDetector{}, ctx, syncShort, VerdictHold)
	assertDetectorVerdict(t, lifecycleExecutionProfileDetector{}, ctx, longRunning, VerdictViolate)
	assertDetectorVerdict(t, lifecycleExecutionProfileDetector{}, ctx, asyncFork, VerdictUnknown)

	assertDetectorVerdict(t, transportHandlerBoundaryDetector{}, ctx, handler, VerdictHold)
	assertDetectorVerdict(t, transportHandlerBoundaryDetector{}, ctx, syncShort, VerdictViolate)
	assertDetectorVerdict(t, transportHandlerBoundaryDetector{}, ctx, Operation{}, VerdictUnknown)
}
