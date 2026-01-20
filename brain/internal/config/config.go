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
