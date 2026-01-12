//! Region selection overlay UI
//!
//! Displays a semi-transparent window showing the captured screenshot
//! and allows the user to drag-select a region.

use crate::config::CaptureRegion;
use crate::screenshot::Screenshot;
use crate::window_utils::WindowInfo;
use std::num::NonZeroU32;
use std::sync::Arc;
use winit::application::ApplicationHandler;
use winit::dpi::{PhysicalPosition, PhysicalSize};
use winit::event::{ElementState, KeyEvent, MouseButton, WindowEvent};
use winit::event_loop::{ActiveEventLoop, ControlFlow, EventLoop};
use winit::keyboard::{Key, NamedKey};
use winit::window::{Window, WindowId};

/// Result of region selection
#[derive(Debug)]
pub enum RegionSelectionResult {
    /// User selected a region
    Selected(CaptureRegion),
    /// User cancelled selection
    Cancelled,
}

/// State for the region selection process
struct RegionSelector {
    screenshot: Screenshot,
    window_info: WindowInfo,
    window: Option<Arc<Window>>,
    surface: Option<softbuffer::Surface<Arc<Window>, Arc<Window>>>,

    // Selection state
    is_selecting: bool,
    start_pos: Option<(i32, i32)>,
    current_pos: Option<(i32, i32)>,
    result: Option<RegionSelectionResult>,
}

impl RegionSelector {
    fn new(screenshot: Screenshot, window_info: WindowInfo) -> Self {
        Self {
            screenshot,
            window_info,
            window: None,
            surface: None,
            is_selecting: false,
            start_pos: None,
            current_pos: None,
            result: None,
        }
    }

    fn render(&mut self) {
        // Get selection rectangle first (before borrowing surface mutably)
        let selection = self.get_selection_rect();

        let Some(surface) = &mut self.surface else {
            return;
        };
        let Some(window) = &self.window else {
            return;
        };

        let size = window.inner_size();
        let width = size.width as usize;
        let height = size.height as usize;

        if width == 0 || height == 0 {
            return;
        }

        // Resize surface buffer if needed
        if surface
            .resize(
                NonZeroU32::new(size.width).unwrap(),
                NonZeroU32::new(size.height).unwrap(),
            )
            .is_err()
        {
            return;
        }

        let Ok(mut buffer) = surface.buffer_mut() else {
            return;
        };

        // Render each pixel
        for y in 0..height {
            for x in 0..width {
                let pixel_idx = y * width + x;

                // Get source pixel from screenshot (scale if needed)
                let src_x = (x * self.screenshot.width as usize) / width;
                let src_y = (y * self.screenshot.height as usize) / height;
                let src_idx = (src_y * self.screenshot.width as usize + src_x) * 4;

                let (r, g, b) = if src_idx + 2 < self.screenshot.data.len() {
                    (
                        self.screenshot.data[src_idx] as u32,
                        self.screenshot.data[src_idx + 1] as u32,
                        self.screenshot.data[src_idx + 2] as u32,
                    )
                } else {
                    (0, 0, 0)
                };

                // Apply dark overlay unless in selection region
                let (r, g, b) = if let Some((sx, sy, sw, sh)) = selection {
                    let xi = x as i32;
                    let yi = y as i32;
                    let in_selection = xi >= sx
                        && xi < sx + sw as i32
                        && yi >= sy
                        && yi < sy + sh as i32;

                    if in_selection {
                        // Full brightness in selection
                        (r, g, b)
                    } else {
                        // Darkened outside selection
                        (r / 2, g / 2, b / 2)
                    }
                } else {
                    // No selection yet - show darkened
                    (r / 2, g / 2, b / 2)
                };

                // Draw selection border
                let on_border = if let Some((sx, sy, sw, sh)) = selection {
                    let border_width = 2i32;
                    let xi = x as i32;
                    let yi = y as i32;

                    // Check if on horizontal borders
                    let on_top = yi >= sy - border_width
                        && yi < sy + border_width
                        && xi >= sx - border_width
                        && xi < sx + sw as i32 + border_width;
                    let on_bottom = yi >= sy + sh as i32 - border_width
                        && yi < sy + sh as i32 + border_width
                        && xi >= sx - border_width
                        && xi < sx + sw as i32 + border_width;

                    // Check if on vertical borders
                    let on_left = xi >= sx - border_width
                        && xi < sx + border_width
                        && yi >= sy - border_width
                        && yi < sy + sh as i32 + border_width;
                    let on_right = xi >= sx + sw as i32 - border_width
                        && xi < sx + sw as i32 + border_width
                        && yi >= sy - border_width
                        && yi < sy + sh as i32 + border_width;

                    on_top || on_bottom || on_left || on_right
                } else {
                    false
                };

                let (r, g, b) = if on_border {
                    (255u32, 100u32, 100u32) // Red border
                } else {
                    (r, g, b)
                };

                buffer[pixel_idx] = (r << 16) | (g << 8) | b;
            }
        }

        buffer.present().ok();
    }

    fn get_selection_rect(&self) -> Option<(i32, i32, u32, u32)> {
        let start = self.start_pos?;
        let current = self.current_pos?;

        let x = start.0.min(current.0);
        let y = start.1.min(current.1);
        let width = (start.0 - current.0).unsigned_abs();
        let height = (start.1 - current.1).unsigned_abs();

        if width > 0 && height > 0 {
            Some((x, y, width, height))
        } else {
            None
        }
    }

    fn finish_selection(&mut self) {
        if let Some((x, y, width, height)) = self.get_selection_rect() {
            // Convert screen coordinates to window-relative coordinates
            let window = self.window.as_ref().unwrap();
            let window_size = window.inner_size();

            // Scale coordinates back to original screenshot size
            let scale_x = self.screenshot.width as f64 / window_size.width as f64;
            let scale_y = self.screenshot.height as f64 / window_size.height as f64;

            let region = CaptureRegion {
                x: (x as f64 * scale_x) as i32,
                y: (y as f64 * scale_y) as i32,
                width: (width as f64 * scale_x) as u32,
                height: (height as f64 * scale_y) as u32,
            };

            self.result = Some(RegionSelectionResult::Selected(region));
        } else {
            self.result = Some(RegionSelectionResult::Cancelled);
        }
    }
}

impl ApplicationHandler for RegionSelector {
    fn resumed(&mut self, event_loop: &ActiveEventLoop) {
        if self.window.is_some() {
            return;
        }

        // Create window attributes
        let window_attrs = Window::default_attributes()
            .with_title("Select Region")
            .with_inner_size(PhysicalSize::new(
                self.window_info.width,
                self.window_info.height,
            ))
            .with_position(PhysicalPosition::new(self.window_info.x, self.window_info.y))
            .with_decorations(false)
            .with_resizable(false);

        match event_loop.create_window(window_attrs) {
            Ok(window) => {
                let window = Arc::new(window);

                // Create softbuffer surface
                let context = softbuffer::Context::new(window.clone()).ok();
                if let Some(context) = context {
                    let surface = softbuffer::Surface::new(&context, window.clone()).ok();
                    self.surface = surface;
                }

                self.window = Some(window);
            }
            Err(e) => {
                eprintln!("[ERROR] Failed to create overlay window: {}", e);
                self.result = Some(RegionSelectionResult::Cancelled);
                event_loop.exit();
            }
        }
    }

    fn window_event(&mut self, event_loop: &ActiveEventLoop, _id: WindowId, event: WindowEvent) {
        match event {
            WindowEvent::CloseRequested => {
                self.result = Some(RegionSelectionResult::Cancelled);
                event_loop.exit();
            }

            WindowEvent::KeyboardInput {
                event:
                    KeyEvent {
                        logical_key,
                        state: ElementState::Pressed,
                        ..
                    },
                ..
            } => match logical_key {
                Key::Named(NamedKey::Escape) => {
                    self.result = Some(RegionSelectionResult::Cancelled);
                    event_loop.exit();
                }
                Key::Named(NamedKey::Enter) => {
                    self.finish_selection();
                    event_loop.exit();
                }
                _ => {}
            },

            WindowEvent::MouseInput { state, button, .. } => {
                if button == MouseButton::Left {
                    match state {
                        ElementState::Pressed => {
                            self.is_selecting = true;
                            // start_pos will be set on next cursor move
                        }
                        ElementState::Released => {
                            if self.is_selecting {
                                self.is_selecting = false;
                                self.finish_selection();
                                event_loop.exit();
                            }
                        }
                    }
                }
            }

            WindowEvent::CursorMoved { position, .. } => {
                let x = position.x as i32;
                let y = position.y as i32;

                if self.is_selecting {
                    if self.start_pos.is_none() {
                        self.start_pos = Some((x, y));
                    }
                    self.current_pos = Some((x, y));

                    if let Some(window) = &self.window {
                        window.request_redraw();
                    }
                }
            }

            WindowEvent::RedrawRequested => {
                self.render();
            }

            _ => {}
        }
    }
}

/// Run the region selection overlay
pub fn select_region(
    screenshot: Screenshot,
    window_info: WindowInfo,
) -> Result<RegionSelectionResult, Box<dyn std::error::Error>> {
    let event_loop = EventLoop::new()?;
    event_loop.set_control_flow(ControlFlow::Wait);

    let mut selector = RegionSelector::new(screenshot, window_info);

    event_loop.run_app(&mut selector)?;

    Ok(selector.result.unwrap_or(RegionSelectionResult::Cancelled))
}
