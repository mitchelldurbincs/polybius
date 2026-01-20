# Brain Config Integration Design

## Overview

Add TOML-based configuration file support to the Brain component, replacing hardcoded defaults while maintaining backwards compatibility with environment variable overrides.

## Config Location

`~/.polybius/config.toml` - alongside the database

## Config Format

```toml
# Polybius Brain configuration

# Directory where Miner saves captures
miner_dir = "~/Music/Miner"

# SQLite database path
db_path = "~/.polybius/brain.db"

# CC-CEDICT dictionary path
cedict_path = "~/.polybius/cedict_ts.u8"
```

## Loading Priority

From lowest to highest precedence:

1. Hardcoded defaults (same as current)
2. Config file values
3. Environment variables (`POLYBIUS_MINER_DIR`, `POLYBIUS_DB`, `POLYBIUS_CEDICT`)

## Behavior

- On first run, if no config file exists, create one with defaults
- If config file exists but is invalid TOML, log warning and use defaults
- Tilde (`~`) expansion handled for all paths
- Config loaded once at startup in `main.go`, passed to services

## Code Changes

### New Package: `internal/config/config.go`

```go
type Config struct {
    MinerDir   string `toml:"miner_dir"`
    DBPath     string `toml:"db_path"`
    CEDICTPath string `toml:"cedict_path"`
}

func Load() (*Config, error)  // implements priority chain
func (c *Config) Save() error // writes current config to file
```

### Changes to `cmd/polybius/main.go`

- Remove `getEnvOrDefault` helper
- Add single `cfg, err := config.Load()` call early in `main()`
- Pass `cfg.DBPath`, `cfg.MinerDir`, `cfg.CEDICTPath` to existing code
- Remove duplicated path construction from `runBrain()`, `runGym()`, `runVocabImport()`

### Dependencies

- Add `github.com/BurntSushi/toml` for TOML parsing

### Error Handling

- Missing config file → create default, continue
- Invalid TOML → log warning, use defaults
- Invalid path after expansion → fail at service startup (existing behavior)

## Testing

`internal/config/config_test.go` with tests for:

- Loading defaults when no file exists
- Parsing valid TOML file
- Env var overrides take precedence
- Tilde expansion works correctly
- Invalid TOML falls back to defaults

## File Layout

```
brain/
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   └── ...
└── cmd/polybius/main.go
```

## What Stays the Same

- `internal/brain.Config` struct (used internally by brain service)
- All other packages unchanged - they receive paths, don't know about config
- Existing env var names preserved for backwards compatibility
