package asset

// Mode identifies how an asset is used.
type Mode string

const (
	ModeBuild Mode = "build"
	ModeDev   Mode = "dev"
)

// CommandSpec describes a process the runtime can start for an asset in dev mode.
type CommandSpec struct {
	Binary  string
	Args    []string
	WorkDir string
}
