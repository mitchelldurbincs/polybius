# Brain Config Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add TOML-based configuration to the Brain component with env var override support.

**Architecture:** New `internal/config` package handles loading from `~/.polybius/config.toml` with fallback to defaults and env var overrides. Main.go simplified to use this single config source.

**Tech Stack:** Go, BurntSushi/toml

---

### Task 1: Add TOML Dependency

**Files:**
- Modify: `brain/go.mod`

**Step 1: Add the dependency**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go get github.com/BurntSushi/toml
```

**Step 2: Verify dependency added**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && grep -q "BurntSushi/toml" go.mod && echo "OK"
```
Expected: `OK`

**Step 3: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/go.mod brain/go.sum && git commit -m "chore: add BurntSushi/toml dependency for config"
```

---

### Task 2: Create Config Package with Tests

**Files:**
- Create: `brain/internal/config/config.go`
- Create: `brain/internal/config/config_test.go`

**Step 1: Write the failing test for default config**

Create `brain/internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Use temp dir to avoid touching real config
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestLoadDefaults -v
```
Expected: FAIL - package doesn't exist

**Step 3: Write minimal config.go to pass the test**

Create `brain/internal/config/config.go`:

```go
// Package config handles loading configuration for the Brain.
package config

import (
	"os"
	"path/filepath"
)

// Config holds all configuration for the Brain.
type Config struct {
	MinerDir   string `toml:"miner_dir"`
	DBPath     string `toml:"db_path"`
	CEDICTPath string `toml:"cedict_path"`
}

// Load loads configuration with priority: defaults < config file < env vars.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		MinerDir:   filepath.Join(homeDir, "Music", "Miner"),
		DBPath:     filepath.Join(homeDir, ".polybius", "brain.db"),
		CEDICTPath: filepath.Join(homeDir, ".polybius", "cedict_ts.u8"),
	}

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestLoadDefaults -v
```
Expected: PASS

**Step 5: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/internal/config/ && git commit -m "feat(config): add config package with default loading"
```

---

### Task 3: Add Config File Loading

**Files:**
- Modify: `brain/internal/config/config.go`
- Modify: `brain/internal/config/config_test.go`

**Step 1: Write the failing test for config file loading**

Add to `brain/internal/config/config_test.go`:

```go
func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestLoadFromFile -v
```
Expected: FAIL - config file values not loaded

**Step 3: Implement config file loading**

Update `brain/internal/config/config.go`:

```go
// Package config handles loading configuration for the Brain.
package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for the Brain.
type Config struct {
	MinerDir   string `toml:"miner_dir"`
	DBPath     string `toml:"db_path"`
	CEDICTPath string `toml:"cedict_path"`
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".polybius", "config.toml"), nil
}

// Load loads configuration with priority: defaults < config file < env vars.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Start with defaults
	cfg := &Config{
		MinerDir:   filepath.Join(homeDir, "Music", "Miner"),
		DBPath:     filepath.Join(homeDir, ".polybius", "brain.db"),
		CEDICTPath: filepath.Join(homeDir, ".polybius", "cedict_ts.u8"),
	}

	// Load from config file if it exists
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			log.Printf("[WARN] Failed to parse config file: %v, using defaults", err)
		}
	}

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestLoadFromFile -v
```
Expected: PASS

**Step 5: Run all config tests**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -v
```
Expected: All PASS

**Step 6: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/internal/config/ && git commit -m "feat(config): add TOML config file loading"
```

---

### Task 4: Add Environment Variable Overrides

**Files:**
- Modify: `brain/internal/config/config.go`
- Modify: `brain/internal/config/config_test.go`

**Step 1: Write the failing test for env var overrides**

Add to `brain/internal/config/config_test.go`:

```go
func TestEnvVarOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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

	// Env vars should win
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestEnvVarOverrides -v
```
Expected: FAIL - env vars not overriding

**Step 3: Implement env var overrides**

Update `brain/internal/config/config.go` - replace the `Load` function:

```go
// Load loads configuration with priority: defaults < config file < env vars.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Start with defaults
	cfg := &Config{
		MinerDir:   filepath.Join(homeDir, "Music", "Miner"),
		DBPath:     filepath.Join(homeDir, ".polybius", "brain.db"),
		CEDICTPath: filepath.Join(homeDir, ".polybius", "cedict_ts.u8"),
	}

	// Load from config file if it exists
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			log.Printf("[WARN] Failed to parse config file: %v, using defaults", err)
		}
	}

	// Apply env var overrides (highest priority)
	if v := os.Getenv("POLYBIUS_MINER_DIR"); v != "" {
		cfg.MinerDir = v
	}
	if v := os.Getenv("POLYBIUS_DB"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("POLYBIUS_CEDICT"); v != "" {
		cfg.CEDICTPath = v
	}

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestEnvVarOverrides -v
```
Expected: PASS

**Step 5: Run all config tests**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -v
```
Expected: All PASS

**Step 6: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/internal/config/ && git commit -m "feat(config): add env var overrides"
```

---

### Task 5: Add Tilde Expansion

**Files:**
- Modify: `brain/internal/config/config.go`
- Modify: `brain/internal/config/config_test.go`

**Step 1: Write the failing test for tilde expansion**

Add to `brain/internal/config/config_test.go`:

```go
func TestTildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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

	// Tildes should be expanded
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestTildeExpansion -v
```
Expected: FAIL - tilde not expanded

**Step 3: Implement tilde expansion**

Update `brain/internal/config/config.go` - add helper and update Load:

```go
// Package config handles loading configuration for the Brain.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for the Brain.
type Config struct {
	MinerDir   string `toml:"miner_dir"`
	DBPath     string `toml:"db_path"`
	CEDICTPath string `toml:"cedict_path"`
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".polybius", "config.toml"), nil
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

// Load loads configuration with priority: defaults < config file < env vars.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Start with defaults
	cfg := &Config{
		MinerDir:   filepath.Join(homeDir, "Music", "Miner"),
		DBPath:     filepath.Join(homeDir, ".polybius", "brain.db"),
		CEDICTPath: filepath.Join(homeDir, ".polybius", "cedict_ts.u8"),
	}

	// Load from config file if it exists
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			log.Printf("[WARN] Failed to parse config file: %v, using defaults", err)
		}
	}

	// Apply env var overrides (highest priority)
	if v := os.Getenv("POLYBIUS_MINER_DIR"); v != "" {
		cfg.MinerDir = v
	}
	if v := os.Getenv("POLYBIUS_DB"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("POLYBIUS_CEDICT"); v != "" {
		cfg.CEDICTPath = v
	}

	// Expand tildes in all paths
	cfg.MinerDir = expandTilde(cfg.MinerDir)
	cfg.DBPath = expandTilde(cfg.DBPath)
	cfg.CEDICTPath = expandTilde(cfg.CEDICTPath)

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestTildeExpansion -v
```
Expected: PASS

**Step 5: Run all config tests**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -v
```
Expected: All PASS

**Step 6: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/internal/config/ && git commit -m "feat(config): add tilde expansion for paths"
```

---

### Task 6: Add Default Config File Creation

**Files:**
- Modify: `brain/internal/config/config.go`
- Modify: `brain/internal/config/config_test.go`

**Step 1: Write the failing test for config file creation**

Add to `brain/internal/config/config_test.go`:

```go
func TestCreatesDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
```

Also add `"strings"` to the test file imports.

**Step 2: Run test to verify it fails**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestCreatesDefaultConfig -v
```
Expected: FAIL - config file not created

**Step 3: Implement Save function and auto-creation**

Update `brain/internal/config/config.go` - add Save and update Load:

```go
// Package config handles loading configuration for the Brain.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for the Brain.
type Config struct {
	MinerDir   string `toml:"miner_dir"`
	DBPath     string `toml:"db_path"`
	CEDICTPath string `toml:"cedict_path"`
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".polybius", "config.toml"), nil
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

// Save writes the config to the config file.
func (c *Config) Save() error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(cfgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(c)
}

// Load loads configuration with priority: defaults < config file < env vars.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Start with defaults (using tilde for portability in saved config)
	cfg := &Config{
		MinerDir:   "~/Music/Miner",
		DBPath:     "~/.polybius/brain.db",
		CEDICTPath: "~/.polybius/cedict_ts.u8",
	}

	// Load from config file if it exists
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(cfgPath); err == nil {
		// File exists, try to load it
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			log.Printf("[WARN] Failed to parse config file: %v, using defaults", err)
		}
	} else if os.IsNotExist(err) {
		// File doesn't exist, create it with defaults
		if err := cfg.Save(); err != nil {
			log.Printf("[WARN] Failed to create default config: %v", err)
		} else {
			log.Printf("[OK] Created default config at %s", cfgPath)
		}
	}

	// Apply env var overrides (highest priority)
	if v := os.Getenv("POLYBIUS_MINER_DIR"); v != "" {
		cfg.MinerDir = v
	}
	if v := os.Getenv("POLYBIUS_DB"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("POLYBIUS_CEDICT"); v != "" {
		cfg.CEDICTPath = v
	}

	// Expand tildes in all paths
	cfg.MinerDir = expandTilde(cfg.MinerDir)
	cfg.DBPath = expandTilde(cfg.DBPath)
	cfg.CEDICTPath = expandTilde(cfg.CEDICTPath)

	// Restore defaults if still using tilde format
	if cfg.MinerDir == "~/Music/Miner" {
		cfg.MinerDir = filepath.Join(homeDir, "Music", "Miner")
	}
	if cfg.DBPath == "~/.polybius/brain.db" {
		cfg.DBPath = filepath.Join(homeDir, ".polybius", "brain.db")
	}
	if cfg.CEDICTPath == "~/.polybius/cedict_ts.u8" {
		cfg.CEDICTPath = filepath.Join(homeDir, ".polybius", "cedict_ts.u8")
	}

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -run TestCreatesDefaultConfig -v
```
Expected: PASS

**Step 5: Run all config tests**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./internal/config -v
```
Expected: All PASS

**Step 6: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/internal/config/ && git commit -m "feat(config): auto-create default config file"
```

---

### Task 7: Integrate Config into main.go

**Files:**
- Modify: `brain/cmd/polybius/main.go`

**Step 1: Update imports and add config loading**

Replace the contents of `brain/cmd/polybius/main.go`:

```go
// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mitchelldurbin/polybius/brain/internal/brain"
	"github.com/mitchelldurbin/polybius/brain/internal/config"
	"github.com/mitchelldurbin/polybius/brain/internal/gym"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
	"github.com/mitchelldurbin/polybius/brain/internal/vocab"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym|vocab>")
		os.Exit(1)
	}

	// Load config once at startup
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "brain":
		runBrain(cfg)
	case "gym":
		runGym(cfg)
	case "vocab":
		runVocab(cfg)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBrain(cfg *config.Config) {
	// Ensure directories exist
	os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)

	brainCfg := brain.Config{
		DBPath:     cfg.DBPath,
		CEDICTPath: cfg.CEDICTPath,
		MinerDir:   cfg.MinerDir,
	}

	svc, err := brain.NewService(brainCfg)
	if err != nil {
		log.Fatalf("Failed to start Brain: %v", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go svc.Start()

	fmt.Println("The Brain is running. Press Ctrl+C to stop.")
	<-sigChan

	fmt.Println("\nShutting down...")
	svc.Stop()
}

func runGym(cfg *config.Config) {
	db, err := storage.OpenDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check for drafts first - show triage if any exist
	draftCount, err := db.CountDraftCards()
	if err != nil {
		log.Printf("Warning: failed to count draft cards: %v", err)
		draftCount = 0
	}
	if draftCount > 0 {
		triageModel := gym.NewTriageModel(db)
		p := tea.NewProgram(triageModel)

		finalModel, err := p.Run()
		if err != nil {
			log.Fatalf("Error running triage: %v", err)
		}

		// Check if user wants to continue to review
		if tm, ok := finalModel.(gym.TriageModel); ok && !tm.StartReview() {
			return // User quit without wanting review
		}
	}

	// Start review session
	session := gym.NewSession(db)
	cards, err := session.GetDueCards(20)
	if err != nil {
		log.Fatalf("Failed to get cards: %v", err)
	}

	if len(cards) == 0 {
		fmt.Println("No cards due for review. Great job!")
		return
	}

	// Create rating callback
	onRate := func(cardID int64, rating int) error {
		return session.SubmitRating(cardID, rating)
	}

	model := gym.NewModel(cards, onRate)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

func runVocab(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: polybius vocab <import> [args]")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "import":
		runVocabImport(cfg)
	default:
		fmt.Printf("Unknown vocab command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func runVocabImport(cfg *config.Config) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: polybius vocab import <file.tsv>")
		os.Exit(1)
	}

	filePath := os.Args[3]

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)

	db, err := storage.OpenDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	result, err := vocab.ImportTSV(db, filePath)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Printf("\nImported %d words (%d new, %d already known)\n",
		result.Added+result.Skipped, result.Added, result.Skipped)
}
```

**Step 2: Run the build**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go build -o bin/polybius ./cmd/polybius
```
Expected: Build succeeds

**Step 3: Run all tests**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./...
```
Expected: All PASS

**Step 4: Commit**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git add brain/cmd/polybius/main.go && git commit -m "refactor: integrate config package into main.go"
```

---

### Task 8: Final Verification

**Step 1: Clean build**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go clean && go build -o bin/polybius ./cmd/polybius
```
Expected: Build succeeds

**Step 2: Run all tests**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && go test ./... -v
```
Expected: All PASS

**Step 3: Test the binary manually**

Run:
```bash
cd C:\Users\mitchell.durbin\source\repos\polybius\brain && ./bin/polybius brain &
sleep 2
kill %1
```
Expected: "The Brain is running" message appears, config file created if not exists

**Step 4: Final commit with all changes**

```bash
cd C:\Users\mitchell.durbin\source\repos\polybius && git status
```
Expected: Working tree clean (all changes committed)
