//! Tests for hotkey parsing functionality

use global_hotkey::hotkey::Code;
use miner::hotkeys::{parse_hotkey, parse_key_code, HotkeyAction};
use miner::BufferDuration;

// ==================== parse_key_code tests ====================

#[test]
fn test_parse_key_code_letters() {
    // All letters A-Z should parse correctly
    let test_cases = [
        ("A", Code::KeyA),
        ("B", Code::KeyB),
        ("C", Code::KeyC),
        ("D", Code::KeyD),
        ("E", Code::KeyE),
        ("F", Code::KeyF),
        ("G", Code::KeyG),
        ("H", Code::KeyH),
        ("I", Code::KeyI),
        ("J", Code::KeyJ),
        ("K", Code::KeyK),
        ("L", Code::KeyL),
        ("M", Code::KeyM),
        ("N", Code::KeyN),
        ("O", Code::KeyO),
        ("P", Code::KeyP),
        ("Q", Code::KeyQ),
        ("R", Code::KeyR),
        ("S", Code::KeyS),
        ("T", Code::KeyT),
        ("U", Code::KeyU),
        ("V", Code::KeyV),
        ("W", Code::KeyW),
        ("X", Code::KeyX),
        ("Y", Code::KeyY),
        ("Z", Code::KeyZ),
    ];

    for (letter, expected) in test_cases {
        assert_eq!(
            parse_key_code(letter).unwrap(),
            expected,
            "Failed for letter {}",
            letter
        );
    }
}

#[test]
fn test_parse_key_code_digits() {
    // All digits 0-9 should parse correctly
    let test_cases = [
        ("0", Code::Digit0),
        ("1", Code::Digit1),
        ("2", Code::Digit2),
        ("3", Code::Digit3),
        ("4", Code::Digit4),
        ("5", Code::Digit5),
        ("6", Code::Digit6),
        ("7", Code::Digit7),
        ("8", Code::Digit8),
        ("9", Code::Digit9),
    ];

    for (digit, expected) in test_cases {
        assert_eq!(
            parse_key_code(digit).unwrap(),
            expected,
            "Failed for digit {}",
            digit
        );
    }
}

#[test]
fn test_parse_key_code_function_keys() {
    let test_cases = [
        ("F1", Code::F1),
        ("F2", Code::F2),
        ("F3", Code::F3),
        ("F4", Code::F4),
        ("F5", Code::F5),
        ("F6", Code::F6),
        ("F7", Code::F7),
        ("F8", Code::F8),
        ("F9", Code::F9),
        ("F10", Code::F10),
        ("F11", Code::F11),
        ("F12", Code::F12),
    ];

    for (key, expected) in test_cases {
        assert_eq!(parse_key_code(key).unwrap(), expected, "Failed for {}", key);
    }
}

#[test]
fn test_parse_key_code_special_keys() {
    let test_cases = [
        ("SPACE", Code::Space),
        ("ENTER", Code::Enter),
        ("RETURN", Code::Enter),
        ("TAB", Code::Tab),
        ("ESCAPE", Code::Escape),
        ("ESC", Code::Escape),
        ("BACKSPACE", Code::Backspace),
        ("DELETE", Code::Delete),
        ("DEL", Code::Delete),
        ("INSERT", Code::Insert),
        ("INS", Code::Insert),
        ("HOME", Code::Home),
        ("END", Code::End),
        ("PAGEUP", Code::PageUp),
        ("PGUP", Code::PageUp),
        ("PAGEDOWN", Code::PageDown),
        ("PGDN", Code::PageDown),
        ("UP", Code::ArrowUp),
        ("DOWN", Code::ArrowDown),
        ("LEFT", Code::ArrowLeft),
        ("RIGHT", Code::ArrowRight),
    ];

    for (key, expected) in test_cases {
        assert_eq!(parse_key_code(key).unwrap(), expected, "Failed for {}", key);
    }
}

#[test]
fn test_parse_key_code_unknown_key() {
    assert!(parse_key_code("UNKNOWN").is_err());
    assert!(parse_key_code("F13").is_err());
    assert!(parse_key_code("CAPSLOCK").is_err());
}

#[test]
fn test_parse_key_code_empty_string() {
    // Empty string should error (single-char branch won't match, multi-char match fails)
    assert!(parse_key_code("").is_err());
}

#[test]
fn test_parse_key_code_lowercase_rejected() {
    // parse_key_code expects uppercase input (caller uppercases)
    // Lowercase letters should fail since they're not in the match
    assert!(parse_key_code("a").is_err());
    assert!(parse_key_code("space").is_err());
}

// ==================== parse_hotkey tests ====================

#[test]
fn test_parse_hotkey_basic_combinations() {
    // These should all parse successfully
    let valid_hotkeys = [
        "CTRL+ALT+1",
        "CTRL+ALT+2",
        "CTRL+ALT+3",
        "CTRL+ALT+S",
        "CTRL+ALT+R",
        "CTRL+SHIFT+A",
        "ALT+F4",
        "CTRL+C",
    ];

    for hotkey in valid_hotkeys {
        assert!(parse_hotkey(hotkey).is_ok(), "Should parse: {}", hotkey);
    }
}

#[test]
fn test_parse_hotkey_case_insensitive() {
    // Hotkey parsing should be case-insensitive
    assert!(parse_hotkey("ctrl+alt+1").is_ok());
    assert!(parse_hotkey("Ctrl+Alt+S").is_ok());
    assert!(parse_hotkey("CTRL+alt+R").is_ok());
}

#[test]
fn test_parse_hotkey_with_spaces() {
    // Spaces around + should be trimmed
    assert!(parse_hotkey("CTRL + ALT + 1").is_ok());
    assert!(parse_hotkey("CTRL  +  ALT  +  S").is_ok());
}

#[test]
fn test_parse_hotkey_modifier_variants() {
    // Test different modifier names
    assert!(parse_hotkey("CONTROL+A").is_ok()); // CONTROL instead of CTRL
    assert!(parse_hotkey("WIN+E").is_ok()); // WIN modifier
    assert!(parse_hotkey("CMD+C").is_ok()); // CMD modifier (Mac style)
    assert!(parse_hotkey("SUPER+L").is_ok()); // SUPER modifier
}

#[test]
fn test_parse_hotkey_empty_string() {
    let result = parse_hotkey("");
    assert!(result.is_err());
    // Empty string becomes [""] after split, which fails key parsing
    assert!(result.unwrap_err().contains("Unknown key"));
}

#[test]
fn test_parse_hotkey_no_key() {
    // Only modifiers, no actual key
    let result = parse_hotkey("CTRL+ALT");
    assert!(result.is_err());
    assert_eq!(result.unwrap_err(), "No key specified in hotkey");
}

#[test]
fn test_parse_hotkey_key_only() {
    // Just a key with no modifiers - should work
    assert!(parse_hotkey("F1").is_ok());
    assert!(parse_hotkey("A").is_ok());
    assert!(parse_hotkey("SPACE").is_ok());
}

#[test]
fn test_parse_hotkey_invalid_key() {
    let result = parse_hotkey("CTRL+ALT+INVALID");
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("Unknown key"));
}

#[test]
fn test_parse_hotkey_special_keys() {
    // Test with special keys
    assert!(parse_hotkey("CTRL+SPACE").is_ok());
    assert!(parse_hotkey("ALT+ENTER").is_ok());
    assert!(parse_hotkey("CTRL+SHIFT+ESC").is_ok());
    assert!(parse_hotkey("CTRL+PGUP").is_ok());
    assert!(parse_hotkey("CTRL+HOME").is_ok());
}

#[test]
fn test_parse_hotkey_all_modifiers() {
    // All modifiers at once
    assert!(parse_hotkey("CTRL+ALT+SHIFT+SUPER+A").is_ok());
}

// ==================== HotkeyAction tests ====================

#[test]
fn test_hotkey_action_equality() {
    assert_eq!(
        HotkeyAction::SaveBuffer(BufferDuration::Seconds5),
        HotkeyAction::SaveBuffer(BufferDuration::Seconds5)
    );
    assert_ne!(
        HotkeyAction::SaveBuffer(BufferDuration::Seconds5),
        HotkeyAction::SaveBuffer(BufferDuration::Seconds10)
    );
    assert_ne!(HotkeyAction::Screenshot, HotkeyAction::RegionSelect);
}

#[test]
fn test_hotkey_action_debug() {
    // Verify Debug is implemented
    let action = HotkeyAction::Screenshot;
    let debug_str = format!("{:?}", action);
    assert!(debug_str.contains("Screenshot"));
}
