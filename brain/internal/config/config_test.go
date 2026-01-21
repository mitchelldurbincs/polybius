package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestLoadFromFile(t *testing.T) {
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

	// Clear env overrides
	os.Unsetenv("POLYBIUS_MINER_DIR")
	os.Unsetenv("POLYBIUS_DB")
	os.Unsetenv("POLYBIUS_CEDICT")

	// Create config dir and file
	configDir := filepath.Join(tmpDir, ".polybius")
	os.MkdirAll(configDir, 0755)

	configContent := `
miner_dir = "/custom/miner"
db_path = "/custom/brain.db"
cedict_path = "/custom/cedict.u8"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MinerDir != "/custom/miner" {
		t.Errorf("MinerDir = %q, want %q", cfg.MinerDir, "/custom/miner")
	}
	if cfg.DBPath != "/custom/brain.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/custom/brain.db")
	}
	if cfg.CEDICTPath != "/custom/cedict.u8" {
		t.Errorf("CEDICTPath = %q, want %q", cfg.CEDICTPath, "/custom/cedict.u8")
	}
}

func TestEnvVarOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Set HOME for Unix or USERPROFILE for Windows
	if runtime.GOOS == "windows" {
		originalUserProfile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tmpDir)
		defer os.Setenv("USERPROFILE", originalUserProfile)
	} else {
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)
	}

	// Create config file with values that will be overridden
	configDir := filepath.Join(tmpDir, ".polybius")
	os.MkdirAll(configDir, 0755)

	configContent := `
miner_dir = "/from/file"
db_path = "/from/file.db"
cedict_path = "/from/file.u8"
`
	configPath := filepath.Join(configDir, "config.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Set env vars to override
	os.Setenv("POLYBIUS_MINER_DIR", "/from/env")
	os.Setenv("POLYBIUS_DB", "/from/env.db")
	os.Setenv("POLYBIUS_CEDICT", "/from/env.u8")
	defer func() {
		os.Unsetenv("POLYBIUS_MINER_DIR")
		os.Unsetenv("POLYBIUS_DB")
		os.Unsetenv("POLYBIUS_CEDICT")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Env vars should win over file
	if cfg.MinerDir != "/from/env" {
		t.Errorf("MinerDir = %q, want %q", cfg.MinerDir, "/from/env")
	}
	if cfg.DBPath != "/from/env.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/from/env.db")
	}
	if cfg.CEDICTPath != "/from/env.u8" {
		t.Errorf("CEDICTPath = %q, want %q", cfg.CEDICTPath, "/from/env.u8")
	}
}

func TestTildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	// Set HOME for Unix or USERPROFILE for Windows
	if runtime.GOOS == "windows" {
		originalUserProfile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tmpDir)
		defer os.Setenv("USERPROFILE", originalUserProfile)
	} else {
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)
	}

	// Clear env overrides
	os.Unsetenv("POLYBIUS_MINER_DIR")
	os.Unsetenv("POLYBIUS_DB")
	os.Unsetenv("POLYBIUS_CEDICT")

	// Create config with tilde paths
	configDir := filepath.Join(tmpDir, ".polybius")
	os.MkdirAll(configDir, 0755)

	configContent := `
miner_dir = "~/custom/miner"
db_path = "~/.custom/brain.db"
cedict_path = "~/dict/cedict.u8"
`
	configPath := filepath.Join(configDir, "config.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Tildes should be expanded to actual home dir
	expectedMinerDir := filepath.Join(tmpDir, "custom", "miner")
	if cfg.MinerDir != expectedMinerDir {
		t.Errorf("MinerDir = %q, want %q", cfg.MinerDir, expectedMinerDir)
	}

	expectedDBPath := filepath.Join(tmpDir, ".custom", "brain.db")
	if cfg.DBPath != expectedDBPath {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, expectedDBPath)
	}

	expectedCEDICTPath := filepath.Join(tmpDir, "dict", "cedict.u8")
	if cfg.CEDICTPath != expectedCEDICTPath {
		t.Errorf("CEDICTPath = %q, want %q", cfg.CEDICTPath, expectedCEDICTPath)
	}
}

func TestCreatesDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Set HOME for Unix or USERPROFILE for Windows
	if runtime.GOOS == "windows" {
		originalUserProfile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tmpDir)
		defer os.Setenv("USERPROFILE", originalUserProfile)
	} else {
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)
	}

	// Clear env overrides
	os.Unsetenv("POLYBIUS_MINER_DIR")
	os.Unsetenv("POLYBIUS_DB")
	os.Unsetenv("POLYBIUS_CEDICT")

	// Config file should not exist yet
	configPath := filepath.Join(tmpDir, ".polybius", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("Config file should not exist before Load()")
	}

	// Load should create the default config
	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Config file should now exist
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file should exist after Load(): %v", err)
	}

	// Read and verify contents
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Should contain the key settings
	if !strings.Contains(string(content), "miner_dir") {
		t.Error("Config should contain miner_dir")
	}
	if !strings.Contains(string(content), "db_path") {
		t.Error("Config should contain db_path")
	}
	if !strings.Contains(string(content), "cedict_path") {
		t.Error("Config should contain cedict_path")
	}
}
