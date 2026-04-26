package liftability

func DefaultRegistry() []Detector {
	return []Detector{
		boundaryContextFirstDetector{},
		boundaryVariadicFreeDetector{},
		boundaryNoCallableValuesDetector{},
		boundaryNoStreamingValuesDetector{},
		boundaryNoSyncPrimitivesDetector{},
		boundaryFullyInstantiatedDetector{},
		boundarySerializableViaCustomEncodingDetector{},
		contractErrorLastDetector{},
		effectsNoParamHeapMutationDetector{},
		effectsNoParamEscapeDetector{},
		effectsNoGlobalWritesDetector{},
		effectsNoGlobalReadsDetector{},
		effectsNoParamInterfaceCallbacksDetector{},
		effectsNoReflectUnsafeDetector{},
		effectsNoOSSideEffectsDetector{},
		contractNoPanicOnlyFailureDetector{},
		contractReceiverReadOnlyDetector{},
		lifecycleNoAsyncForkDetector{},
		lifecycleLongRunningLoopDetector{},
		lifecycleExecutionProfileDetector{},
		transportHandlerBoundaryDetector{},
		transportReceiverReturnsSelfDetector{},
	}
}
