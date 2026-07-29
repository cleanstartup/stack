package config

import (
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

func init() {
	loadDotenvUpwards()
}

func Load[T any]() T {
	cfg, err := env.ParseAs[T]()
	if err != nil {
		panic(err)
	}
	return cfg
}

func loadDotenvUpwards() {
	wd, err := os.Getwd()
	if err != nil {
		_ = godotenv.Load(".env")
		return
	}

	seen := map[string]struct{}{}
	for current := wd; ; current = filepath.Dir(current) {
		candidate := filepath.Join(current, ".env")
		if _, ok := seen[candidate]; !ok {
			_ = godotenv.Load(candidate)
			seen[candidate] = struct{}{}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
}
