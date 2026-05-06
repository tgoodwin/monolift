package activation

func classifyEdgeAlignment(kind EdgeKind) EdgeAlignClass {
	switch kind {
	case InterfaceDispatch, HTTPHandlerRegistration, CallbackRegistration, ChannelFlow:
		return Strong
	case StructFieldFuncValue, StructLiteralFieldAssignment, PackageVarFuncValue, MapFuncValue:
		return Weak
	case DirectCall, ConcreteMethodCall, ClosureCapture, GoroutineLaunch, Unsupported:
		return Anti
	default:
		return Anti
	}
}
