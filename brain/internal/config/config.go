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
	// Start with defaults using tilde notation for portability
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
		// File exists, load it
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			log.Printf("[WARN] Failed to parse config file: %v, using defaults", err)
		}
	} else if os.IsNotExist(err) {
		// File doesn't exist, create with defaults
		if err := cfg.Save(); err != nil {
			log.Printf("[WARN] Failed to create default config: %v", err)
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
