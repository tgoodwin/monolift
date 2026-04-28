package emit

import (
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/surface"
	"github.com/tgoodwin/monolift/pkg/compiler/transport"
)

func TestTemplateForSurfaceDispatchesSessionToStreamProxy(t *testing.T) {
	got := TemplateForSurface(surface.RegionSurface{
		Category:     surface.SurfaceSession,
		WireProtocol: surface.WireProtocolStreamProxy,
	})
	if got != transport.TemplateStreamProxy {
		t.Fatalf("TemplateForSurface=%q want %q", got, transport.TemplateStreamProxy)
	}
}
