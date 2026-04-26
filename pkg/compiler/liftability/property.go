package liftability

import (
	"sort"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type PropertyID string

const (
	PropertyBoundaryContextFirst                  PropertyID = "boundary.context-first"
	PropertyBoundaryVariadicFree                  PropertyID = "boundary.variadic-free"
	PropertyBoundaryNoCallableValues              PropertyID = "boundary.no-callable-values"
	PropertyBoundaryNoStreamingValues             PropertyID = "boundary.no-streaming-values"
	PropertyBoundaryNoSyncPrimitives              PropertyID = "boundary.no-sync-primitives"
	PropertyBoundaryFullyInstantiated             PropertyID = "boundary.fully-instantiated"
	PropertyBoundarySerializableViaCustomEncoding PropertyID = "boundary.serializable-via-custom-encoding"
	PropertyContractErrorLast                     PropertyID = "contract.error-last"
	PropertyEffectsNoParamHeapMutation            PropertyID = "effects.no-param-heap-mutation"
	PropertyEffectsNoParamEscape                  PropertyID = "effects.no-param-escape"
	PropertyEffectsNoGlobalWrites                 PropertyID = "effects.no-global-writes"
	PropertyEffectsNoGlobalReads                  PropertyID = "effects.no-global-reads"
	PropertyEffectsNoParamInterfaceCallbacks      PropertyID = "effects.no-param-interface-callbacks"
	PropertyEffectsNoReflectUnsafe                PropertyID = "effects.no-reflect-unsafe"
	PropertyEffectsNoOSSideEffects                PropertyID = "effects.no-os-side-effects"
	PropertyContractNoPanicOnlyFailure            PropertyID = "contract.no-panic-only-failure"
	PropertyContractReceiverReadOnly              PropertyID = "contract.receiver-read-only"
	PropertyLifecycleNoAsyncFork                  PropertyID = "lifecycle.no-async-fork"
	PropertyLifecycleLongRunningLoop              PropertyID = "lifecycle.long-running-loop"
	PropertyLifecycleExecutionProfile             PropertyID = "lifecycle.execution-profile"
	PropertyTransportHandlerBoundary              PropertyID = "transport.handler-boundary"
	PropertyTransportReceiverReturnsSelf          PropertyID = "transport.receiver-returns-self"
	// PropertyStateMutexEnclosesStoreInvariant is an archetype-evidence property.
	PropertyStateMutexEnclosesStoreInvariant PropertyID = "state.mutex-encloses-store-invariant"
	// PropertyStateReceiverOwnedState is an archetype-evidence property.
	PropertyStateReceiverOwnedState PropertyID = "state.receiver-owned-state"
	// PropertyStateKeyedAccessInvariant is an archetype-evidence property.
	PropertyStateKeyedAccessInvariant PropertyID = "state.keyed-access-invariant"
)

type Verdict string

const (
	VerdictHold    Verdict = "Hold"
	VerdictViolate Verdict = "Violate"
	VerdictUnknown Verdict = "Unknown"
)

type Source string

const (
	SourceTypes     Source = "types"
	SourceSSA       Source = "ssa"
	SourceCallgraph Source = "callgraph"
)

const (
	SubjectReceiver = "receiver"
	SubjectBody     = "body"
)

type Evidence struct {
	PropertyID PropertyID
	Subject    string
	Verdict    Verdict
	Source     Source
	Detail     string
}

type Admission string

const (
	AdmissionLiftable    Admission = "liftable"
	AdmissionRefused     Admission = "refused"
	AdmissionUnsupported Admission = "unsupported"
)

type Classification struct {
	Operation   reportv2.SymbolIdentity
	Admission   Admission
	Properties  []Evidence
	RefusalCode string
}

type Result struct {
	Root         Classification
	PerOperation []Classification
	Diagnostics  []extract.Diagnostic
}

func sortEvidence(items []Evidence) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].PropertyID != items[j].PropertyID {
			return items[i].PropertyID < items[j].PropertyID
		}
		if items[i].Subject != items[j].Subject {
			return items[i].Subject < items[j].Subject
		}
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		if items[i].Verdict != items[j].Verdict {
			return items[i].Verdict < items[j].Verdict
		}
		return items[i].Detail < items[j].Detail
	})
}

func toReportEvidence(items []Evidence) []reportv2.PropertyEvidence {
	out := make([]reportv2.PropertyEvidence, 0, len(items))
	for _, item := range items {
		out = append(out, reportv2.PropertyEvidence{
			PropertyID: string(item.PropertyID),
			Subject:    item.Subject,
			Verdict:    string(item.Verdict),
			Source:     string(item.Source),
			Detail:     item.Detail,
		})
	}
	return out
}
