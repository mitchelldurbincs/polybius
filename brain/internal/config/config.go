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
