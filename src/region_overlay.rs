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
        BeginPaint, BitBlt, CreateCompatibleBitmap, CreateCompatibleDC, CreateSolidBrush,
        DeleteDC, DeleteObject, EndPaint, FillRect, GetDC, InvalidateRect, ReleaseDC,
        SelectObject, SetDIBits, BITMAPINFO, BITMAPINFOHEADER, BI_RGB, DIB_RGB_COLORS, HDC,
        PAINTSTRUCT, SRCCOPY,
    };
    use windows::Win32::System::LibraryLoader::GetModuleHandleW;
    use windows::Win32::UI::WindowsAndMessaging::{
        CreateWindowExW, DefWindowProcW, DestroyWindow, DispatchMessageW, GetClientRect,
        GetMessageW, PostQuitMessage, RegisterClassW, ShowWindow, TranslateMessage, CS_HREDRAW,
        CS_VREDRAW, MSG, SW_SHOW, WINDOW_EX_STYLE, WM_DESTROY, WM_KEYDOWN, WM_LBUTTONDOWN,
        WM_LBUTTONUP, WM_MOUSEMOVE, WM_PAINT, WNDCLASSW, WS_POPUP, WS_VISIBLE,
    };

    const VK_ESCAPE: usize = 0x1B;
    const VK_RETURN: usize = 0x0D;

    struct OverlayState {
        screenshot: Screenshot,
        window_info: WindowInfo,
        is_selecting: bool,
        start_x: i32,
        start_y: i32,
        current_x: i32,
        current_y: i32,
        result: Option<RegionSelectionResult>,
        hdc_mem: HDC,
        hbm_screenshot: windows::Win32::Graphics::Gdi::HBITMAP,
    }

    thread_local! {
        static OVERLAY_STATE: RefCell<Option<OverlayState>> = const { RefCell::new(None) };
    }

    pub fn select_region(
        screenshot: Screenshot,
        window_info: WindowInfo,
    ) -> Result<RegionSelectionResult, Box<dyn std::error::Error>> {
        unsafe {
            let hinstance = GetModuleHandleW(None)?;

            // Register window class
            let class_name = w!("MinerRegionOverlay");
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

            // Create memory DC and bitmap for the screenshot
            let hdc_screen = GetDC(None);
            let hdc_mem = CreateCompatibleDC(hdc_screen);
            let hbm_screenshot = CreateCompatibleBitmap(
                hdc_screen,
                screenshot.width as i32,
                screenshot.height as i32,
            );
            SelectObject(hdc_mem, hbm_screenshot);

            // Convert RGBA to BGR for Windows bitmap (bottom-up)
            let mut bgr_data: Vec<u8> =
                Vec::with_capacity((screenshot.width * screenshot.height * 4) as usize);
            for y in (0..screenshot.height).rev() {
                for x in 0..screenshot.width {
                    let idx = ((y * screenshot.width + x) * 4) as usize;
                    bgr_data.push(screenshot.data[idx + 2]); // B
                    bgr_data.push(screenshot.data[idx + 1]); // G
                    bgr_data.push(screenshot.data[idx]); // R
                    bgr_data.push(0); // Padding
                }
            }

            let bmi = BITMAPINFO {
                bmiHeader: BITMAPINFOHEADER {
                    biSize: std::mem::size_of::<BITMAPINFOHEADER>() as u32,
                    biWidth: screenshot.width as i32,
                    biHeight: screenshot.height as i32,
                    biPlanes: 1,
                    biBitCount: 32,
                    biCompression: BI_RGB.0,
                    ..Default::default()
                },
                ..Default::default()
            };

            SetDIBits(
                hdc_mem,
                hbm_screenshot,
                0,
                screenshot.height,
                bgr_data.as_ptr() as *const _,
                &bmi,
                DIB_RGB_COLORS,
            );

            ReleaseDC(None, hdc_screen);

            // Store state
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
                    hbm_screenshot,
                });
            });

            // Create the overlay window
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

            ShowWindow(hwnd, SW_SHOW);

            // Message loop
            let mut msg = MSG::default();
            while GetMessageW(&mut msg, None, 0, 0).as_bool() {
                let _ = TranslateMessage(&msg);
                DispatchMessageW(&msg);
            }

            // Cleanup
            DeleteDC(hdc_mem);
            DeleteObject(hbm_screenshot);

            // Get result
            let result = OVERLAY_STATE.with(|state| {
                state
                    .borrow_mut()
                    .take()
                    .and_then(|s| s.result)
                    .unwrap_or(RegionSelectionResult::Cancelled)
            });

            Ok(result)
        }
    }

    unsafe extern "system" fn window_proc(
        hwnd: HWND,
        msg: u32,
        wparam: WPARAM,
        lparam: LPARAM,
    ) -> LRESULT {
        match msg {
            WM_PAINT => {
                let mut ps = PAINTSTRUCT::default();
                let hdc = BeginPaint(hwnd, &mut ps);

                OVERLAY_STATE.with(|state| {
                    if let Some(ref state) = *state.borrow() {
                        let mut rect = RECT::default();
                        GetClientRect(hwnd, &mut rect);
                        let width = rect.right - rect.left;
                        let height = rect.bottom - rect.top;

                        // Create a temporary DC for compositing
                        let hdc_temp = CreateCompatibleDC(hdc);
                        let hbm_temp = CreateCompatibleBitmap(hdc, width, height);
                        SelectObject(hdc_temp, hbm_temp);

                        // Draw screenshot scaled to window size
                        windows::Win32::Graphics::Gdi::StretchBlt(
                            hdc_temp,
                            0,
                            0,
                            width,
                            height,
                            state.hdc_mem,
                            0,
                            0,
                            state.screenshot.width as i32,
                            state.screenshot.height as i32,
                            SRCCOPY,
                        );

                        // Apply dark overlay to non-selected areas
                        let dark_brush = CreateSolidBrush(
                            windows::Win32::Foundation::COLORREF(0x00404040),
                        );

                        // If we have a selection, darken everything except the selection
                        if state.is_selecting || (state.start_x != state.current_x) {
                            let sel_left = state.start_x.min(state.current_x);
                            let sel_top = state.start_y.min(state.current_y);
                            let sel_right = state.start_x.max(state.current_x);
                            let sel_bottom = state.start_y.max(state.current_y);

                            // Draw darkened regions around selection
                            // Top
                            if sel_top > 0 {
                                let r = RECT {
                                    left: 0,
                                    top: 0,
                                    right: width,
                                    bottom: sel_top,
                                };
                                darken_rect(hdc_temp, &r);
                            }
                            // Bottom
                            if sel_bottom < height {
                                let r = RECT {
                                    left: 0,
                                    top: sel_bottom,
                                    right: width,
                                    bottom: height,
                                };
                                darken_rect(hdc_temp, &r);
                            }
                            // Left
                            let r = RECT {
                                left: 0,
                                top: sel_top,
                                right: sel_left,
                                bottom: sel_bottom,
                            };
                            darken_rect(hdc_temp, &r);
                            // Right
                            let r = RECT {
                                left: sel_right,
                                top: sel_top,
                                right: width,
                                bottom: sel_bottom,
                            };
                            darken_rect(hdc_temp, &r);

                            // Draw selection border
                            let border_brush = CreateSolidBrush(
                                windows::Win32::Foundation::COLORREF(0x004080FF),
                            );
                            let border = RECT {
                                left: sel_left - 2,
                                top: sel_top - 2,
                                right: sel_right + 2,
                                bottom: sel_top,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            let border = RECT {
                                left: sel_left - 2,
                                top: sel_bottom,
                                right: sel_right + 2,
                                bottom: sel_bottom + 2,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            let border = RECT {
                                left: sel_left - 2,
                                top: sel_top,
                                right: sel_left,
                                bottom: sel_bottom,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            let border = RECT {
                                left: sel_right,
                                top: sel_top,
                                right: sel_right + 2,
                                bottom: sel_bottom,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            DeleteObject(border_brush);
                        } else {
                            // No selection - darken entire image
                            let r = RECT {
                                left: 0,
                                top: 0,
                                right: width,
                                bottom: height,
                            };
                            darken_rect(hdc_temp, &r);
                        }

                        DeleteObject(dark_brush);

                        // Copy to screen
                        BitBlt(hdc, 0, 0, width, height, hdc_temp, 0, 0, SRCCOPY);

                        DeleteDC(hdc_temp);
                        DeleteObject(hbm_temp);
                    }
                });

                EndPaint(hwnd, &ps);
                LRESULT(0)
            }

            WM_LBUTTONDOWN => {
                let x = (lparam.0 & 0xFFFF) as i16 as i32;
                let y = ((lparam.0 >> 16) & 0xFFFF) as i16 as i32;

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

            WM_MOUSEMOVE => {
                let x = (lparam.0 & 0xFFFF) as i16 as i32;
                let y = ((lparam.0 >> 16) & 0xFFFF) as i16 as i32;

                OVERLAY_STATE.with(|state| {
                    if let Some(ref mut state) = *state.borrow_mut() {
                        if state.is_selecting {
                            state.current_x = x;
                            state.current_y = y;
                            InvalidateRect(hwnd, None, false);
                        }
                    }
                });

                LRESULT(0)
            }

            WM_LBUTTONUP => {
                OVERLAY_STATE.with(|state| {
                    if let Some(ref mut state) = *state.borrow_mut() {
                        if state.is_selecting {
                            state.is_selecting = false;

                            let sel_left = state.start_x.min(state.current_x);
                            let sel_top = state.start_y.min(state.current_y);
                            let sel_right = state.start_x.max(state.current_x);
                            let sel_bottom = state.start_y.max(state.current_y);

                            let sel_width = (sel_right - sel_left) as u32;
                            let sel_height = (sel_bottom - sel_top) as u32;

                            if sel_width > 10 && sel_height > 10 {
                                // Get window dimensions for scaling
                                let mut rect = RECT::default();
                                GetClientRect(hwnd, &mut rect);
                                let win_width = (rect.right - rect.left) as f64;
                                let win_height = (rect.bottom - rect.top) as f64;

                                // Scale to original screenshot coordinates
                                let scale_x = state.screenshot.width as f64 / win_width;
                                let scale_y = state.screenshot.height as f64 / win_height;

                                let region = CaptureRegion {
                                    x: (sel_left as f64 * scale_x) as i32,
                                    y: (sel_top as f64 * scale_y) as i32,
                                    width: (sel_width as f64 * scale_x) as u32,
                                    height: (sel_height as f64 * scale_y) as u32,
                                };

                                state.result = Some(RegionSelectionResult::Selected(region));
                                DestroyWindow(hwnd);
                            }
                        }
                    }
                });

                LRESULT(0)
            }

            WM_KEYDOWN => {
                match wparam.0 {
                    VK_ESCAPE => {
                        OVERLAY_STATE.with(|state| {
                            if let Some(ref mut state) = *state.borrow_mut() {
                                state.result = Some(RegionSelectionResult::Cancelled);
                            }
                        });
                        DestroyWindow(hwnd);
                    }
                    VK_RETURN => {
                        // Confirm current selection
                        OVERLAY_STATE.with(|state| {
                            if let Some(ref mut state) = *state.borrow_mut() {
                                let sel_left = state.start_x.min(state.current_x);
                                let sel_top = state.start_y.min(state.current_y);
                                let sel_right = state.start_x.max(state.current_x);
                                let sel_bottom = state.start_y.max(state.current_y);

                                let sel_width = (sel_right - sel_left) as u32;
                                let sel_height = (sel_bottom - sel_top) as u32;

                                if sel_width > 10 && sel_height > 10 {
                                    let mut rect = RECT::default();
                                    GetClientRect(hwnd, &mut rect);
                                    let win_width = (rect.right - rect.left) as f64;
                                    let win_height = (rect.bottom - rect.top) as f64;

                                    let scale_x = state.screenshot.width as f64 / win_width;
                                    let scale_y = state.screenshot.height as f64 / win_height;

                                    let region = CaptureRegion {
                                        x: (sel_left as f64 * scale_x) as i32,
                                        y: (sel_top as f64 * scale_y) as i32,
                                        width: (sel_width as f64 * scale_x) as u32,
                                        height: (sel_height as f64 * scale_y) as u32,
                                    };

                                    state.result = Some(RegionSelectionResult::Selected(region));
                                    DestroyWindow(hwnd);
                                }
                            }
                        });
                    }
                    _ => {}
                }
                LRESULT(0)
            }

            WM_DESTROY => {
                PostQuitMessage(0);
                LRESULT(0)
            }

            _ => DefWindowProcW(hwnd, msg, wparam, lparam),
        }
    }

    /// Darken a rectangle by drawing a semi-transparent overlay
    unsafe fn darken_rect(hdc: HDC, rect: &RECT) {
        // Since we can't easily do alpha blending with GDI,
        // we'll just draw a dark solid color for now
        let dark_brush = CreateSolidBrush(windows::Win32::Foundation::COLORREF(0x00000000));

        // Create a pattern that gives appearance of darkening
        // by drawing every other pixel
        for y in rect.top..rect.bottom {
            for x in rect.left..rect.right {
                if (x + y) % 2 == 0 {
                    windows::Win32::Graphics::Gdi::SetPixel(
                        hdc,
                        x,
                        y,
                        windows::Win32::Foundation::COLORREF(0x00000000),
                    );
                }
            }
        }

        DeleteObject(dark_brush);
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
