package stencil

import npmpkg "github.com/cleanstartup/stack/internal/npm"

func AddNPMDependencies(project *npmpkg.Project) {
	if project == nil {
		return
	}
	project.AddDevDependency("@stencil/core", "4.43.5")
	project.AddDependency("altcha", "^3.0.2")
	project.AddDependency("embla-carousel", "^8.6.0")
	project.AddDependency("embla-carousel-auto-scroll", "^8.6.0")
	project.AddDependency("htmx.org", "^2.0.10")
	project.AddDependency("posthog-js", "^1.379.2")
	project.RequireBin("stencil")
}
