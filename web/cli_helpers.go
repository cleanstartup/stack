package web

import (
	"strings"
	"time"

	"github.com/cleanstartup/stack/cli"
)

type runCommandInput struct {
	Addr      string
	OutputDir string
}

type buildCommandInput struct {
	WorkspaceDir string
	OutputDir    string
}

type devCommandInput struct {
	Addr         string
	WorkspaceDir string
	OutputDir    string
	PollInterval time.Duration
	Child        bool
}

func stringParam(inv *cli.Invocation, fallback string, name string, aliases ...string) string {
	if inv == nil {
		return fallback
	}
	key := cli.Param(name, aliases...)
	value := inv.StringParam(key)
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolParam(inv *cli.Invocation, fallback bool, name string, aliases ...string) bool {
	if inv == nil {
		return fallback
	}
	key := cli.Param(name, aliases...)
	value := strings.TrimSpace(inv.StringParam(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	case "0", "f", "false", "n", "no", "off":
		return false
	default:
		return fallback
	}
}
