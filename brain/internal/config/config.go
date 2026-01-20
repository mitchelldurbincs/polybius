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

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}
