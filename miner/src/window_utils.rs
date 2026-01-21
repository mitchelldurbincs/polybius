//! Window information utilities for Windows platform

/// Information about a window
#[derive(Debug, Clone)]
pub struct WindowInfo {
    pub hwnd: isize,
    pub title: String,
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
}

#[cfg(windows)]
mod windows_impl {
    use super::WindowInfo;
    use windows::Win32::Foundation::{HWND, RECT};
    use windows::Win32::UI::WindowsAndMessaging::{
        GetForegroundWindow, GetWindowRect, GetWindowTextLengthW, GetWindowTextW,
    };

    /// Get information about the current foreground window
    pub fn get_foreground_window_info() -> Result<WindowInfo, String> {
        unsafe {
            let hwnd = GetForegroundWindow();
            if hwnd.0.is_null() {
                return Err("No foreground window found".to_string());
            }

            get_window_info(hwnd)
        }
    }

    /// Get information about a specific window by handle
    pub fn get_window_info(hwnd: HWND) -> Result<WindowInfo, String> {
        unsafe {
            // Get window title
            let title_len = GetWindowTextLengthW(hwnd);
            let title = if title_len > 0 {
                let mut buffer: Vec<u16> = vec![0; (title_len + 1) as usize];
                let copied = GetWindowTextW(hwnd, &mut buffer);
                if copied > 0 {
                    String::from_utf16_lossy(&buffer[..copied as usize])
                } else {
                    String::new()
                }
            } else {
                String::new()
            };

            // Get window rect
            let mut rect = RECT::default();
            if GetWindowRect(hwnd, &mut rect).is_err() {
                return Err("Failed to get window rect".to_string());
            }

            let width = (rect.right - rect.left) as u32;
            let height = (rect.bottom - rect.top) as u32;

            Ok(WindowInfo {
                hwnd: hwnd.0 as isize,
                title,
                x: rect.left,
                y: rect.top,
                width,
                height,
            })
        }
    }

    /// Find a window by title pattern (case-insensitive substring match)
    pub fn find_window_by_pattern(pattern: &str) -> Result<WindowInfo, String> {
        use win_screenshot::prelude::*;

        let pattern_lower = pattern.to_lowercase();
        let windows = window_list().map_err(|e| format!("Failed to get window list: {:?}", e))?;

        for window in windows {
            if window.window_name.to_lowercase().contains(&pattern_lower) {
                let hwnd = HWND(window.hwnd as *mut _);
                return get_window_info(hwnd);
            }
        }

        Err(format!("No window found matching pattern: {}", pattern))
    }
}

#[cfg(windows)]
pub use windows_impl::*;

#[cfg(not(windows))]
pub fn get_foreground_window_info() -> Result<WindowInfo, String> {
    Err("Window utilities only supported on Windows".to_string())
}

#[cfg(not(windows))]
pub fn find_window_by_pattern(_pattern: &str) -> Result<WindowInfo, String> {
    Err("Window utilities only supported on Windows".to_string())
}
