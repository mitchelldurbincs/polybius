//! OCR (Optical Character Recognition) module
//!
//! Uses Windows.Media.Ocr API on Windows for text extraction.
//! Returns empty results on non-Windows platforms.

use crate::screenshot::Screenshot;
use serde::Serialize;

/// OCR processing error
#[derive(Debug)]
pub enum OcrError {
    InitError(String),
    ProcessError(String),
    LanguageNotAvailable(String),
}

impl std::fmt::Display for OcrError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            OcrError::InitError(msg) => write!(f, "OCR init error: {}", msg),
            OcrError::ProcessError(msg) => write!(f, "OCR process error: {}", msg),
            OcrError::LanguageNotAvailable(lang) => {
                write!(f, "OCR language '{}' not available on this system", lang)
            }
        }
    }
}

impl std::error::Error for OcrError {}

/// A recognized word with its bounding box
#[derive(Debug, Clone, Serialize)]
pub struct OcrWord {
    pub text: String,
    pub bbox: [f64; 4], // x, y, width, height
}

/// OCR result containing extracted text and word details
#[derive(Debug, Clone, Serialize)]
pub struct OcrResult {
    pub text: String,
    pub words: Vec<OcrWord>,
    pub language: String,
}

impl OcrResult {
    /// Create an empty result
    pub fn empty() -> Self {
        Self {
            text: String::new(),
            words: Vec::new(),
            language: String::new(),
        }
    }
}

/// OCR processor
pub struct OcrProcessor {
    #[cfg(windows)]
    engine: windows::Media::Ocr::OcrEngine,
    #[cfg(windows)]
    language_tag: String,
    #[cfg(not(windows))]
    _phantom: std::marker::PhantomData<()>,
}

impl OcrProcessor {
    /// Create OCR processor for a specific language
    ///
    /// Language tag should be a BCP-47 tag like "en-US", "zh-Hans", "ja", etc.
    /// Use "auto" for auto-detection.
    pub fn new(language_tag: &str) -> Result<Self, OcrError> {
        #[cfg(windows)]
        {
            Self::new_windows(language_tag)
        }

        #[cfg(not(windows))]
        {
            let _ = language_tag;
            Ok(Self {
                _phantom: std::marker::PhantomData,
            })
        }
    }

    /// Extract text from a screenshot
    pub fn process(&self, screenshot: &Screenshot) -> Result<OcrResult, OcrError> {
        #[cfg(windows)]
        {
            self.process_windows(screenshot)
        }

        #[cfg(not(windows))]
        {
            let _ = screenshot;
            // Return empty result on non-Windows
            Ok(OcrResult::empty())
        }
    }

    /// Check if OCR is available on this platform
    pub fn is_available() -> bool {
        #[cfg(windows)]
        {
            true
        }

        #[cfg(not(windows))]
        {
            false
        }
    }
}

// Windows implementation
#[cfg(windows)]
impl OcrProcessor {
    fn new_windows(language_tag: &str) -> Result<Self, OcrError> {
        use windows::Globalization::Language;
        use windows::Media::Ocr::OcrEngine;

        let engine = if language_tag == "auto" {
            // Try to get user's preferred language
            OcrEngine::TryCreateFromUserProfileLanguages()
                .map_err(|e| {
                    OcrError::InitError(format!("Failed to create auto-detect engine: {}", e))
                })?
                .ok_or_else(|| OcrError::InitError("No OCR languages available".to_string()))?
        } else {
            let language = Language::CreateLanguage(&language_tag.into()).map_err(|e| {
                OcrError::InitError(format!("Invalid language tag '{}': {}", language_tag, e))
            })?;

            if !OcrEngine::IsLanguageSupported(&language).map_err(|e| {
                OcrError::InitError(format!("Failed to check language support: {}", e))
            })? {
                return Err(OcrError::LanguageNotAvailable(language_tag.to_string()));
            }

            OcrEngine::TryCreateFromLanguage(&language)
                .map_err(|e| OcrError::InitError(format!("Failed to create OCR engine: {}", e)))?
                .ok_or_else(|| OcrError::LanguageNotAvailable(language_tag.to_string()))?
        };

        Ok(Self {
            engine,
            language_tag: language_tag.to_string(),
        })
    }

    fn process_windows(&self, screenshot: &Screenshot) -> Result<OcrResult, OcrError> {
        use windows::Graphics::Imaging::{BitmapPixelFormat, SoftwareBitmap};
        use windows::Storage::Streams::{Buffer, IBuffer};

        // Create a SoftwareBitmap from the screenshot data
        // Windows OCR expects BGRA8 format
        let mut bgra_data = screenshot.data.clone();
        for chunk in bgra_data.chunks_exact_mut(4) {
            chunk.swap(0, 2); // Swap R and B (RGBA -> BGRA)
        }

        let bitmap = SoftwareBitmap::Create(
            BitmapPixelFormat::Bgra8,
            screenshot.width as i32,
            screenshot.height as i32,
        )
        .map_err(|e| OcrError::ProcessError(format!("Failed to create bitmap: {}", e)))?;

        // Copy pixel data to the bitmap
        let buffer = Buffer::Create(bgra_data.len() as u32)
            .map_err(|e| OcrError::ProcessError(format!("Failed to create buffer: {}", e)))?;

        // Write data to buffer
        {
            use windows::Storage::Streams::IBufferByteAccess;
            let byte_access: IBufferByteAccess = buffer
                .cast()
                .map_err(|e| OcrError::ProcessError(format!("Failed to get byte access: {}", e)))?;

            unsafe {
                let ptr = byte_access.Buffer().map_err(|e| {
                    OcrError::ProcessError(format!("Failed to get buffer ptr: {}", e))
                })?;
                std::ptr::copy_nonoverlapping(bgra_data.as_ptr(), ptr, bgra_data.len());
            }

            buffer.SetLength(bgra_data.len() as u32).map_err(|e| {
                OcrError::ProcessError(format!("Failed to set buffer length: {}", e))
            })?;
        }

        bitmap
            .CopyFromBuffer(&buffer)
            .map_err(|e| OcrError::ProcessError(format!("Failed to copy to bitmap: {}", e)))?;

        // Run OCR
        let ocr_result = self
            .engine
            .RecognizeAsync(&bitmap)
            .map_err(|e| OcrError::ProcessError(format!("Failed to start OCR: {}", e)))?
            .get()
            .map_err(|e| OcrError::ProcessError(format!("OCR failed: {}", e)))?;

        // Extract text and words
        let text = ocr_result
            .Text()
            .map_err(|e| OcrError::ProcessError(format!("Failed to get text: {}", e)))?
            .to_string();

        let mut words = Vec::new();

        let lines = ocr_result
            .Lines()
            .map_err(|e| OcrError::ProcessError(format!("Failed to get lines: {}", e)))?;

        for line in lines {
            let line_words = line
                .Words()
                .map_err(|e| OcrError::ProcessError(format!("Failed to get words: {}", e)))?;

            for word in line_words {
                let word_text = word
                    .Text()
                    .map_err(|e| OcrError::ProcessError(format!("Failed to get word text: {}", e)))?
                    .to_string();

                let bbox = word
                    .BoundingRect()
                    .map_err(|e| OcrError::ProcessError(format!("Failed to get bbox: {}", e)))?;

                words.push(OcrWord {
                    text: word_text,
                    bbox: [
                        bbox.X as f64,
                        bbox.Y as f64,
                        bbox.Width as f64,
                        bbox.Height as f64,
                    ],
                });
            }
        }

        // Get detected language if available
        let language = self.language_tag.clone();

        Ok(OcrResult {
            text,
            words,
            language,
        })
    }
}
