//! Metadata module for JSON output
//!
//! Creates structured JSON metadata files that combine information
//! about audio, screenshots, and OCR results.

use crate::ocr::{OcrResult, OcrWord};
use chrono::{DateTime, Utc};
use serde::Serialize;
use std::fs;
use std::path::Path;

/// Version of the metadata schema (string for backwards compatibility)
const METADATA_VERSION: &str = "1.0";

/// Schema version as integer for programmatic version checking
const SCHEMA_VERSION: u32 = 1;

/// Complete card metadata combining audio, screenshot, and OCR
#[derive(Debug, Serialize)]
pub struct CardMetadata {
    /// Schema version as integer for programmatic validation
    pub schema_version: u32,
    /// Human-readable version string
    pub version: String,
    pub timestamp: String,
    pub audio: AudioMetadata,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub screenshot: Option<ScreenshotMetadata>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ocr: Option<OcrMetadata>,
}

/// Audio file metadata
#[derive(Debug, Serialize)]
pub struct AudioMetadata {
    pub file: String,
    pub duration_seconds: f32,
    pub sample_rate: u32,
    pub channels: u16,
}

/// Screenshot file metadata
#[derive(Debug, Serialize)]
pub struct ScreenshotMetadata {
    pub file: String,
    pub width: u32,
    pub height: u32,
}

/// OCR extraction metadata
#[derive(Debug, Serialize)]
pub struct OcrMetadata {
    pub language: String,
    pub text: String,
    pub words: Vec<OcrWordMetadata>,
}

/// Individual word from OCR
#[derive(Debug, Serialize)]
pub struct OcrWordMetadata {
    pub text: String,
    pub bbox: [f64; 4],
}

impl CardMetadata {
    /// Create new metadata with audio information
    pub fn new(
        audio_filename: &str,
        duration_seconds: f32,
        sample_rate: u32,
        channels: u16,
    ) -> Self {
        let timestamp: DateTime<Utc> = Utc::now();

        Self {
            schema_version: SCHEMA_VERSION,
            version: METADATA_VERSION.to_string(),
            timestamp: timestamp.to_rfc3339(),
            audio: AudioMetadata {
                file: audio_filename.to_string(),
                duration_seconds,
                sample_rate,
                channels,
            },
            screenshot: None,
            ocr: None,
        }
    }

    /// Add screenshot metadata
    pub fn with_screenshot(mut self, filename: &str, width: u32, height: u32) -> Self {
        self.screenshot = Some(ScreenshotMetadata {
            file: filename.to_string(),
            width,
            height,
        });
        self
    }

    /// Add OCR metadata from OcrResult
    pub fn with_ocr(mut self, ocr_result: &OcrResult) -> Self {
        if !ocr_result.text.is_empty() {
            self.ocr = Some(OcrMetadata {
                language: ocr_result.language.clone(),
                text: ocr_result.text.clone(),
                words: ocr_result
                    .words
                    .iter()
                    .map(|w| OcrWordMetadata {
                        text: w.text.clone(),
                        bbox: w.bbox,
                    })
                    .collect(),
            });
        }
        self
    }

    /// Save metadata to JSON file
    pub fn save(&self, path: &Path) -> Result<(), std::io::Error> {
        let json = serde_json::to_string_pretty(self)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))?;
        fs::write(path, json)?;
        Ok(())
    }
}

impl From<&OcrWord> for OcrWordMetadata {
    fn from(word: &OcrWord) -> Self {
        Self {
            text: word.text.clone(),
            bbox: word.bbox,
        }
    }
}
