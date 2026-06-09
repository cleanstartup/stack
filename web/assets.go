package web

import (
	assetpkg "github.com/cleanstartup/stack/asset"
	hugosupportpkg "github.com/cleanstartup/stack/hugosupport"
)

type AssetKind = assetpkg.AssetKind

const (
	AssetKindCSS  = assetpkg.AssetKindCSS
	AssetKindJS   = assetpkg.AssetKindJS
	AssetKindFile = assetpkg.AssetKindFile
)

type AssetSource = assetpkg.AssetSource
type AssetWorkspace = assetpkg.AssetWorkspace
type AssetNamer = assetpkg.AssetNamer
type AssetRef = assetpkg.AssetRef
type WatchPathsProvider = assetpkg.WatchPathsProvider
type AssetEntry = assetpkg.AssetEntry
type AssetRegistry = assetpkg.AssetRegistry
type AssetManifest = assetpkg.AssetManifest
type ContentRegistry = hugosupportpkg.ContentRegistry
type LayoutRegistry = hugosupportpkg.LayoutRegistry

var (
	NewAssetRegistry   = assetpkg.NewAssetRegistry
	NewContentRegistry = hugosupportpkg.NewContentRegistry
	NewLayoutRegistry  = hugosupportpkg.NewLayoutRegistry
	WithWatchPaths     = assetpkg.WithWatchPaths
	FromFile           = assetpkg.FromFile
	FromDir            = assetpkg.FromDir
	FromFiles          = assetpkg.FromFiles
	FromFS             = assetpkg.FromFS
	Generated          = assetpkg.Generated
	AssetURL           = assetpkg.AssetURL
	CallerDir          = assetpkg.CallerDir
	SourcePaths        = assetpkg.SourcePaths
	AssetID            = assetpkg.AssetID
	CleanWatchPaths    = assetpkg.CleanWatchPaths
	CopyFile           = assetpkg.CopyFile
	CopyDir            = assetpkg.CopyDir
	CopyFS             = assetpkg.CopyFS
	ListFiles          = assetpkg.ListFiles
)
