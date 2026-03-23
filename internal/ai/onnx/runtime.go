//go:build with_onnx

package onnx

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	ortInitOnce sync.Once
	ortInitErr  error
)

// initORT initialises the global ONNX Runtime environment exactly once.
// It resolves the shared library path, extracts the embedded bytes if needed,
// and calls ort.InitializeEnvironment.
func initORT() error {
	ortInitOnce.Do(func() {
		libPath, err := resolveLibPath()
		if err != nil {
			ortInitErr = fmt.Errorf("onnx: locate runtime lib: %w", err)
			return
		}
		ort.SetSharedLibraryPath(libPath)
		if err := ort.InitializeEnvironment(); err != nil {
			ortInitErr = fmt.Errorf("onnx: initialize environment: %w", err)
		}
	})
	return ortInitErr
}

// resolveLibPath returns a filesystem path to libonnxruntime for the current
// platform.  It checks (in order):
//
//  1. macOS .app bundle — Contents/Frameworks/ next to the executable.
//  2. User cache dir — where we extract the embedded bytes on first run.
//  3. Embedded bytes — extracted to the cache dir on demand.
func resolveLibPath() (string, error) {
	// 1. .app bundle (Contents/MacOS/thawts → ../Frameworks/libonnxruntime.dylib)
	if exe, err := os.Executable(); err == nil {
		bundlePath := filepath.Join(filepath.Dir(exe), "..", "Frameworks", ortLibFilename)
		if _, err := os.Stat(bundlePath); err == nil {
			return bundlePath, nil
		}
	}

	// 2 & 3. Cache dir — extract embedded bytes if not already there.
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dest := filepath.Join(cacheDir, "thawts", ortLibFilename)

	if needsExtraction(dest) {
		if err := extractLib(dest); err != nil {
			return "", err
		}
	}
	return dest, nil
}

// needsExtraction returns true when dest is missing or its SHA-256 does not
// match the embedded bytes (i.e. the app was updated).
func needsExtraction(dest string) bool {
	data, err := os.ReadFile(dest)
	if err != nil {
		return true
	}
	destSum := sha256.Sum256(data)
	srcSum := sha256.Sum256(ortLibBytes)
	return destSum != srcSum
}

// extractLib writes ortLibBytes to dest, creating parent directories as needed.
func extractLib(dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("onnx: create cache dir: %w", err)
	}
	log.Printf("onnx: extracting runtime library to %s", dest)
	if err := os.WriteFile(dest, ortLibBytes, 0o755); err != nil {
		return fmt.Errorf("onnx: write runtime lib: %w", err)
	}
	return nil
}
