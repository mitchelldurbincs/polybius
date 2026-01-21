//! Tests for configuration management

use miner::config::{CaptureRegion, Config};

// ==================== Default value tests ====================

#[test]
fn test_default_config_has_all_sections() {
    let config = Config::default();

    // Verify all sections exist and have sensible defaults
    assert!(config.general.notifications);
    assert!(config.audio.buffer_5s);
    assert!(config.audio.buffer_10s);
    assert!(config.audio.buffer_15s);
    assert!(config.vision.enabled);
    assert!(config.vision.screenshot_enabled);
    assert!(config.vision.ocr_enabled);
}

#[test]
fn test_default_hotkeys() {
    let config = Config::default();

    assert_eq!(config.hotkeys.save_5s, "CTRL+ALT+1");
    assert_eq!(config.hotkeys.save_10s, "CTRL+ALT+2");
    assert_eq!(config.hotkeys.save_15s, "CTRL+ALT+3");
    assert_eq!(config.hotkeys.screenshot, "CTRL+ALT+S");
    assert_eq!(config.hotkeys.region_select, "CTRL+ALT+R");
}

#[test]
fn test_default_vision_config() {
    let config = Config::default();

    assert_eq!(config.vision.capture_mode, "foreground");
    assert_eq!(config.vision.ocr_language, "en-US");
    assert!(config.vision.window_pattern.is_none());
    assert!(config.vision.regions.is_empty());
}

// ==================== Tilde expansion tests ====================

#[test]
fn test_save_dir_tilde_expansion() {
    let mut config = Config::default();
    config.general.save_directory = "~/Music/TestDir".to_string();

    let expanded = config.save_dir();

    // Should NOT start with ~ after expansion
    assert!(!expanded.to_string_lossy().starts_with("~"));
    // Should end with the path component
    assert!(
        expanded.to_string_lossy().ends_with("Music/TestDir")
            || expanded.to_string_lossy().ends_with("Music\\TestDir")
    );
}

#[test]
fn test_save_dir_absolute_path_unchanged() {
    let mut config = Config::default();

    #[cfg(windows)]
    {
        config.general.save_directory = "C:\\Users\\Test\\Music".to_string();
        let result = config.save_dir();
        assert_eq!(result.to_string_lossy(), "C:\\Users\\Test\\Music");
    }

    #[cfg(not(windows))]
    {
        config.general.save_directory = "/home/test/music".to_string();
        let result = config.save_dir();
        assert_eq!(result.to_string_lossy(), "/home/test/music");
    }
}

#[test]
fn test_save_dir_relative_path() {
    let mut config = Config::default();
    config.general.save_directory = "relative/path".to_string();

    let result = config.save_dir();
    // Relative paths should be unchanged
    assert_eq!(result.to_string_lossy(), "relative/path");
}

#[test]
fn test_save_dir_tilde_only() {
    let mut config = Config::default();
    config.general.save_directory = "~".to_string();

    let result = config.save_dir();
    // Just "~" without slash should remain unchanged
    assert_eq!(result.to_string_lossy(), "~");
}

// ==================== TOML parsing tests ====================

#[test]
fn test_parse_minimal_toml() {
    let toml_str = "";
    let config: Config = toml::from_str(toml_str).unwrap();

    // Should use all defaults
    assert_eq!(config.hotkeys.save_5s, "CTRL+ALT+1");
    assert!(config.audio.buffer_5s);
}

#[test]
fn test_parse_partial_toml() {
    let toml_str = r#"
[hotkeys]
save_5s = "CTRL+SHIFT+1"
"#;
    let config: Config = toml::from_str(toml_str).unwrap();

    // Changed value
    assert_eq!(config.hotkeys.save_5s, "CTRL+SHIFT+1");
    // Default values for others
    assert_eq!(config.hotkeys.save_10s, "CTRL+ALT+2");
    assert_eq!(config.hotkeys.screenshot, "CTRL+ALT+S");
}

#[test]
fn test_parse_full_toml() {
    let toml_str = r#"
[general]
save_directory = "C:/TestPath"
notifications = false

[hotkeys]
save_5s = "F1"
save_10s = "F2"
save_15s = "F3"
screenshot = "F4"
region_select = "F5"

[audio]
buffer_5s = false
buffer_10s = true
buffer_15s = false

[vision]
enabled = true
screenshot_enabled = false
capture_mode = "window"
window_pattern = "Chrome"
ocr_enabled = true
ocr_language = "zh-CN"
metadata_enabled = false
"#;
    let config: Config = toml::from_str(toml_str).unwrap();

    assert_eq!(config.general.save_directory, "C:/TestPath");
    assert!(!config.general.notifications);
    assert_eq!(config.hotkeys.save_5s, "F1");
    assert!(!config.audio.buffer_5s);
    assert!(config.audio.buffer_10s);
    assert_eq!(config.vision.capture_mode, "window");
    assert_eq!(config.vision.window_pattern, Some("Chrome".to_string()));
    assert_eq!(config.vision.ocr_language, "zh-CN");
    assert!(!config.vision.metadata_enabled);
}

#[test]
fn test_parse_toml_with_regions() {
    let toml_str = r#"
[vision.regions.Chrome]
x = 100
y = 200
width = 800
height = 600

[vision.regions."VLC media player"]
x = 0
y = 0
width = 1920
height = 1080
"#;
    let config: Config = toml::from_str(toml_str).unwrap();

    assert_eq!(config.vision.regions.len(), 2);

    let chrome_region = config.vision.regions.get("Chrome").unwrap();
    assert_eq!(chrome_region.x, 100);
    assert_eq!(chrome_region.y, 200);
    assert_eq!(chrome_region.width, 800);
    assert_eq!(chrome_region.height, 600);

    let vlc_region = config.vision.regions.get("VLC media player").unwrap();
    assert_eq!(vlc_region.x, 0);
    assert_eq!(vlc_region.width, 1920);
}

#[test]
fn test_parse_toml_unknown_fields_ignored() {
    // Unknown fields should be silently ignored (serde default behavior)
    let toml_str = r#"
[general]
save_directory = "test"
unknown_field = "should be ignored"
"#;
    let result: Result<Config, _> = toml::from_str(toml_str);
    assert!(result.is_ok());
}

// ==================== Serialization round-trip tests ====================

#[test]
fn test_config_serialization_roundtrip() {
    let original = Config::default();
    let toml_str = toml::to_string_pretty(&original).unwrap();
    let parsed: Config = toml::from_str(&toml_str).unwrap();

    // Values should match after round-trip
    assert_eq!(original.hotkeys.save_5s, parsed.hotkeys.save_5s);
    assert_eq!(original.audio.buffer_5s, parsed.audio.buffer_5s);
    assert_eq!(original.vision.ocr_language, parsed.vision.ocr_language);
}

#[test]
fn test_capture_region_serialization() {
    let region = CaptureRegion {
        x: -50, // Can be negative (off-screen)
        y: 100,
        width: 1920,
        height: 1080,
    };

    let json = serde_json::to_string(&region).unwrap();
    let parsed: CaptureRegion = serde_json::from_str(&json).unwrap();

    assert_eq!(region.x, parsed.x);
    assert_eq!(region.y, parsed.y);
    assert_eq!(region.width, parsed.width);
    assert_eq!(region.height, parsed.height);
}

#[test]
fn test_capture_region_toml_serialization() {
    let region = CaptureRegion {
        x: 0,
        y: 0,
        width: 800,
        height: 600,
    };

    let toml_str = toml::to_string(&region).unwrap();
    let parsed: CaptureRegion = toml::from_str(&toml_str).unwrap();

    assert_eq!(region.x, parsed.x);
    assert_eq!(region.y, parsed.y);
    assert_eq!(region.width, parsed.width);
    assert_eq!(region.height, parsed.height);
}

// ==================== Edge case tests ====================

#[test]
fn test_config_with_empty_strings() {
    let toml_str = r#"
[general]
save_directory = ""

[vision]
window_pattern = ""
"#;
    let config: Config = toml::from_str(toml_str).unwrap();

    assert_eq!(config.general.save_directory, "");
    // Empty string for window_pattern should deserialize as Some("")
    assert_eq!(config.vision.window_pattern, Some("".to_string()));
}

#[test]
fn test_capture_region_zero_dimensions() {
    // Zero dimensions are technically valid
    let region = CaptureRegion {
        x: 0,
        y: 0,
        width: 0,
        height: 0,
    };

    let json = serde_json::to_string(&region).unwrap();
    let parsed: CaptureRegion = serde_json::from_str(&json).unwrap();

    assert_eq!(parsed.width, 0);
    assert_eq!(parsed.height, 0);
}
