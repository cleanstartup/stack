package tailwind

import npmpkg "github.com/cleanstartup/stack/npm"

func AddNPMDependencies(project *npmpkg.Project) {
	if project == nil {
		return
	}
	project.AddDevDependency("tailwindcss", "^4.0.0")
	project.AddDevDependency("@tailwindcss/cli", "^4.0.0")
	project.RequireBin("tailwindcss")
}
