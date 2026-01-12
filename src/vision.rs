//! Vision module - unified interface for screenshot capture and OCR
//!
//! Combines screenshot capture and OCR processing into a single
//! easy-to-use interface with proper error handling.

use crate::config::VisionConfig;
use crate::metadata::CardMetadata;
use crate::ocr::{OcrError, OcrProcessor, OcrResult};
use crate::screenshot::{Screenshot, ScreenshotError};
use std::path::Path;

/// Vision capture error
#[derive(Debug)]
pub enum VisionError {
    Screenshot(ScreenshotError),
    Ocr(OcrError),
    Io(std::io::Error),
    Disabled,
}

impl std::fmt::Display for VisionError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            VisionError::Screenshot(e) => write!(f, "Screenshot error: {}", e),
            VisionError::Ocr(e) => write!(f, "OCR error: {}", e),
            VisionError::Io(e) => write!(f, "IO error: {}", e),
            VisionError::Disabled => write!(f, "Vision capture is disabled"),
        }
    }
}

impl std::error::Error for VisionError {}

impl From<ScreenshotError> for VisionError {
    fn from(e: ScreenshotError) -> Self {
        VisionError::Screenshot(e)
    }
}

impl From<OcrError> for VisionError {
    fn from(e: OcrError) -> Self {
        VisionError::Ocr(e)
    }
}

impl From<std::io::Error> for VisionError {
    fn from(e: std::io::Error) -> Self {
        VisionError::Io(e)
    }
}

/// Capture mode for screenshots
#[derive(Debug, Clone)]
pub enum CaptureMode {
    /// Capture the entire primary screen
    Screen,
    /// Capture a specific window by title pattern
    Window(String),
}

impl CaptureMode {
    /// Create from config string
    pub fn from_config(mode: &str, window_pattern: Option<&str>) -> Self {
        match mode.to_lowercase().as_str() {
            "window" => {
                if let Some(pattern) = window_pattern {
                    CaptureMode::Window(pattern.to_string())
                } else {
                    // Fall back to screen capture if no pattern specified
                    CaptureMode::Screen
                }
            }
            _ => CaptureMode::Screen,
        }
    }
}

/// Result of a vision capture operation
pub struct CaptureResult {
    pub screenshot: Option<Screenshot>,
    pub ocr_result: Option<OcrResult>,
}

/// Vision capture manager
pub struct VisionCapture {
    enabled: bool,
    screenshot_enabled: bool,
    ocr_enabled: bool,
    capture_mode: CaptureMode,
    ocr_processor: Option<OcrProcessor>,
    metadata_enabled: bool,
}

impl VisionCapture {
    /// Create a new vision capture instance from config
    pub fn new(config: &VisionConfig) -> Self {
        // If vision is disabled at the master level, return disabled instance
        if !config.enabled {
            return Self {
                enabled: false,
                screenshot_enabled: false,
                ocr_enabled: false,
                capture_mode: CaptureMode::Screen,
                ocr_processor: None,
                metadata_enabled: false,
            };
        }

        // Parse capture mode
        let capture_mode =
            CaptureMode::from_config(&config.capture_mode, config.window_pattern.as_deref());

        // Initialize OCR processor if enabled
        let ocr_processor = if config.ocr_enabled && OcrProcessor::is_available() {
            match OcrProcessor::new(&config.ocr_language) {
                Ok(processor) => {
                    println!(
                        "[OK] OCR initialized with language: {}",
                        config.ocr_language
                    );
                    Some(processor)
                }
                Err(e) => {
                    eprintln!("[WARN] Failed to initialize OCR: {}", e);
                    eprintln!("[WARN] OCR will be disabled for this session");
                    None
                }
            }
        } else {
            if config.ocr_enabled && !OcrProcessor::is_available() {
                eprintln!("[WARN] OCR is not available on this platform");
            }
            None
        };

        Self {
            enabled: true,
            screenshot_enabled: config.screenshot_enabled,
            ocr_enabled: config.ocr_enabled && ocr_processor.is_some(),
            capture_mode,
            ocr_processor,
            metadata_enabled: config.metadata_enabled,
        }
    }

    /// Check if vision capture is enabled
    pub fn is_enabled(&self) -> bool {
        self.enabled && self.screenshot_enabled
    }

    /// Check if OCR is enabled and available
    pub fn is_ocr_enabled(&self) -> bool {
        self.ocr_enabled && self.ocr_processor.is_some()
    }

    /// Check if metadata output is enabled
    pub fn is_metadata_enabled(&self) -> bool {
        self.metadata_enabled
    }

    /// Capture screenshot and optionally run OCR
    pub fn capture(&self) -> Result<CaptureResult, VisionError> {
        if !self.enabled {
            return Err(VisionError::Disabled);
        }

        // Capture screenshot
        let screenshot = if self.screenshot_enabled {
            let ss = match &self.capture_mode {
                CaptureMode::Screen => Screenshot::capture_screen()?,
                CaptureMode::Window(pattern) => Screenshot::capture_window(pattern)?,
            };
            Some(ss)
        } else {
            None
        };

        // Run OCR if enabled and we have a screenshot
        let ocr_result = if self.ocr_enabled {
            if let (Some(ref ss), Some(ref processor)) = (&screenshot, &self.ocr_processor) {
                match processor.process(ss) {
                    Ok(result) => Some(result),
                    Err(e) => {
                        eprintln!("[WARN] OCR processing failed: {}", e);
                        None
                    }
                }
            } else {
                None
            }
        } else {
            None
        };

        Ok(CaptureResult {
            screenshot,
            ocr_result,
        })
    }

    /// Save capture results to files
    ///
    /// Takes a base path (without extension) and saves:
    /// - {base_path}.png - Screenshot image
    /// - Returns screenshot dimensions for metadata
    pub fn save_screenshot(
        &self,
        result: &CaptureResult,
        base_path: &Path,
    ) -> Result<Option<(String, u32, u32)>, VisionError> {
        if let Some(ref screenshot) = result.screenshot {
            let png_path = base_path.with_extension("png");
            screenshot.save_png(&png_path)?;

            let filename = png_path
                .file_name()
                .and_then(|n| n.to_str())
                .unwrap_or("screenshot.png")
                .to_string();

            println!("[OK] Screenshot saved: {}", png_path.display());

            Ok(Some((filename, screenshot.width, screenshot.height)))
        } else {
            Ok(None)
        }
    }

    /// Create metadata for a capture, including audio information
    pub fn create_metadata(
        &self,
        audio_filename: &str,
        duration_seconds: f32,
        sample_rate: u32,
        channels: u16,
        capture_result: &CaptureResult,
        screenshot_info: Option<(String, u32, u32)>,
    ) -> CardMetadata {
        let mut metadata =
            CardMetadata::new(audio_filename, duration_seconds, sample_rate, channels);

        // Add screenshot info if available
        if let Some((filename, width, height)) = screenshot_info {
            metadata = metadata.with_screenshot(&filename, width, height);
        }

        // Add OCR results if available
        if let Some(ref ocr_result) = capture_result.ocr_result {
            metadata = metadata.with_ocr(ocr_result);
        }

        metadata
    }

    /// Save metadata to JSON file
    pub fn save_metadata(
        &self,
        metadata: &CardMetadata,
        base_path: &Path,
    ) -> Result<(), VisionError> {
        if !self.metadata_enabled {
            return Ok(());
        }

        let json_path = base_path.with_extension("json");
        metadata.save(&json_path)?;

        println!("[OK] Metadata saved: {}", json_path.display());

        Ok(())
    }
}
