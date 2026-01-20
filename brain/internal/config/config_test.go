package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Use temp dir to avoid touching real config
	tmpDir := t.TempDir()

	// Set HOME for Unix or USERPROFILE for Windows
	// os.UserHomeDir() checks different env vars per platform
	if runtime.GOOS == "windows" {
		originalUserProfile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tmpDir)
		defer os.Setenv("USERPROFILE", originalUserProfile)
	} else {
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)
	}

	// Clear any env overrides
	os.Unsetenv("POLYBIUS_MINER_DIR")
	os.Unsetenv("POLYBIUS_DB")
	os.Unsetenv("POLYBIUS_CEDICT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults use home dir
	expectedMinerDir := filepath.Join(tmpDir, "Music", "Miner")
	if cfg.MinerDir != expectedMinerDir {
		t.Errorf("MinerDir = %q, want %q", cfg.MinerDir, expectedMinerDir)
	}

	expectedDBPath := filepath.Join(tmpDir, ".polybius", "brain.db")
	if cfg.DBPath != expectedDBPath {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, expectedDBPath)
	}

	expectedCEDICTPath := filepath.Join(tmpDir, ".polybius", "cedict_ts.u8")
	if cfg.CEDICTPath != expectedCEDICTPath {
		t.Errorf("CEDICTPath = %q, want %q", cfg.CEDICTPath, expectedCEDICTPath)
	}
}
