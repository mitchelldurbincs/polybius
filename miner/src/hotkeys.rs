//! Global hotkey registration and parsing

use global_hotkey::hotkey::{Code, HotKey, Modifiers};
use global_hotkey::GlobalHotKeyManager;

use crate::audio::BufferDuration;

/// Action triggered by a hotkey press
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HotkeyAction {
    /// Save audio buffer with the given duration
    SaveBuffer(BufferDuration),
    /// Take screenshot only (save + clipboard)
    Screenshot,
    /// Open region selection mode
    RegionSelect,
}

/// Hotkey manager for the application
pub struct HotkeyManager {
    _manager: GlobalHotKeyManager,
    pub hotkey_5s: Option<(HotKey, u32)>,
    pub hotkey_10s: Option<(HotKey, u32)>,
    pub hotkey_15s: Option<(HotKey, u32)>,
    pub hotkey_screenshot: Option<(HotKey, u32)>,
    pub hotkey_region: Option<(HotKey, u32)>,
}

impl HotkeyManager {
    /// Create a new hotkey manager and register hotkeys based on config
    #[allow(clippy::too_many_arguments)]
    pub fn new(
        hotkey_5s_str: &str,
        hotkey_10s_str: &str,
        hotkey_15s_str: &str,
        hotkey_screenshot_str: &str,
        hotkey_region_str: &str,
        enabled_5s: bool,
        enabled_10s: bool,
        enabled_15s: bool,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        let manager = GlobalHotKeyManager::new()?;

        let hotkey_5s = if enabled_5s {
            match parse_hotkey(hotkey_5s_str) {
                Ok(hk) => {
                    manager.register(hk)?;
                    println!("[OK] Registered hotkey: {} (5s)", hotkey_5s_str);
                    Some((hk, hk.id()))
                }
                Err(e) => {
                    eprintln!(
                        "[WARN] Failed to parse 5s hotkey '{}': {}",
                        hotkey_5s_str, e
                    );
                    None
                }
            }
        } else {
            None
        };

        let hotkey_10s = if enabled_10s {
            match parse_hotkey(hotkey_10s_str) {
                Ok(hk) => {
                    manager.register(hk)?;
                    println!("[OK] Registered hotkey: {} (10s)", hotkey_10s_str);
                    Some((hk, hk.id()))
                }
                Err(e) => {
                    eprintln!(
                        "[WARN] Failed to parse 10s hotkey '{}': {}",
                        hotkey_10s_str, e
                    );
                    None
                }
            }
        } else {
            None
        };

        let hotkey_15s = if enabled_15s {
            match parse_hotkey(hotkey_15s_str) {
                Ok(hk) => {
                    manager.register(hk)?;
                    println!("[OK] Registered hotkey: {} (15s)", hotkey_15s_str);
                    Some((hk, hk.id()))
                }
                Err(e) => {
                    eprintln!(
                        "[WARN] Failed to parse 15s hotkey '{}': {}",
                        hotkey_15s_str, e
                    );
                    None
                }
            }
        } else {
            None
        };

        // Screenshot hotkey is always enabled
        let hotkey_screenshot = match parse_hotkey(hotkey_screenshot_str) {
            Ok(hk) => {
                manager.register(hk)?;
                println!(
                    "[OK] Registered hotkey: {} (screenshot)",
                    hotkey_screenshot_str
                );
                Some((hk, hk.id()))
            }
            Err(e) => {
                eprintln!(
                    "[WARN] Failed to parse screenshot hotkey '{}': {}",
                    hotkey_screenshot_str, e
                );
                None
            }
        };

        // Region select hotkey is always enabled
        let hotkey_region = match parse_hotkey(hotkey_region_str) {
            Ok(hk) => {
                manager.register(hk)?;
                println!(
                    "[OK] Registered hotkey: {} (region select)",
                    hotkey_region_str
                );
                Some((hk, hk.id()))
            }
            Err(e) => {
                eprintln!(
                    "[WARN] Failed to parse region select hotkey '{}': {}",
                    hotkey_region_str, e
                );
                None
            }
        };

        Ok(Self {
            _manager: manager,
            hotkey_5s,
            hotkey_10s,
            hotkey_15s,
            hotkey_screenshot,
            hotkey_region,
        })
    }

    /// Get the buffer duration for a given hotkey ID (legacy method)
    pub fn duration_for_id(&self, id: u32) -> Option<BufferDuration> {
        if let Some((_, hk_id)) = self.hotkey_5s {
            if hk_id == id {
                return Some(BufferDuration::Seconds5);
            }
        }
        if let Some((_, hk_id)) = self.hotkey_10s {
            if hk_id == id {
                return Some(BufferDuration::Seconds10);
            }
        }
        if let Some((_, hk_id)) = self.hotkey_15s {
            if hk_id == id {
                return Some(BufferDuration::Seconds15);
            }
        }
        None
    }

    /// Get the action for a given hotkey ID
    pub fn action_for_id(&self, id: u32) -> Option<HotkeyAction> {
        if let Some((_, hk_id)) = self.hotkey_5s {
            if hk_id == id {
                return Some(HotkeyAction::SaveBuffer(BufferDuration::Seconds5));
            }
        }
        if let Some((_, hk_id)) = self.hotkey_10s {
            if hk_id == id {
                return Some(HotkeyAction::SaveBuffer(BufferDuration::Seconds10));
            }
        }
        if let Some((_, hk_id)) = self.hotkey_15s {
            if hk_id == id {
                return Some(HotkeyAction::SaveBuffer(BufferDuration::Seconds15));
            }
        }
        if let Some((_, hk_id)) = self.hotkey_screenshot {
            if hk_id == id {
                return Some(HotkeyAction::Screenshot);
            }
        }
        if let Some((_, hk_id)) = self.hotkey_region {
            if hk_id == id {
                return Some(HotkeyAction::RegionSelect);
            }
        }
        None
    }
}

/// Parse a hotkey string like "CTRL+ALT+1" into a HotKey
pub fn parse_hotkey(s: &str) -> Result<HotKey, String> {
    let parts: Vec<String> = s.split('+').map(|p| p.trim().to_uppercase()).collect();

    if parts.is_empty() {
        return Err("Empty hotkey string".to_string());
    }

    let mut modifiers = Modifiers::empty();
    let mut key_code: Option<Code> = None;

    for part in &parts {
        match part.as_str() {
            "CTRL" | "CONTROL" => modifiers |= Modifiers::CONTROL,
            "ALT" => modifiers |= Modifiers::ALT,
            "SHIFT" => modifiers |= Modifiers::SHIFT,
            "SUPER" | "WIN" | "CMD" => modifiers |= Modifiers::SUPER,
            _ => {
                // Must be the key
                key_code = Some(parse_key_code(part)?);
            }
        }
    }

    let code = key_code.ok_or("No key specified in hotkey")?;
    let mods = if modifiers.is_empty() {
        None
    } else {
        Some(modifiers)
    };

    Ok(HotKey::new(mods, code))
}

/// Parse a key code string into a Code
pub fn parse_key_code(s: &str) -> Result<Code, String> {
    // Handle single characters
    if s.len() == 1 {
        let c = s.chars().next().unwrap();
        return match c {
            'A' => Ok(Code::KeyA),
            'B' => Ok(Code::KeyB),
            'C' => Ok(Code::KeyC),
            'D' => Ok(Code::KeyD),
            'E' => Ok(Code::KeyE),
            'F' => Ok(Code::KeyF),
            'G' => Ok(Code::KeyG),
            'H' => Ok(Code::KeyH),
            'I' => Ok(Code::KeyI),
            'J' => Ok(Code::KeyJ),
            'K' => Ok(Code::KeyK),
            'L' => Ok(Code::KeyL),
            'M' => Ok(Code::KeyM),
            'N' => Ok(Code::KeyN),
            'O' => Ok(Code::KeyO),
            'P' => Ok(Code::KeyP),
            'Q' => Ok(Code::KeyQ),
            'R' => Ok(Code::KeyR),
            'S' => Ok(Code::KeyS),
            'T' => Ok(Code::KeyT),
            'U' => Ok(Code::KeyU),
            'V' => Ok(Code::KeyV),
            'W' => Ok(Code::KeyW),
            'X' => Ok(Code::KeyX),
            'Y' => Ok(Code::KeyY),
            'Z' => Ok(Code::KeyZ),
            '0' => Ok(Code::Digit0),
            '1' => Ok(Code::Digit1),
            '2' => Ok(Code::Digit2),
            '3' => Ok(Code::Digit3),
            '4' => Ok(Code::Digit4),
            '5' => Ok(Code::Digit5),
            '6' => Ok(Code::Digit6),
            '7' => Ok(Code::Digit7),
            '8' => Ok(Code::Digit8),
            '9' => Ok(Code::Digit9),
            _ => Err(format!("Unknown key: {}", c)),
        };
    }

    // Handle function keys and special keys
    match s {
        "F1" => Ok(Code::F1),
        "F2" => Ok(Code::F2),
        "F3" => Ok(Code::F3),
        "F4" => Ok(Code::F4),
        "F5" => Ok(Code::F5),
        "F6" => Ok(Code::F6),
        "F7" => Ok(Code::F7),
        "F8" => Ok(Code::F8),
        "F9" => Ok(Code::F9),
        "F10" => Ok(Code::F10),
        "F11" => Ok(Code::F11),
        "F12" => Ok(Code::F12),
        "SPACE" => Ok(Code::Space),
        "ENTER" | "RETURN" => Ok(Code::Enter),
        "TAB" => Ok(Code::Tab),
        "ESCAPE" | "ESC" => Ok(Code::Escape),
        "BACKSPACE" => Ok(Code::Backspace),
        "DELETE" | "DEL" => Ok(Code::Delete),
        "INSERT" | "INS" => Ok(Code::Insert),
        "HOME" => Ok(Code::Home),
        "END" => Ok(Code::End),
        "PAGEUP" | "PGUP" => Ok(Code::PageUp),
        "PAGEDOWN" | "PGDN" => Ok(Code::PageDown),
        "UP" => Ok(Code::ArrowUp),
        "DOWN" => Ok(Code::ArrowDown),
        "LEFT" => Ok(Code::ArrowLeft),
        "RIGHT" => Ok(Code::ArrowRight),
        _ => Err(format!("Unknown key: {}", s)),
    }
}
