package plugin_test

import (
	"testing"

	"github.com/cleanstartup/stack/plugin"
)

func TestAssetsOfType(t *testing.T) {
	assets := plugin.Assets{
		{Path: "a.css", ContentType: "text/css"},
		{Path: "b.css", ContentType: "text/css+head"},
		{Path: "c.js", ContentType: "application/javascript"},
		{Path: "d.js", ContentType: "application/javascript+module"},
		{Path: "e.js", ContentType: "application/javascript+footer"},
	}

	cases := []struct {
		name        string
		contentType string
		wantPaths   []string
	}{
		{
			name:        "bare matches only unsuffixed",
			contentType: "application/javascript",
			wantPaths:   []string{"c.js"},
		},
		{
			name:        "exact hint matches only that hint",
			contentType: "application/javascript+module",
			wantPaths:   []string{"d.js"},
		},
		{
			name:        "family glob matches bare and every hint",
			contentType: "application/javascript+*",
			wantPaths:   []string{"c.js", "d.js", "e.js"},
		},
		{
			name:        "bare css matches only unsuffixed",
			contentType: "text/css",
			wantPaths:   []string{"a.css"},
		},
		{
			name:        "css family glob matches bare and hinted",
			contentType: "text/css+*",
			wantPaths:   []string{"a.css", "b.css"},
		},
		{
			name:        "no match returns empty",
			contentType: "font/woff2",
			wantPaths:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assets.OfType(tc.contentType)
			if len(got) != len(tc.wantPaths) {
				t.Fatalf("OfType(%q) = %v, want paths %v", tc.contentType, got, tc.wantPaths)
			}
			for i, a := range got {
				if a.Path != tc.wantPaths[i] {
					t.Fatalf("OfType(%q)[%d].Path = %q, want %q", tc.contentType, i, a.Path, tc.wantPaths[i])
				}
			}
		})
	}
}
