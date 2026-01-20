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
        SetDIBits, BITMAPINFO, BITMAPINFOHEADER, BI_RGB, DIB_RGB_COLORS, HDC, PAINTSTRUCT, SRCCOPY,
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
        hdc_dark: HDC,
        hbm_screenshot: windows::Win32::Graphics::Gdi::HBITMAP,
        hbm_darkened: windows::Win32::Graphics::Gdi::HBITMAP,
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

            // Create memory DCs and bitmaps for the screenshot (original + darkened)
            let hdc_screen = GetDC(None);
            let hdc_mem = CreateCompatibleDC(hdc_screen);
            let hdc_dark = CreateCompatibleDC(hdc_screen);
            let hbm_screenshot = CreateCompatibleBitmap(
                hdc_screen,
                screenshot.width as i32,
                screenshot.height as i32,
            );
            let hbm_darkened = CreateCompatibleBitmap(
                hdc_screen,
                screenshot.width as i32,
                screenshot.height as i32,
            );
            SelectObject(hdc_mem, hbm_screenshot);
            SelectObject(hdc_dark, hbm_darkened);

            // Convert RGBA to BGR for Windows bitmap (bottom-up) - original
            let mut bgr_data: Vec<u8> =
                Vec::with_capacity((screenshot.width * screenshot.height * 4) as usize);
            // Also create darkened version (40% brightness)
            let mut bgr_dark: Vec<u8> =
                Vec::with_capacity((screenshot.width * screenshot.height * 4) as usize);

            for y in (0..screenshot.height).rev() {
                for x in 0..screenshot.width {
                    let idx = ((y * screenshot.width + x) * 4) as usize;
                    let r = screenshot.data[idx];
                    let g = screenshot.data[idx + 1];
                    let b = screenshot.data[idx + 2];

                    // Original
                    bgr_data.push(b); // B
                    bgr_data.push(g); // G
                    bgr_data.push(r); // R
                    bgr_data.push(0); // Padding

                    // Darkened (40% brightness)
                    bgr_dark.push((b as u32 * 40 / 100) as u8);
                    bgr_dark.push((g as u32 * 40 / 100) as u8);
                    bgr_dark.push((r as u32 * 40 / 100) as u8);
                    bgr_dark.push(0);
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

            SetDIBits(
                hdc_dark,
                hbm_darkened,
                0,
                screenshot.height,
                bgr_dark.as_ptr() as *const _,
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
                    hdc_dark,
                    hbm_screenshot,
                    hbm_darkened,
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
            DeleteDC(hdc_dark);
            DeleteObject(hbm_screenshot);
            DeleteObject(hbm_darkened);

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
                        let win_width = rect.right - rect.left;
                        let win_height = rect.bottom - rect.top;
                        let img_width = state.screenshot.width as i32;
                        let img_height = state.screenshot.height as i32;

                        // Create a temporary DC for compositing
                        let hdc_temp = CreateCompatibleDC(hdc);
                        let hbm_temp = CreateCompatibleBitmap(hdc, win_width, win_height);
                        SelectObject(hdc_temp, hbm_temp);

                        // Start with darkened screenshot as base (fast StretchBlt)
                        windows::Win32::Graphics::Gdi::StretchBlt(
                            hdc_temp,
                            0,
                            0,
                            win_width,
                            win_height,
                            state.hdc_dark,
                            0,
                            0,
                            img_width,
                            img_height,
                            SRCCOPY,
                        );

                        // If we have a selection, draw the bright region on top
                        if state.is_selecting || (state.start_x != state.current_x) {
                            let sel_left = state.start_x.min(state.current_x);
                            let sel_top = state.start_y.min(state.current_y);
                            let sel_right = state.start_x.max(state.current_x);
                            let sel_bottom = state.start_y.max(state.current_y);
                            let sel_width = sel_right - sel_left;
                            let sel_height = sel_bottom - sel_top;

                            // Calculate source coordinates in original image
                            let scale_x = img_width as f64 / win_width as f64;
                            let scale_y = img_height as f64 / win_height as f64;
                            let src_x = (sel_left as f64 * scale_x) as i32;
                            let src_y = (sel_top as f64 * scale_y) as i32;
                            let src_w = (sel_width as f64 * scale_x) as i32;
                            let src_h = (sel_height as f64 * scale_y) as i32;

                            // Draw bright (original) region on top of darkened base
                            windows::Win32::Graphics::Gdi::StretchBlt(
                                hdc_temp,
                                sel_left,
                                sel_top,
                                sel_width,
                                sel_height,
                                state.hdc_mem,
                                src_x,
                                src_y,
                                src_w,
                                src_h,
                                SRCCOPY,
                            );

                            // Draw selection border
                            let border_brush = CreateSolidBrush(
                                windows::Win32::Foundation::COLORREF(0x00FF8040), // Orange-ish
                            );
                            // Top border
                            let border = RECT {
                                left: sel_left - 2,
                                top: sel_top - 2,
                                right: sel_right + 2,
                                bottom: sel_top,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            // Bottom border
                            let border = RECT {
                                left: sel_left - 2,
                                top: sel_bottom,
                                right: sel_right + 2,
                                bottom: sel_bottom + 2,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            // Left border
                            let border = RECT {
                                left: sel_left - 2,
                                top: sel_top,
                                right: sel_left,
                                bottom: sel_bottom,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            // Right border
                            let border = RECT {
                                left: sel_right,
                                top: sel_top,
                                right: sel_right + 2,
                                bottom: sel_bottom,
                            };
                            FillRect(hdc_temp, &border, border_brush);
                            DeleteObject(border_brush);
                        }

                        // Copy composited result to screen
                        BitBlt(hdc, 0, 0, win_width, win_height, hdc_temp, 0, 0, SRCCOPY);

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
