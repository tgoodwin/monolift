package liftability

import "testing"

func TestBoundaryDetectors(t *testing.T) {
	tests := []struct {
		name     string
		detector Detector
		hold     string
		violate  string
		unknown  Operation
	}{
		{"context-first", boundaryContextFirstDetector{}, "ContextFirst", "NoContext", testContextOperationNoParams(t)},
		{"variadic-free", boundaryVariadicFreeDetector{}, "ContextFirst", "Variadic", Operation{}},
		{"no-callable-values", boundaryNoCallableValuesDetector{}, "ContextFirst", "Callable", Operation{}},
		{"no-streaming-values", boundaryNoStreamingValuesDetector{}, "ContextFirst", "Streaming", Operation{}},
		{"no-sync-primitives", boundaryNoSyncPrimitivesDetector{}, "ContextFirst", "SyncPrimitive", Operation{}},
		{"fully-instantiated", boundaryFullyInstantiatedDetector{}, "ContextFirst", "Generic", Operation{}},
		{"serializable-via-custom-encoding", boundarySerializableViaCustomEncodingDetector{}, "CustomJSONHold", "Callable", mustOperation(t, "SerializableUnknown")},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, holdOp := testContextAndOperation(t, tc.hold)
			assertDetectorVerdict(t, tc.detector, ctx, holdOp, VerdictHold)
			_, violateOp := testContextAndOperation(t, tc.violate)
			assertDetectorVerdict(t, tc.detector, ctx, violateOp, VerdictViolate)
			assertDetectorVerdict(t, tc.detector, ctx, tc.unknown, VerdictUnknown)
		})
	}
}

func testContextOperationNoParams(t *testing.T) Operation {
	t.Helper()
	_, op := testContextAndOperation(t, "NoParams")
	return op
}

func mustOperation(t *testing.T, decl string) Operation {
	t.Helper()
	_, op := testContextAndOperation(t, decl)
	return op
}
