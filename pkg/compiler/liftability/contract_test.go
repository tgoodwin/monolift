package liftability

import "testing"

func TestContractDetectors(t *testing.T) {
	ctx, errorLastHold := testContextAndOperation(t, "RequestReply")
	_, errorLastViolate := testContextAndOperation(t, "NoError")
	assertDetectorVerdict(t, contractErrorLastDetector{}, ctx, errorLastHold, VerdictHold)
	assertDetectorVerdict(t, contractErrorLastDetector{}, ctx, errorLastViolate, VerdictViolate)
	assertDetectorVerdict(t, contractErrorLastDetector{}, ctx, Operation{}, VerdictUnknown)

	_, panicViolate := testContextAndOperation(t, "PanicOnly")
	assertDetectorVerdict(t, contractNoPanicOnlyFailureDetector{}, ctx, errorLastHold, VerdictHold)
	assertDetectorVerdict(t, contractNoPanicOnlyFailureDetector{}, ctx, panicViolate, VerdictViolate)
	assertDetectorVerdict(t, contractNoPanicOnlyFailureDetector{}, ctx, Operation{}, VerdictHold)

	_, readOnly := testMethodContextAndOperation(t, "(*Service).ReadOnly")
	_, mutate := testMethodContextAndOperation(t, "(*Service).Mutate")
	assertDetectorVerdict(t, contractReceiverReadOnlyDetector{}, ctx, readOnly, VerdictHold)
	assertDetectorVerdict(t, contractReceiverReadOnlyDetector{}, ctx, mutate, VerdictViolate)
	assertDetectorVerdict(t, contractReceiverReadOnlyDetector{}, ctx, Operation{}, VerdictHold)

	_, builder := testMethodOnType(t, "Builder", "WithName", "(*Builder).WithName")
	assertDetectorVerdict(t, transportReceiverReturnsSelfDetector{}, ctx, readOnly, VerdictHold)
	assertDetectorVerdict(t, transportReceiverReturnsSelfDetector{}, ctx, builder, VerdictViolate)
	assertDetectorVerdict(t, transportReceiverReturnsSelfDetector{}, ctx, Operation{}, VerdictUnknown)
}
