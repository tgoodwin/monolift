package passes

import (
	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/stateclass"
	"github.com/tgoodwin/monolift/pkg/compiler/surface"
	"github.com/tgoodwin/monolift/pkg/compiler/transport"
)

func init() {
	extract.RegisterLiftabilityAnalyzer(liftability.ForExtract)
	extract.RegisterShapeClassifier(transport.ForExtract)
	extract.RegisterShapeValidator(transport.ValidatePragmaOptions)
	extract.RegisterStateInferer(stateclass.ForExtract)
	extract.RegisterSeamDetector(stateclass.ForExtractSeams)
	extract.RegisterSurfaceDeriver(surface.Derive)
}
