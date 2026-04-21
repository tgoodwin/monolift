package passes

import (
	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/shape"
	"github.com/tgoodwin/monolift/pkg/compiler/stateclass"
)

func init() {
	extract.RegisterShapeClassifier(shape.ForExtract)
	extract.RegisterShapeValidator(shape.ValidatePragmaOptions)
	extract.RegisterStateInferer(stateclass.ForExtract)
}
