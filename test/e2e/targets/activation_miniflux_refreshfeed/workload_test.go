package activation_miniflux_refreshfeed

import (
	"strings"
	"testing"
)

func TestFreshFeedURLIsUniqueFixtureURL(t *testing.T) {
	first := freshFeedURL()
	second := freshFeedURL()
	if first == second {
		t.Fatalf("fresh feed URLs matched: %s", first)
	}
	const prefix = "http://rss-feed-server/index.xml?monolift_resource="
	if !strings.HasPrefix(first, prefix) {
		t.Fatalf("first feed URL %q does not use fixture prefix %q", first, prefix)
	}
	if !strings.HasPrefix(second, prefix) {
		t.Fatalf("second feed URL %q does not use fixture prefix %q", second, prefix)
	}
}
