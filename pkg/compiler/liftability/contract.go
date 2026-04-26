package liftability

import "go/types"

type contractErrorLastDetector struct{}

func (contractErrorLastDetector) ID() PropertyID { return PropertyContractErrorLast }

func (contractErrorLastDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyContractErrorLast, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	if op.Signature.Results().Len() == 0 {
		return VerdictViolate, []Evidence{bodyEvidence(PropertyContractErrorLast, VerdictViolate, SourceTypes, "signature has no results")}, nil
	}
	if isErrorType(op.Signature.Results().At(op.Signature.Results().Len() - 1).Type()) {
		return VerdictHold, []Evidence{bodyEvidence(PropertyContractErrorLast, VerdictHold, SourceTypes, "terminal result is error")}, nil
	}
	return VerdictViolate, []Evidence{bodyEvidence(PropertyContractErrorLast, VerdictViolate, SourceTypes, "terminal result is not error")}, nil
}

type contractNoPanicOnlyFailureDetector struct{}

func (contractNoPanicOnlyFailureDetector) ID() PropertyID { return PropertyContractNoPanicOnlyFailure }

func (contractNoPanicOnlyFailureDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil || !facts.hasPanic {
		return VerdictHold, []Evidence{bodyEvidence(PropertyContractNoPanicOnlyFailure, VerdictHold, SourceSSA, "no panic instructions observed")}, nil
	}
	if op.Signature != nil && op.Signature.Results().Len() > 0 && isErrorType(op.Signature.Results().At(op.Signature.Results().Len()-1).Type()) {
		return VerdictHold, []Evidence{bodyEvidence(PropertyContractNoPanicOnlyFailure, VerdictHold, SourceSSA, "panic exists but error channel is available")}, nil
	}
	return VerdictViolate, []Evidence{bodyEvidence(PropertyContractNoPanicOnlyFailure, VerdictViolate, SourceSSA, "panic path exists without terminal error result")}, nil
}

type contractReceiverReadOnlyDetector struct{}

func (contractReceiverReadOnlyDetector) ID() PropertyID { return PropertyContractReceiverReadOnly }

func (contractReceiverReadOnlyDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil || op.Signature.Recv() == nil {
		return VerdictHold, []Evidence{bodyEvidence(PropertyContractReceiverReadOnly, VerdictHold, SourceSSA, "operation has no receiver")}, nil
	}
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyContractReceiverReadOnly, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if len(facts.receiverMutations) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyContractReceiverReadOnly, VerdictHold, SourceSSA, "receiver is not mutated")}, nil
	}
	out := make([]Evidence, 0, len(facts.receiverMutations))
	for _, detail := range facts.receiverMutations {
		out = append(out, Evidence{
			PropertyID: PropertyContractReceiverReadOnly,
			Subject:    SubjectReceiver,
			Verdict:    VerdictViolate,
			Source:     SourceSSA,
			Detail:     detail,
		})
	}
	sortEvidence(out)
	return VerdictViolate, out, nil
}

type transportReceiverReturnsSelfDetector struct{}

func (transportReceiverReturnsSelfDetector) ID() PropertyID {
	return PropertyTransportReceiverReturnsSelf
}

func (transportReceiverReturnsSelfDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyTransportReceiverReturnsSelf, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	if op.Signature.Recv() == nil || op.Signature.Results().Len() == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyTransportReceiverReturnsSelf, VerdictHold, SourceTypes, "operation is not a builder-chain candidate")}, nil
	}
	recvType := op.Signature.Recv().Type()
	resultType := op.Signature.Results().At(0).Type()
	if types.AssignableTo(resultType, recvType) || types.AssignableTo(recvType, resultType) {
		return VerdictViolate, []Evidence{bodyEvidence(PropertyTransportReceiverReturnsSelf, VerdictViolate, SourceTypes, "first result is assignable to the receiver type")}, nil
	}
	return VerdictHold, []Evidence{bodyEvidence(PropertyTransportReceiverReturnsSelf, VerdictHold, SourceTypes, "first result does not return receiver/builder self")}, nil
}
