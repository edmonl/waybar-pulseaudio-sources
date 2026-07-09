// Package runtimepath resolves paths under the process runtime directory.
package runtimepath

import (
	"fmt"
	"os"
	"path/filepath"
)

// RuntimeDirJoin returns name inside XDG_RUNTIME_DIR, TMPDIR, or /tmp.
func RuntimeDirJoin(name string) (string, error) {
	envName := "XDG_RUNTIME_DIR"
	runtimeDir := os.Getenv(envName)
	if runtimeDir == "" {
		envName = "TMPDIR"
		runtimeDir = os.Getenv(envName)
	}

	if runtimeDir == "" {
		runtimeDir = "/tmp"
		envName = runtimeDir
	} else if !filepath.IsAbs(runtimeDir) {
		return "", fmt.Errorf("%v must be an absolute path", envName)
	}

	info, err := os.Stat(runtimeDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve runtime directory %v: %w", envName, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%v must be a directory", envName)
	}

	return filepath.Join(runtimeDir, name), nil
}

// WorkingDirJoin resolves path against the current working directory when it is relative.
func WorkingDirJoin(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, path), nil
}
