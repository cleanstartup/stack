package lit

import "testing"

type fakeNPM struct {
	deps    map[string]string
	devDeps map[string]string
	bins    []string
}

func (f *fakeNPM) AddDependency(name, version string) {
	if f.deps == nil {
		f.deps = map[string]string{}
	}
	f.deps[name] = version
}

func (f *fakeNPM) AddDevDependency(name, version string) {
	if f.devDeps == nil {
		f.devDeps = map[string]string{}
	}
	f.devDeps[name] = version
}

func (f *fakeNPM) RequireBin(name string) {
	f.bins = append(f.bins, name)
}

func TestAddNPMDependenciesDeclaresLit(t *testing.T) {
	fake := &fakeNPM{}
	AddNPMDependencies(fake)
	if _, ok := fake.deps["lit"]; !ok {
		t.Fatalf("expected \"lit\" to be declared as a dependency, got %+v", fake.deps)
	}
}

func TestAddNPMDependenciesNilProjectIsNoop(t *testing.T) {
	// Must not panic — mirrors tailwind's AddNPMDependencies nil-safety,
	// since stageCtx.NPM is nil in every caller today (no orchestrator wires
	// a live plugin.NPM into StageContext yet).
	AddNPMDependencies(nil)
}
