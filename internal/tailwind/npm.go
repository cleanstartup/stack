package tailwind

import "github.com/cleanstartup/stack/plugin"

func AddNPMDependencies(project plugin.NPM) {
	if project == nil {
		return
	}
	project.AddDevDependency("tailwindcss", "^4.0.0")
	project.AddDevDependency("@tailwindcss/cli", "^4.0.0")
	project.RequireBin("tailwindcss")
}
