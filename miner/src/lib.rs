//! The Miner - Audio and Vision capture library
//!
//! This library provides the core functionality for The Miner application:
//! - Audio capture with ring buffers
//! - Screenshot capture and OCR
//! - Configuration management
//! - Hotkey parsing

pub mod audio;
pub mod clipboard;
pub mod config;
pub mod hotkeys;
pub mod metadata;
pub mod notifications;
pub mod ocr;
pub mod region_overlay;
pub mod screenshot;
pub mod tray;
pub mod vision;
pub mod wav;
pub mod window_utils;

// Re-export commonly used types
pub use audio::{AudioCapture, AudioFormat, BufferDuration};
pub use config::{CaptureRegion, Config};
pub use hotkeys::{parse_hotkey, parse_key_code, HotkeyAction, HotkeyManager};
pub use metadata::{AudioMetadata, CardMetadata, OcrMetadata, OcrWordMetadata, ScreenshotMetadata};
pub use ocr::{OcrResult, OcrWord};
pub use screenshot::Screenshot;
pub use vision::VisionCapture;
