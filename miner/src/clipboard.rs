//! Clipboard module for copying images to the Windows clipboard

use crate::screenshot::Screenshot;

#[derive(Debug)]
pub enum ClipboardError {
    OpenFailed(String),
    SetDataFailed(String),
    MemoryError(String),
    NotSupported,
}

impl std::fmt::Display for ClipboardError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ClipboardError::OpenFailed(msg) => write!(f, "Failed to open clipboard: {}", msg),
            ClipboardError::SetDataFailed(msg) => write!(f, "Failed to set clipboard data: {}", msg),
            ClipboardError::MemoryError(msg) => write!(f, "Memory error: {}", msg),
            ClipboardError::NotSupported => write!(f, "Clipboard not supported on this platform"),
        }
    }
}

impl std::error::Error for ClipboardError {}

#[cfg(windows)]
mod windows_impl {
    use super::*;
    use std::ptr;
    use windows::Win32::Foundation::HANDLE;
    use windows::Win32::Graphics::Gdi::{BITMAPINFOHEADER, BI_RGB};
    use windows::Win32::System::DataExchange::{
        CloseClipboard, EmptyClipboard, OpenClipboard, SetClipboardData,
    };
    use windows::Win32::System::Memory::{GlobalAlloc, GlobalLock, GlobalUnlock, GMEM_MOVEABLE};

    // CF_DIB clipboard format constant
    const CF_DIB: u32 = 8;

    /// Copy a screenshot to the Windows clipboard as DIB (Device Independent Bitmap)
    pub fn copy_to_clipboard(screenshot: &Screenshot) -> Result<(), ClipboardError> {
        let width = screenshot.width;
        let height = screenshot.height;
        let rgba_data = screenshot.as_rgba_bytes();

        // DIB format: BITMAPINFOHEADER + pixel data (BGR, bottom-up)
        let header_size = std::mem::size_of::<BITMAPINFOHEADER>();
        let row_size = ((width * 3 + 3) / 4) * 4; // 24-bit BGR, padded to 4-byte boundary
        let pixel_size = (row_size * height) as usize;
        let total_size = header_size + pixel_size;

        unsafe {
            // Open clipboard
            if OpenClipboard(None).is_err() {
                return Err(ClipboardError::OpenFailed("OpenClipboard failed".to_string()));
            }

            // Empty clipboard
            if EmptyClipboard().is_err() {
                CloseClipboard().ok();
                return Err(ClipboardError::SetDataFailed("EmptyClipboard failed".to_string()));
            }

            // Allocate global memory
            let hmem = GlobalAlloc(GMEM_MOVEABLE, total_size);
            if hmem.is_err() {
                CloseClipboard().ok();
                return Err(ClipboardError::MemoryError("GlobalAlloc failed".to_string()));
            }
            let hmem = hmem.unwrap();

            // Lock memory and write data
            let ptr = GlobalLock(hmem);
            if ptr.is_null() {
                CloseClipboard().ok();
                return Err(ClipboardError::MemoryError("GlobalLock failed".to_string()));
            }

            // Write BITMAPINFOHEADER
            let header = BITMAPINFOHEADER {
                biSize: header_size as u32,
                biWidth: width as i32,
                biHeight: height as i32, // Positive = bottom-up DIB
                biPlanes: 1,
                biBitCount: 24, // 24-bit BGR
                biCompression: BI_RGB.0,
                biSizeImage: pixel_size as u32,
                biXPelsPerMeter: 0,
                biYPelsPerMeter: 0,
                biClrUsed: 0,
                biClrImportant: 0,
            };

            ptr::copy_nonoverlapping(
                &header as *const BITMAPINFOHEADER as *const u8,
                ptr as *mut u8,
                header_size,
            );

            // Write pixel data (convert RGBA to BGR, flip vertically)
            let pixel_ptr = (ptr as *mut u8).add(header_size);
            for y in 0..height {
                let src_row = (height - 1 - y) as usize; // Flip: bottom row first
                let dst_offset = (y * row_size) as usize;

                for x in 0..width {
                    let src_offset = (src_row * width as usize + x as usize) * 4;
                    let dst_pixel_offset = dst_offset + (x as usize) * 3;

                    // RGBA -> BGR
                    let r = rgba_data[src_offset];
                    let g = rgba_data[src_offset + 1];
                    let b = rgba_data[src_offset + 2];

                    *pixel_ptr.add(dst_pixel_offset) = b;
                    *pixel_ptr.add(dst_pixel_offset + 1) = g;
                    *pixel_ptr.add(dst_pixel_offset + 2) = r;
                }
            }

            GlobalUnlock(hmem).ok();

            // Set clipboard data
            let result = SetClipboardData(CF_DIB, HANDLE(hmem.0));
            CloseClipboard().ok();

            if result.is_err() {
                return Err(ClipboardError::SetDataFailed("SetClipboardData failed".to_string()));
            }

            Ok(())
        }
    }
}

#[cfg(windows)]
pub use windows_impl::copy_to_clipboard;

#[cfg(not(windows))]
pub fn copy_to_clipboard(_screenshot: &Screenshot) -> Result<(), ClipboardError> {
    Err(ClipboardError::NotSupported)
}
