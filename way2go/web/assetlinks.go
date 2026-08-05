package web

import (
	"net/url"
	"strings"
)

// AssetLinks is the runtime-local view of registered assets: already
// resolved base URLs, no knowledge of stack/asset. Built once at the
// composition boundary (stack's WebApp.Handler()), consumed per-request by
// Page.Render.
type AssetLinks struct {
	Styles  []string
	Scripts []string
}

// withVersion appends a cache-busting query param. Local copy of the same
// ten lines stack/asset uses internally — no reason to import stack for it.
func withVersion(assetURL, version string) string {
	if strings.TrimSpace(version) == "" {
		return assetURL
	}
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return assetURL
	}
	query := parsed.Query()
	query.Set("v", version)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
