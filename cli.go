package stack

import (
	"os"

	"github.com/cleanstartup/stack/cli"
)

// CLIModule is a standalone CLI target. Create one with CLI() and run it with CLIApp().
type CLIModule interface {
	CLIApp()
}

// CLI composes commands into a CLIModule. Commands may be plain activities or
// groups created with cli.Group().
//
// Example:
//
//	func CLIModule() stack.CLIModule {
//	    return stack.CLI(
//	        cli.Group("setup", checkCmd, encryptionCmd),
//	        cli.Group("user",  importCmd, syncCmd),
//	    )
//	}
//
//	func main() { myapp.CLIModule().CLIApp() }
func CLI(cmds ...cli.Command) CLIModule {
	return &cliModule{cmds: cmds}
}

type cliModule struct {
	cmds []cli.Command
}

func (m *cliModule) CLIApp() {
	registry := cli.BuildRegistry(m.cmds...)
	runRegistry(registry, os.Args[1:])
}
