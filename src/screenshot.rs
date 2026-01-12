//! Screenshot capture module
//!
//! Captures screenshots using platform-specific APIs:
//! - Windows: win-screenshot crate
//! - Other platforms: screenshots crate

use std::path::Path;

/// Screenshot capture error
#[derive(Debug)]
pub enum ScreenshotError {
    CaptureError(String),
    EncodeError(String),
    IoError(std::io::Error),
}

impl std::fmt::Display for ScreenshotError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ScreenshotError::CaptureError(msg) => write!(f, "Capture error: {}", msg),
            ScreenshotError::EncodeError(msg) => write!(f, "Encode error: {}", msg),
            ScreenshotError::IoError(e) => write!(f, "IO error: {}", e),
        }
    }
}

impl std::error::Error for ScreenshotError {}

impl From<std::io::Error> for ScreenshotError {
    fn from(e: std::io::Error) -> Self {
        ScreenshotError::IoError(e)
    }
}

/// Screenshot data container
pub struct Screenshot {
    pub width: u32,
    pub height: u32,
    pub data: Vec<u8>, // RGBA pixels
}

impl Screenshot {
    /// Capture the primary monitor
    pub fn capture_screen() -> Result<Self, ScreenshotError> {
        #[cfg(windows)]
        {
            capture_screen_windows()
        }

        #[cfg(not(windows))]
        {
            capture_screen_other()
        }
    }

    /// Capture a specific window by title pattern
    #[allow(dead_code)]
    pub fn capture_window(title_pattern: &str) -> Result<Self, ScreenshotError> {
        #[cfg(windows)]
        {
            capture_window_windows(title_pattern)
        }

        #[cfg(not(windows))]
        {
            // Fall back to full screen capture on non-Windows
            let _ = title_pattern;
            capture_screen_other()
        }
    }

    /// Capture the currently active/foreground window
    pub fn capture_foreground() -> Result<Self, ScreenshotError> {
        #[cfg(windows)]
        {
            capture_foreground_windows()
        }

        #[cfg(not(windows))]
        {
            // Fall back to full screen capture on non-Windows
            capture_screen_other()
        }
    }

    /// Save screenshot as PNG
    pub fn save_png(&self, path: &Path) -> Result<(), ScreenshotError> {
        use image::{ImageBuffer, Rgba};

        let img: ImageBuffer<Rgba<u8>, Vec<u8>> =
            ImageBuffer::from_raw(self.width, self.height, self.data.clone()).ok_or_else(|| {
                ScreenshotError::EncodeError("Failed to create image buffer".to_string())
            })?;

        img.save(path)
            .map_err(|e| ScreenshotError::EncodeError(e.to_string()))?;

        Ok(())
    }

    /// Get image data as bytes for OCR processing
    pub fn as_rgba_bytes(&self) -> &[u8] {
        &self.data
    }
}

// Windows implementation using win-screenshot
#[cfg(windows)]
fn capture_screen_windows() -> Result<Screenshot, ScreenshotError> {
    use win_screenshot::prelude::*;

    let buf = capture_display().map_err(|e| ScreenshotError::CaptureError(format!("{:?}", e)))?;

    // win-screenshot returns BGRA, convert to RGBA
    let mut rgba_data = buf.pixels.clone();
    for chunk in rgba_data.chunks_exact_mut(4) {
        chunk.swap(0, 2); // Swap B and R
    }

    Ok(Screenshot {
        width: buf.width,
        height: buf.height,
        data: rgba_data,
    })
}

#[cfg(windows)]
fn capture_window_windows(title_pattern: &str) -> Result<Screenshot, ScreenshotError> {
    use win_screenshot::prelude::*;

    // Find window matching the pattern
    let windows = window_list().map_err(|e| ScreenshotError::CaptureError(format!("{:?}", e)))?;

    let target_hwnd = windows
        .into_iter()
        .find(|w| {
            w.window_name
                .to_lowercase()
                .contains(&title_pattern.to_lowercase())
        })
        .ok_or_else(|| {
            ScreenshotError::CaptureError(format!("Window '{}' not found", title_pattern))
        })?;

    let buf = capture_window(target_hwnd.hwnd)
        .map_err(|e| ScreenshotError::CaptureError(format!("{:?}", e)))?;

    // win-screenshot returns BGRA, convert to RGBA
    let mut rgba_data = buf.pixels.clone();
    for chunk in rgba_data.chunks_exact_mut(4) {
        chunk.swap(0, 2); // Swap B and R
    }

    Ok(Screenshot {
        width: buf.width,
        height: buf.height,
        data: rgba_data,
    })
}

/// Capture the foreground (active) window on Windows
#[cfg(windows)]
fn capture_foreground_windows() -> Result<Screenshot, ScreenshotError> {
    use win_screenshot::prelude::*;
    use windows::Win32::UI::WindowsAndMessaging::GetForegroundWindow;

    // Get the foreground window handle
    let hwnd = unsafe { GetForegroundWindow() };
    if hwnd.0.is_null() {
        return Err(ScreenshotError::CaptureError(
            "No foreground window found".to_string(),
        ));
    }

    let buf = capture_window(hwnd.0 as isize)
        .map_err(|e| ScreenshotError::CaptureError(format!("{:?}", e)))?;

    // win-screenshot returns BGRA, convert to RGBA
    let mut rgba_data = buf.pixels.clone();
    for chunk in rgba_data.chunks_exact_mut(4) {
        chunk.swap(0, 2); // Swap B and R
    }

    Ok(Screenshot {
        width: buf.width,
        height: buf.height,
        data: rgba_data,
    })
}

// Non-Windows: screenshot capture not available
// The screenshots crate requires system libraries that may not be present
#[cfg(not(windows))]
fn capture_screen_other() -> Result<Screenshot, ScreenshotError> {
    Err(ScreenshotError::CaptureError(
        "Screenshot capture is only supported on Windows".to_string(),
    ))
}
