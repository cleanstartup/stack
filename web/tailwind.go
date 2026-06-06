package web

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (e *BuildEngine) buildStyleBundle(ctx context.Context, workspace *Workspace) error {
	if e == nil || e.builder == nil || e.builder.tailwind == nil || workspace == nil {
		return nil
	}
	if len(e.builder.tailwind.Inputs()) == 0 {
		return nil
	}

	bundle := e.builder.tailwind.BundleRef()
	outputDir := workspace.OutputAssetDir(bundle.Kind, bundle.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, tailwindBundleFile)
	inputPath := filepath.Join(workspace.Temp, "tailwind.input.css")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		return err
	}

	input, err := e.tailwindInput(workspace, inputPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		return err
	}

	if err := runTailwind(ctx, inputPath, outputPath); err != nil {
		return err
	}
	return nil
}

func (e *BuildEngine) tailwindInput(workspace *Workspace, inputPath string) (string, error) {
	if e == nil || e.builder == nil || e.builder.tailwind == nil || workspace == nil {
		return "", nil
	}
	var out strings.Builder
	out.WriteString(`@import "tailwindcss";`)
	out.WriteString("\n")

	for _, scanPath := range e.builder.tailwind.ScanPaths() {
		rel, err := filepath.Rel(filepath.Dir(inputPath), scanPath)
		if err != nil {
			rel = scanPath
		}
		if strings.TrimSpace(rel) == "" {
			continue
		}
		out.WriteString(`@source "`)
		out.WriteString(filepath.ToSlash(rel))
		out.WriteString(`";`)
		out.WriteString("\n")
	}

	for _, source := range e.builder.tailwind.Inputs() {
		if source == nil {
			continue
		}
		names, err := materializeTailwindSource(source, workspace, AssetKindCSS)
		if err != nil {
			return "", err
		}
		sourceDir := workspace.TailwindAssetDir(AssetKindCSS, source.ID())
		for _, name := range names {
			rel, err := filepath.Rel(filepath.Dir(inputPath), filepath.Join(sourceDir, name))
			if err != nil {
				rel = filepath.Join(sourceDir, name)
			}
			out.WriteString(`@import "`)
			out.WriteString(filepath.ToSlash(rel))
			out.WriteString(`";`)
			out.WriteString("\n")
		}
	}

	return out.String(), nil
}

func runTailwind(ctx context.Context, inputPath, outputPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	runners := [][]string{
		{"go", "tool", "tailwind", "-i", inputPath, "-o", outputPath, "--minify"},
		{"tailwindcss", "-i", inputPath, "-o", outputPath, "--minify"},
	}
	var lastErr error
	for _, args := range runners {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("tailwind build failed via %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return lastErr
}

func materializeTailwindSource(source AssetSource, workspace *Workspace, kind AssetKind) ([]string, error) {
	if source == nil {
		return nil, nil
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace is nil")
	}

	switch typed := source.(type) {
	case watchedAssetSource:
		return materializeTailwindSource(typed.source, workspace, kind)
	case fileAssetSource:
		dstDir := workspace.TailwindAssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		name := filepath.Base(typed.path)
		if err := copyFile(filepath.Join(dstDir, name), typed.path); err != nil {
			return nil, err
		}
		return []string{name}, nil
	case dirAssetSource:
		dstDir := workspace.TailwindAssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		return copyDir(dstDir, typed.path)
	case fsAssetSource:
		dstDir := workspace.TailwindAssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		sub, err := fs.Sub(typed.source, typed.root)
		if err != nil {
			return nil, err
		}
		return copyFS(dstDir, sub)
	case generatedAssetSource:
		dstDir := workspace.TailwindAssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		if typed.write == nil {
			return nil, nil
		}
		if err := typed.write(dstDir); err != nil {
			return nil, err
		}
		return listFiles(dstDir)
	default:
		return source.Materialize(workspace, kind)
	}
}
