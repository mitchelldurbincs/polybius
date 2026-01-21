//! Region selection overlay UI
//!
//! Displays a window showing the captured screenshot and allows
//! the user to drag-select a region. Uses raw Win32 APIs to avoid
//! winit's event loop limitation.

use crate::config::CaptureRegion;
use crate::screenshot::Screenshot;
use crate::window_utils::WindowInfo;

/// Result of region selection
#[derive(Debug)]
pub enum RegionSelectionResult {
    /// User selected a region
    Selected(CaptureRegion),
    /// User cancelled selection
    Cancelled,
}

#[cfg(windows)]
mod windows_impl {
    use super::*;
    use std::cell::RefCell;
    use windows::core::w;
    use windows::Win32::Foundation::{HWND, LPARAM, LRESULT, RECT, WPARAM};
    use windows::Win32::Graphics::Gdi::{
        BeginPaint, BitBlt, CreateCompatibleBitmap, CreateCompatibleDC, CreateSolidBrush, DeleteDC,
        DeleteObject, EndPaint, FillRect, GetDC, InvalidateRect, ReleaseDC, SelectObject,
        SetDIBits, BITMAPINFO, BITMAPINFOHEADER, BI_RGB, DIB_RGB_COLORS, HDC, HBITMAP,
        PAINTSTRUCT, SRCCOPY,
    };
    use windows::Win32::System::LibraryLoader::GetModuleHandleW;
    use windows::Win32::UI::WindowsAndMessaging::{
        CreateWindowExW, DefWindowProcW, DestroyWindow, DispatchMessageW, GetClientRect,
        GetMessageW, PostQuitMessage, RegisterClassW, ShowWindow, TranslateMessage, CS_HREDRAW,
        CS_VREDRAW, MSG, SW_SHOW, WINDOW_EX_STYLE, WM_DESTROY, WM_KEYDOWN, WM_LBUTTONDOWN,
        WM_LBUTTONUP, WM_MOUSEMOVE, WM_PAINT, WNDCLASSW, WS_POPUP, WS_VISIBLE,
    };

    // Virtual key codes
    const VK_ESCAPE: usize = 0x1B;
    const VK_RETURN: usize = 0x0D;

    // Visual constants
    const DARKEN_PERCENT: u32 = 40;
    const SELECTION_BORDER_COLOR: u32 = 0x00FF8040; // Orange
    const SELECTION_BORDER_WIDTH: i32 = 2;
    const MIN_SELECTION_SIZE: u32 = 10;

    /// Normalized selection rectangle (always left < right, top < bottom)
    #[derive(Clone, Copy)]
    struct SelectionRect {
        left: i32,
        top: i32,
        right: i32,
        bottom: i32,
    }

    impl SelectionRect {
        fn from_points(x1: i32, y1: i32, x2: i32, y2: i32) -> Self {
            Self {
                left: x1.min(x2),
                top: y1.min(y2),
                right: x1.max(x2),
                bottom: y1.max(y2),
            }
        }

        fn width(&self) -> u32 {
            (self.right - self.left) as u32
        }

        fn height(&self) -> u32 {
            (self.bottom - self.top) as u32
        }

        fn is_valid(&self) -> bool {
            self.width() > MIN_SELECTION_SIZE && self.height() > MIN_SELECTION_SIZE
        }

        /// Scale selection from window coordinates to image coordinates
        fn to_capture_region(&self, window_size: (i32, i32), image_size: (u32, u32)) -> CaptureRegion {
            let scale_x = image_size.0 as f64 / window_size.0 as f64;
            let scale_y = image_size.1 as f64 / window_size.1 as f64;

            CaptureRegion {
                x: (self.left as f64 * scale_x) as i32,
                y: (self.top as f64 * scale_y) as i32,
                width: (self.width() as f64 * scale_x) as u32,
                height: (self.height() as f64 * scale_y) as u32,
            }
        }
    }

    struct OverlayState {
        screenshot: Screenshot,
        #[allow(dead_code)] // Stored for potential future use (e.g., displaying window title)
        window_info: WindowInfo,
        is_selecting: bool,
        start_x: i32,
        start_y: i32,
        current_x: i32,
        current_y: i32,
        result: Option<RegionSelectionResult>,
        hdc_mem: HDC,
        hdc_dark: HDC,
        #[allow(dead_code)] // Handle must be kept alive while hdc_mem is in use
        hbm_screenshot: HBITMAP,
        #[allow(dead_code)] // Handle must be kept alive while hdc_dark is in use
        hbm_darkened: HBITMAP,
    }

    impl OverlayState {
        fn selection_rect(&self) -> SelectionRect {
            SelectionRect::from_points(self.start_x, self.start_y, self.current_x, self.current_y)
        }

        fn has_selection(&self) -> bool {
            self.is_selecting || self.start_x != self.current_x
        }
    }

    thread_local! {
        static OVERLAY_STATE: RefCell<Option<OverlayState>> = const { RefCell::new(None) };
    }

    // ============================================================
    // Helper functions
    // ============================================================

    /// Extract mouse coordinates from LPARAM (handles signed coords for multi-monitor)
    fn mouse_coords_from_lparam(lparam: LPARAM) -> (i32, i32) {
        let x = (lparam.0 & 0xFFFF) as i16 as i32;
        let y = ((lparam.0 >> 16) & 0xFFFF) as i16 as i32;
        (x, y)
    }

    /// Convert RGBA screenshot data to BGR format for Windows bitmaps (bottom-up order).
    /// Returns (original_bgr, darkened_bgr) tuple.
    fn convert_rgba_to_bgr(screenshot: &Screenshot) -> (Vec<u8>, Vec<u8>) {
        let pixel_count = (screenshot.width * screenshot.height * 4) as usize;
        let mut bgr_original = Vec::with_capacity(pixel_count);
        let mut bgr_darkened = Vec::with_capacity(pixel_count);

        // Windows bitmaps are bottom-up, so iterate rows in reverse
        for y in (0..screenshot.height).rev() {
            for x in 0..screenshot.width {
                let idx = ((y * screenshot.width + x) * 4) as usize;
                let (r, g, b) = (
                    screenshot.data[idx],
                    screenshot.data[idx + 1],
                    screenshot.data[idx + 2],
                );

                // Original: BGR + padding
                bgr_original.extend_from_slice(&[b, g, r, 0]);

                // Darkened: apply brightness reduction
                bgr_darkened.extend_from_slice(&[
                    (b as u32 * DARKEN_PERCENT / 100) as u8,
                    (g as u32 * DARKEN_PERCENT / 100) as u8,
                    (r as u32 * DARKEN_PERCENT / 100) as u8,
                    0,
                ]);
            }
        }

        (bgr_original, bgr_darkened)
    }

    /// Create BITMAPINFO header for the given dimensions
    fn create_bitmap_info(width: u32, height: u32) -> BITMAPINFO {
        BITMAPINFO {
            bmiHeader: BITMAPINFOHEADER {
                biSize: std::mem::size_of::<BITMAPINFOHEADER>() as u32,
                biWidth: width as i32,
                biHeight: height as i32,
                biPlanes: 1,
                biBitCount: 32,
                biCompression: BI_RGB.0,
                ..Default::default()
            },
            ..Default::default()
        }
    }

    /// Draw a rectangular border around a selection
    unsafe fn draw_selection_border(hdc: HDC, sel: SelectionRect) {
        let brush = CreateSolidBrush(windows::Win32::Foundation::COLORREF(SELECTION_BORDER_COLOR));
        let bw = SELECTION_BORDER_WIDTH;

        // Top, bottom, left, right borders
        let borders = [
            RECT { left: sel.left - bw, top: sel.top - bw, right: sel.right + bw, bottom: sel.top },
            RECT { left: sel.left - bw, top: sel.bottom, right: sel.right + bw, bottom: sel.bottom + bw },
            RECT { left: sel.left - bw, top: sel.top, right: sel.left, bottom: sel.bottom },
            RECT { left: sel.right, top: sel.top, right: sel.right + bw, bottom: sel.bottom },
        ];

        for border in &borders {
            FillRect(hdc, border, brush);
        }

        let _ = DeleteObject(brush);
    }

    // ============================================================
    // Main entry point
    // ============================================================

    pub fn select_region(
        screenshot: Screenshot,
        window_info: WindowInfo,
    ) -> Result<RegionSelectionResult, Box<dyn std::error::Error>> {
        unsafe {
            let hinstance = GetModuleHandleW(None)?;
            let class_name = w!("MinerRegionOverlay");

            // Register window class
            let wc = WNDCLASSW {
                style: CS_HREDRAW | CS_VREDRAW,
                lpfnWndProc: Some(window_proc),
                hInstance: hinstance.into(),
                lpszClassName: class_name,
                hCursor: windows::Win32::UI::WindowsAndMessaging::LoadCursorW(
                    None,
                    windows::Win32::UI::WindowsAndMessaging::IDC_CROSS,
                )
                .unwrap_or_default(),
                ..Default::default()
            };
            RegisterClassW(&wc);

            // Create memory DCs and bitmaps for original + darkened screenshots
            let hdc_screen = GetDC(None);
            let hdc_mem = CreateCompatibleDC(hdc_screen);
            let hdc_dark = CreateCompatibleDC(hdc_screen);
            let hbm_screenshot = CreateCompatibleBitmap(hdc_screen, screenshot.width as i32, screenshot.height as i32);
            let hbm_darkened = CreateCompatibleBitmap(hdc_screen, screenshot.width as i32, screenshot.height as i32);
            SelectObject(hdc_mem, hbm_screenshot);
            SelectObject(hdc_dark, hbm_darkened);

            // Convert and upload bitmap data
            let (bgr_original, bgr_darkened) = convert_rgba_to_bgr(&screenshot);
            let bmi = create_bitmap_info(screenshot.width, screenshot.height);

            SetDIBits(hdc_mem, hbm_screenshot, 0, screenshot.height, bgr_original.as_ptr() as *const _, &bmi, DIB_RGB_COLORS);
            SetDIBits(hdc_dark, hbm_darkened, 0, screenshot.height, bgr_darkened.as_ptr() as *const _, &bmi, DIB_RGB_COLORS);
            ReleaseDC(None, hdc_screen);

            // Store state for window_proc callbacks
            OVERLAY_STATE.with(|state| {
                *state.borrow_mut() = Some(OverlayState {
                    screenshot,
                    window_info: window_info.clone(),
                    is_selecting: false,
                    start_x: 0,
                    start_y: 0,
                    current_x: 0,
                    current_y: 0,
                    result: None,
                    hdc_mem,
                    hdc_dark,
                    hbm_screenshot,
                    hbm_darkened,
                });
            });

            // Create and show overlay window
            let hwnd = CreateWindowExW(
                WINDOW_EX_STYLE(0),
                class_name,
                w!("Select Region - Drag to select, ESC to cancel"),
                WS_POPUP | WS_VISIBLE,
                window_info.x,
                window_info.y,
                window_info.width as i32,
                window_info.height as i32,
                None,
                None,
                hinstance,
                None,
            )?;
            let _ = ShowWindow(hwnd, SW_SHOW);

            // Run message loop until window closes
            let mut msg = MSG::default();
            while GetMessageW(&mut msg, None, 0, 0).as_bool() {
                let _ = TranslateMessage(&msg);
                DispatchMessageW(&msg);
            }

            // Cleanup GDI resources
            let _ = DeleteDC(hdc_mem);
            let _ = DeleteDC(hdc_dark);
            let _ = DeleteObject(hbm_screenshot);
            let _ = DeleteObject(hbm_darkened);

            // Extract result
            let result = OVERLAY_STATE.with(|state| {
                state.borrow_mut().take()
                    .and_then(|s| s.result)
                    .unwrap_or(RegionSelectionResult::Cancelled)
            });

            Ok(result)
        }
    }

    // ============================================================
    // Window procedure and message handlers
    // ============================================================

    unsafe extern "system" fn window_proc(
        hwnd: HWND,
        msg: u32,
        wparam: WPARAM,
        lparam: LPARAM,
    ) -> LRESULT {
        match msg {
            WM_PAINT => handle_paint(hwnd),
            WM_LBUTTONDOWN => handle_mouse_down(hwnd, lparam),
            WM_MOUSEMOVE => handle_mouse_move(hwnd, lparam),
            WM_LBUTTONUP => handle_mouse_up(hwnd),
            WM_KEYDOWN => handle_key_down(hwnd, wparam),
            WM_DESTROY => {
                PostQuitMessage(0);
                LRESULT(0)
            }
            _ => DefWindowProcW(hwnd, msg, wparam, lparam),
        }
    }

    unsafe fn handle_paint(hwnd: HWND) -> LRESULT {
        let mut ps = PAINTSTRUCT::default();
        let hdc = BeginPaint(hwnd, &mut ps);

        OVERLAY_STATE.with(|state| {
            let state_ref = state.borrow();
            let Some(ref state) = *state_ref else { return };

            let mut rect = RECT::default();
            let _ = GetClientRect(hwnd, &mut rect);
            let win_size = (rect.right - rect.left, rect.bottom - rect.top);
            let img_size = (state.screenshot.width as i32, state.screenshot.height as i32);

            // Create temp DC for compositing
            let hdc_temp = CreateCompatibleDC(hdc);
            let hbm_temp = CreateCompatibleBitmap(hdc, win_size.0, win_size.1);
            SelectObject(hdc_temp, hbm_temp);

            // Draw darkened screenshot as base
            let _ = windows::Win32::Graphics::Gdi::StretchBlt(
                hdc_temp, 0, 0, win_size.0, win_size.1,
                state.hdc_dark, 0, 0, img_size.0, img_size.1,
                SRCCOPY,
            );

            // If selecting, overlay the bright (original) region
            if state.has_selection() {
                let sel = state.selection_rect();

                // Calculate source region in original image coordinates
                let scale_x = img_size.0 as f64 / win_size.0 as f64;
                let scale_y = img_size.1 as f64 / win_size.1 as f64;
                let src_x = (sel.left as f64 * scale_x) as i32;
                let src_y = (sel.top as f64 * scale_y) as i32;
                let src_w = (sel.width() as f64 * scale_x) as i32;
                let src_h = (sel.height() as f64 * scale_y) as i32;

                // Draw original (bright) region over darkened base
                let _ = windows::Win32::Graphics::Gdi::StretchBlt(
                    hdc_temp, sel.left, sel.top, sel.width() as i32, sel.height() as i32,
                    state.hdc_mem, src_x, src_y, src_w, src_h,
                    SRCCOPY,
                );

                draw_selection_border(hdc_temp, sel);
            }

            // Blit composited result to screen
            let _ = BitBlt(hdc, 0, 0, win_size.0, win_size.1, hdc_temp, 0, 0, SRCCOPY);

            let _ = DeleteDC(hdc_temp);
            let _ = DeleteObject(hbm_temp);
        });

        let _ = EndPaint(hwnd, &ps);
        LRESULT(0)
    }

    unsafe fn handle_mouse_down(_hwnd: HWND, lparam: LPARAM) -> LRESULT {
        let (x, y) = mouse_coords_from_lparam(lparam);

        OVERLAY_STATE.with(|state| {
            if let Some(ref mut state) = *state.borrow_mut() {
                state.is_selecting = true;
                state.start_x = x;
                state.start_y = y;
                state.current_x = x;
                state.current_y = y;
            }
        });

        LRESULT(0)
    }

    unsafe fn handle_mouse_move(hwnd: HWND, lparam: LPARAM) -> LRESULT {
        let (x, y) = mouse_coords_from_lparam(lparam);

        OVERLAY_STATE.with(|state| {
            if let Some(ref mut state) = *state.borrow_mut() {
                if state.is_selecting {
                    state.current_x = x;
                    state.current_y = y;
                    let _ = InvalidateRect(hwnd, None, false);
                }
            }
        });

        LRESULT(0)
    }

    unsafe fn handle_mouse_up(hwnd: HWND) -> LRESULT {
        OVERLAY_STATE.with(|state| {
            if let Some(ref mut state) = *state.borrow_mut() {
                if state.is_selecting {
                    state.is_selecting = false;
                    try_confirm_selection(state, hwnd);
                }
            }
        });

        LRESULT(0)
    }

    unsafe fn handle_key_down(hwnd: HWND, wparam: WPARAM) -> LRESULT {
        match wparam.0 {
            VK_ESCAPE => {
                OVERLAY_STATE.with(|state| {
                    if let Some(ref mut state) = *state.borrow_mut() {
                        state.result = Some(RegionSelectionResult::Cancelled);
                    }
                });
                let _ = DestroyWindow(hwnd);
            }
            VK_RETURN => {
                OVERLAY_STATE.with(|state| {
                    if let Some(ref mut state) = *state.borrow_mut() {
                        try_confirm_selection(state, hwnd);
                    }
                });
            }
            _ => {}
        }
        LRESULT(0)
    }

    /// Validate selection and confirm if large enough, closing the window
    unsafe fn try_confirm_selection(state: &mut OverlayState, hwnd: HWND) {
        let sel = state.selection_rect();
        if !sel.is_valid() {
            return;
        }

        let mut rect = RECT::default();
        let _ = GetClientRect(hwnd, &mut rect);
        let win_size = (rect.right - rect.left, rect.bottom - rect.top);
        let img_size = (state.screenshot.width, state.screenshot.height);

        let region = sel.to_capture_region(win_size, img_size);
        state.result = Some(RegionSelectionResult::Selected(region));
        let _ = DestroyWindow(hwnd);
    }
}

#[cfg(windows)]
pub use windows_impl::select_region;

#[cfg(not(windows))]
pub fn select_region(
    _screenshot: Screenshot,
    _window_info: WindowInfo,
) -> Result<RegionSelectionResult, Box<dyn std::error::Error>> {
    Err("Region selection is only supported on Windows".into())
}
