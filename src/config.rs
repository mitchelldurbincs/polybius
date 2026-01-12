//! Configuration management for The Miner

use directories::ProjectDirs;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

/// Main configuration struct
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default)]
    pub general: GeneralConfig,
    #[serde(default)]
    pub hotkeys: HotkeyConfig,
    #[serde(default)]
    pub audio: AudioConfig,
}

/// General application settings
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GeneralConfig {
    /// Directory to save audio files
    #[serde(default = "default_save_directory")]
    pub save_directory: String,
    /// Show notification after save
    #[serde(default = "default_true")]
    pub notifications: bool,
}

/// Hotkey configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HotkeyConfig {
    /// Hotkey for saving 10 seconds
    #[serde(default = "default_hotkey_10s")]
    pub save_10s: String,
    /// Hotkey for saving 30 seconds
    #[serde(default = "default_hotkey_30s")]
    pub save_30s: String,
    /// Hotkey for saving 60 seconds
    #[serde(default = "default_hotkey_60s")]
    pub save_60s: String,
}

/// Audio capture settings
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AudioConfig {
    /// Enable 10 second buffer
    #[serde(default = "default_true")]
    pub buffer_10s: bool,
    /// Enable 30 second buffer
    #[serde(default = "default_true")]
    pub buffer_30s: bool,
    /// Enable 60 second buffer
    #[serde(default)]
    pub buffer_60s: bool,
}

// Default value functions for serde
fn default_save_directory() -> String {
    if let Some(dirs) = directories::UserDirs::new() {
        if let Some(music_dir) = dirs.audio_dir() {
            return music_dir.join("Miner").to_string_lossy().to_string();
        }
    }
    "~/Music/Miner".to_string()
}

fn default_true() -> bool {
    true
}

fn default_hotkey_10s() -> String {
    "CTRL+ALT+1".to_string()
}

fn default_hotkey_30s() -> String {
    "CTRL+ALT+2".to_string()
}

fn default_hotkey_60s() -> String {
    "CTRL+ALT+3".to_string()
}

impl Default for Config {
    fn default() -> Self {
        Self {
            general: GeneralConfig::default(),
            hotkeys: HotkeyConfig::default(),
            audio: AudioConfig::default(),
        }
    }
}

impl Default for GeneralConfig {
    fn default() -> Self {
        Self {
            save_directory: default_save_directory(),
            notifications: true,
        }
    }
}

impl Default for HotkeyConfig {
    fn default() -> Self {
        Self {
            save_10s: default_hotkey_10s(),
            save_30s: default_hotkey_30s(),
            save_60s: default_hotkey_60s(),
        }
    }
}

impl Default for AudioConfig {
    fn default() -> Self {
        Self {
            buffer_10s: true,
            buffer_30s: true,
            buffer_60s: false, // Disabled by default to save memory
        }
    }
}

impl Config {
    /// Get the config file path
    pub fn config_path() -> Option<PathBuf> {
        ProjectDirs::from("", "", "miner").map(|dirs| dirs.config_dir().join("config.toml"))
    }

    /// Load configuration from file, or create default if it doesn't exist
    pub fn load() -> Self {
        let Some(config_path) = Self::config_path() else {
            eprintln!("[WARN] Could not determine config directory, using defaults");
            return Self::default();
        };

        if config_path.exists() {
            match fs::read_to_string(&config_path) {
                Ok(contents) => match toml::from_str(&contents) {
                    Ok(config) => {
                        println!("[OK] Loaded config from {}", config_path.display());
                        return config;
                    }
                    Err(e) => {
                        eprintln!("[WARN] Failed to parse config: {}, using defaults", e);
                    }
                },
                Err(e) => {
                    eprintln!("[WARN] Failed to read config: {}, using defaults", e);
                }
            }
        } else {
            // Create default config file
            let config = Self::default();
            if let Err(e) = config.save() {
                eprintln!("[WARN] Failed to save default config: {}", e);
            } else {
                println!("[OK] Created default config at {}", config_path.display());
            }
            return config;
        }

        Self::default()
    }

    /// Save configuration to file
    pub fn save(&self) -> Result<(), Box<dyn std::error::Error>> {
        let config_path = Self::config_path().ok_or("Could not determine config directory")?;

        // Create parent directories if needed
        if let Some(parent) = config_path.parent() {
            fs::create_dir_all(parent)?;
        }

        let contents = toml::to_string_pretty(self)?;
        fs::write(&config_path, contents)?;
        Ok(())
    }

    /// Get the expanded save directory path
    pub fn save_dir(&self) -> PathBuf {
        let path = &self.general.save_directory;

        // Expand ~ to home directory
        if path.starts_with("~/") {
            if let Some(home) = directories::UserDirs::new() {
                return home.home_dir().join(&path[2..]);
            }
        }

        PathBuf::from(path)
    }

    /// Ensure save directory exists
    pub fn ensure_save_dir(&self) -> Result<PathBuf, std::io::Error> {
        let dir = self.save_dir();
        fs::create_dir_all(&dir)?;
        Ok(dir)
    }
}
