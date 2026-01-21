//! Tests for metadata JSON generation

use miner::metadata::CardMetadata;
use miner::ocr::{OcrResult, OcrWord};
use std::fs;
use tempfile::tempdir;

// ==================== CardMetadata builder tests ====================

#[test]
fn test_card_metadata_new() {
    let meta = CardMetadata::new("test_audio.wav", 5.5, 48000, 2);

    assert_eq!(meta.schema_version, 1);
    assert_eq!(meta.version, "1.0");
    assert_eq!(meta.audio.file, "test_audio.wav");
    assert_eq!(meta.audio.duration_seconds, 5.5);
    assert_eq!(meta.audio.sample_rate, 48000);
    assert_eq!(meta.audio.channels, 2);
    assert!(meta.screenshot.is_none());
    assert!(meta.ocr.is_none());
}

#[test]
fn test_card_metadata_with_screenshot() {
    let meta =
        CardMetadata::new("audio.wav", 10.0, 44100, 2).with_screenshot("shot.png", 1920, 1080);

    assert!(meta.screenshot.is_some());
    let screenshot = meta.screenshot.unwrap();
    assert_eq!(screenshot.file, "shot.png");
    assert_eq!(screenshot.width, 1920);
    assert_eq!(screenshot.height, 1080);
}

#[test]
fn test_card_metadata_with_ocr() {
    let ocr_result = OcrResult {
        text: "Hello World".to_string(),
        words: vec![
            OcrWord {
                text: "Hello".to_string(),
                bbox: [10.0, 20.0, 50.0, 30.0],
            },
            OcrWord {
                text: "World".to_string(),
                bbox: [70.0, 20.0, 50.0, 30.0],
            },
        ],
        language: "en-US".to_string(),
    };

    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2).with_ocr(&ocr_result);

    assert!(meta.ocr.is_some());
    let ocr = meta.ocr.unwrap();
    assert_eq!(ocr.language, "en-US");
    assert_eq!(ocr.text, "Hello World");
    assert_eq!(ocr.words.len(), 2);
    assert_eq!(ocr.words[0].text, "Hello");
    assert_eq!(ocr.words[1].text, "World");
}

#[test]
fn test_card_metadata_with_empty_ocr() {
    let ocr_result = OcrResult {
        text: String::new(),
        words: vec![],
        language: "en-US".to_string(),
    };

    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2).with_ocr(&ocr_result);

    // Empty OCR text should result in no OCR metadata
    assert!(meta.ocr.is_none());
}

#[test]
fn test_card_metadata_builder_chaining() {
    let ocr_result = OcrResult {
        text: "Test".to_string(),
        words: vec![],
        language: "en-US".to_string(),
    };

    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2)
        .with_screenshot("shot.png", 800, 600)
        .with_ocr(&ocr_result);

    assert!(meta.screenshot.is_some());
    assert!(meta.ocr.is_some());
}

// ==================== JSON serialization tests ====================

#[test]
fn test_card_metadata_json_serialization() {
    let meta = CardMetadata::new("test.wav", 5.0, 48000, 2);

    let json = serde_json::to_string_pretty(&meta).unwrap();

    assert!(json.contains("\"schema_version\": 1"));
    assert!(json.contains("\"version\": \"1.0\""));
    assert!(json.contains("\"file\": \"test.wav\""));
    assert!(json.contains("\"duration_seconds\": 5.0"));
    assert!(json.contains("\"sample_rate\": 48000"));
    assert!(json.contains("\"channels\": 2"));
    // Null fields should be omitted (skip_serializing_if)
    assert!(!json.contains("\"screenshot\""));
    assert!(!json.contains("\"ocr\""));
}

#[test]
fn test_card_metadata_json_with_all_fields() {
    let ocr_result = OcrResult {
        text: "Test".to_string(),
        words: vec![OcrWord {
            text: "Test".to_string(),
            bbox: [0.0, 0.0, 100.0, 50.0],
        }],
        language: "en-US".to_string(),
    };

    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2)
        .with_screenshot("shot.png", 1920, 1080)
        .with_ocr(&ocr_result);

    let json = serde_json::to_string_pretty(&meta).unwrap();

    assert!(json.contains("\"screenshot\""));
    assert!(json.contains("\"ocr\""));
    assert!(json.contains("\"bbox\""));
}

#[test]
fn test_card_metadata_json_roundtrip() {
    let ocr_result = OcrResult {
        text: "Hello".to_string(),
        words: vec![OcrWord {
            text: "Hello".to_string(),
            bbox: [10.0, 20.0, 30.0, 40.0],
        }],
        language: "zh-CN".to_string(),
    };

    let original = CardMetadata::new("audio.wav", 10.5, 44100, 1)
        .with_screenshot("img.png", 1280, 720)
        .with_ocr(&ocr_result);

    let json = serde_json::to_string(&original).unwrap();
    let parsed: serde_json::Value = serde_json::from_str(&json).unwrap();

    // Verify key fields
    assert_eq!(parsed["schema_version"], 1);
    assert_eq!(parsed["audio"]["file"], "audio.wav");
    assert_eq!(parsed["audio"]["duration_seconds"], 10.5);
    assert_eq!(parsed["screenshot"]["width"], 1280);
    assert_eq!(parsed["ocr"]["language"], "zh-CN");
}

// ==================== File saving tests ====================

#[test]
fn test_card_metadata_save_to_file() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("metadata.json");

    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2);
    meta.save(&path).unwrap();

    assert!(path.exists());

    let content = fs::read_to_string(&path).unwrap();
    assert!(content.contains("audio.wav"));
    assert!(content.contains("schema_version"));
}

#[test]
fn test_card_metadata_save_creates_valid_json() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.json");

    let ocr_result = OcrResult {
        text: "Test text".to_string(),
        words: vec![],
        language: "en-US".to_string(),
    };

    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2)
        .with_screenshot("shot.png", 800, 600)
        .with_ocr(&ocr_result);

    meta.save(&path).unwrap();

    // Verify saved file is valid JSON
    let content = fs::read_to_string(&path).unwrap();
    let parsed: serde_json::Value = serde_json::from_str(&content).unwrap();

    assert!(parsed.is_object());
    assert!(parsed["audio"].is_object());
}

// ==================== Edge case tests ====================

#[test]
fn test_card_metadata_unicode_filename() {
    let meta = CardMetadata::new("音声_2024.wav", 5.0, 48000, 2);
    let json = serde_json::to_string(&meta).unwrap();

    assert!(json.contains("音声_2024.wav"));
}

#[test]
fn test_card_metadata_zero_duration() {
    let meta = CardMetadata::new("empty.wav", 0.0, 48000, 2);

    assert_eq!(meta.audio.duration_seconds, 0.0);
}

#[test]
fn test_card_metadata_mono_audio() {
    let meta = CardMetadata::new("mono.wav", 5.0, 22050, 1);

    assert_eq!(meta.audio.channels, 1);
    assert_eq!(meta.audio.sample_rate, 22050);
}

#[test]
fn test_ocr_word_bbox_values() {
    let ocr_result = OcrResult {
        text: "A".to_string(),
        words: vec![OcrWord {
            text: "A".to_string(),
            bbox: [-10.5, 0.0, 100.25, 50.75], // Can have negative x (partial offscreen)
        }],
        language: "en-US".to_string(),
    };

    let meta = CardMetadata::new("audio.wav", 1.0, 48000, 2).with_ocr(&ocr_result);
    let ocr = meta.ocr.unwrap();

    assert_eq!(ocr.words[0].bbox[0], -10.5);
    assert_eq!(ocr.words[0].bbox[2], 100.25);
}

#[test]
fn test_card_metadata_timestamp_format() {
    let meta = CardMetadata::new("audio.wav", 5.0, 48000, 2);

    // Timestamp should be RFC3339 format
    assert!(meta.timestamp.contains("T")); // ISO 8601 separator
    assert!(meta.timestamp.contains("-")); // Date separator
    assert!(meta.timestamp.contains(":")); // Time separator
}
