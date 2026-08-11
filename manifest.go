package stack

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

// manifestFileName is the manifest's path relative to the WebApp target's
// asset OutputDir — both when build writes it to disk and when Main reads
// it back from the embedded release tree (CUP-30).
const manifestFileName = "stack-manifest.json"

// AssetEntry is one WebAppTarget mount, frozen so a release binary can
// rehydrate it without ever calling Build/Consume again: URL is the mount
// point Consume chose (dev), EmbedPath is that same asset's path relative to
// OutputDir (works unchanged against an embed.FS rooted the same way), and
// Kind says how Main should treat it on rehydrate — "css"/"js" feed
// AssetLinks, "mount" is served but not linked.
type AssetEntry struct {
	URL       string `json:"url"`
	EmbedPath string `json:"embedPath"`
	Kind      string `json:"kind"` // css | js | mount
}

// AssetManifest is the only serve-time truth a release binary has: the
// WebApp target's mounts and CSS/JS links, as produced by a `build` run's
// Consume, serialized once so a release `run` never repeats it.
type AssetManifest struct {
	Entries []AssetEntry `json:"entries"`
}

func writeManifest(outputDir string, manifest AssetManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, manifestFileName), data, 0o644)
}

func readManifest(fsys fs.FS) (AssetManifest, error) {
	data, err := fs.ReadFile(fsys, manifestFileName)
	if err != nil {
		return AssetManifest{}, err
	}
	var manifest AssetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return AssetManifest{}, err
	}
	return manifest, nil
}
