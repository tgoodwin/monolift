package activation

import "testing"

func TestClassifyEdgeAlignmentCoversEveryEdgeKind(t *testing.T) {
	tests := []struct {
		kind EdgeKind
		want EdgeAlignClass
	}{
		{DirectCall, Anti},
		{ConcreteMethodCall, Anti},
		{StructFieldFuncValue, Weak},
		{PackageVarFuncValue, Weak},
		{MapFuncValue, Weak},
		{InterfaceDispatch, Strong},
		{GoroutineLaunch, Anti},
		{HTTPHandlerRegistration, Strong},
		{ChannelFlow, Strong},
		{ClosureCapture, Anti},
		{CallbackRegistration, Strong},
		{StructLiteralFieldAssignment, Weak},
		{Unsupported, Anti},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := classifyEdgeAlignment(tt.kind); got != tt.want {
				t.Fatalf("classifyEdgeAlignment(%s) = %s, want %s", tt.kind, got, tt.want)
			}
		})
	}
}
