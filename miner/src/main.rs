//! The Miner v0.4.0 - Stage 4: Vision Module
//!
//! Records system audio continuously via WASAPI loopback, with multiple
//! buffer durations (10s/30s/60s). Runs as a system tray application.
//! Now includes screenshot capture and OCR for extracting text.
//!
//! Hotkeys:
//! - Ctrl+Alt+1: Save last 10 seconds
//! - Ctrl+Alt+2: Save last 30 seconds
//! - Ctrl+Alt+3: Save last 60 seconds

mod audio;
mod clipboard;
mod config;
mod hotkeys;
mod metadata;
mod notifications;
mod ocr;
mod region_overlay;
mod screenshot;
mod tray;
mod vision;
mod wav;
mod window_utils;

use std::path::PathBuf;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use global_hotkey::{GlobalHotKeyEvent, HotKeyState};
use tray_icon::menu::MenuEvent;

use audio::{AudioCapture, BufferDuration};
use config::Config;
use hotkeys::{HotkeyAction, HotkeyManager};
use screenshot::Screenshot;
use tray::TrayManager;
use vision::VisionCapture;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("The Miner v0.4.0 - Stage 4 (Vision Module)");
    println!("==========================================\n");

    // Hide console window on Windows (release builds)
    #[cfg(all(windows, not(debug_assertions)))]
    hide_console_window();

    // 1. Load configuration (mutable for saving regions)
    let mut config = Config::load();

    // Ensure save directory exists
    let save_dir = config.ensure_save_dir()?;
    println!("[OK] Save directory: {}", save_dir.display());

    // 2. Initialize vision capture (screenshot + OCR)
    let vision = VisionCapture::new(&config.vision);
    if vision.is_enabled() {
        println!("[OK] Vision capture enabled");
        if vision.is_ocr_enabled() {
            println!("[OK] OCR enabled");
        }
    } else {
        println!("[INFO] Vision capture disabled");
    }

    // 3. Determine which buffers to enable
    let mut enabled_buffers = Vec::new();
    if config.audio.buffer_5s {
        enabled_buffers.push(BufferDuration::Seconds5);
    }
    if config.audio.buffer_10s {
        enabled_buffers.push(BufferDuration::Seconds10);
    }
    if config.audio.buffer_15s {
        enabled_buffers.push(BufferDuration::Seconds15);
    }

    if enabled_buffers.is_empty() {
        return Err("No buffers enabled in configuration".into());
    }

    // 4. Initialize audio capture
    let mut audio = AudioCapture::new(&enabled_buffers)?;

    // 5. Register hotkeys
    let hotkeys = HotkeyManager::new(
        &config.hotkeys.save_5s,
        &config.hotkeys.save_10s,
        &config.hotkeys.save_15s,
        &config.hotkeys.screenshot,
        &config.hotkeys.region_select,
        config.audio.buffer_5s,
        config.audio.buffer_10s,
        config.audio.buffer_15s,
    )?;

    // 6. Create system tray
    let tray = TrayManager::new(
        config.audio.buffer_5s,
        config.audio.buffer_10s,
        config.audio.buffer_15s,
    )?;

    println!("\n[RECORDING] Capturing audio to ring buffers...");
    println!("[RECORDING] Use hotkeys or tray menu to save\n");

    // 7. Main event loop
    let show_notifications = config.general.notifications;

    loop {
        // Pump Windows messages (required for global hotkeys to work)
        #[cfg(windows)]
        {
            use windows::Win32::UI::WindowsAndMessaging::{DispatchMessageW, PeekMessageW, TranslateMessage, MSG, PM_REMOVE};
            unsafe {
                let mut msg: MSG = std::mem::zeroed();
                while PeekMessageW(&mut msg, None, 0, 0, PM_REMOVE).as_bool() {
                    let _ = TranslateMessage(&msg);
                    DispatchMessageW(&msg);
                }
            }
        }

        // Check for hotkey events (non-blocking)
        if let Ok(event) = GlobalHotKeyEvent::receiver().try_recv() {
            println!("[DEBUG] Hotkey event received: id={}, state={:?}", event.id, event.state);
            if event.state == HotKeyState::Pressed {
                match hotkeys.action_for_id(event.id) {
                    Some(HotkeyAction::SaveBuffer(duration)) => {
                        handle_save(&mut audio, &vision, duration, &save_dir, show_notifications, &config)?;
                    }
                    Some(HotkeyAction::Screenshot) => {
                        handle_screenshot_only(&vision, &save_dir, show_notifications, &config)?;
                    }
                    Some(HotkeyAction::RegionSelect) => {
                        handle_region_select(&mut config)?;
                    }
                    None => {
                        println!("[DEBUG] Unknown hotkey id: {}", event.id);
                    }
                }
            }
        }

        // Check for menu events (non-blocking)
        if let Ok(event) = MenuEvent::receiver().try_recv() {
            match event.id.0.as_str() {
                tray::MENU_SAVE_5S => {
                    handle_save(
                        &mut audio,
                        &vision,
                        BufferDuration::Seconds5,
                        &save_dir,
                        show_notifications,
                        &config,
                    )?;
                }
                tray::MENU_SAVE_10S => {
                    handle_save(
                        &mut audio,
                        &vision,
                        BufferDuration::Seconds10,
                        &save_dir,
                        show_notifications,
                        &config,
                    )?;
                }
                tray::MENU_SAVE_15S => {
                    handle_save(
                        &mut audio,
                        &vision,
                        BufferDuration::Seconds15,
                        &save_dir,
                        show_notifications,
                        &config,
                    )?;
                }
                tray::MENU_PAUSE => {
                    if audio.is_recording() {
                        audio.pause();
                        tray.set_paused(true);
                        println!("[PAUSED] Recording paused");
                    } else {
                        audio.resume();
                        tray.set_paused(false);
                        println!("[RECORDING] Recording resumed");
                    }
                }
                tray::MENU_OPEN_FOLDER => {
                    open_folder(&save_dir);
                }
                tray::MENU_SETTINGS => {
                    open_config_file();
                }
                tray::MENU_QUIT => {
                    println!("[EXIT] Quitting...");
                    break;
                }
                _ => {}
            }
        }

        // Small sleep to prevent busy-waiting (~100 checks per second)
        std::thread::sleep(Duration::from_millis(10));
    }

    Ok(())
}

/// Handle saving audio and vision capture to files
fn handle_save(
    audio: &mut AudioCapture,
    vision: &VisionCapture,
    duration: BufferDuration,
    save_dir: &PathBuf,
    show_notification: bool,
    config: &Config,
) -> Result<(), Box<dyn std::error::Error>> {
    println!("\n[SAVE] Saving {} second buffer...", duration.as_secs());

    // Check if this buffer is available
    if !audio.has_buffer(duration) {
        println!("[WARN] {} second buffer is not enabled", duration.as_secs());
        if show_notification {
            notifications::notify_error(&format!(
                "{} second buffer is not enabled",
                duration.as_secs()
            ));
        }
        return Ok(());
    }

    // Get samples from buffer
    let samples = match audio.peek_buffer(duration) {
        Some(s) => s,
        None => {
            println!("[WARN] Could not read buffer");
            return Ok(());
        }
    };

    if samples.is_empty() {
        println!("[WARN] No audio in buffer!");
        if show_notification {
            notifications::notify_error("No audio in buffer");
        }
        return Ok(());
    }

    let duration_secs =
        samples.len() as f32 / (audio.format.sample_rate as f32 * audio.format.channels as f32);

    println!(
        "[INFO] Captured {} samples ({:.2} seconds)",
        samples.len(),
        duration_secs
    );

    // Generate timestamp for all files
    let timestamp = generate_timestamp();
    let base_name = format!("audio_{}", timestamp);
    let base_path = save_dir.join(&base_name);

    // 1. Save audio
    let audio_filename = format!("{}.wav", base_name);
    let audio_path = save_dir.join(&audio_filename);
    wav::write_wav(
        &audio_path,
        &samples,
        audio.format.sample_rate,
        audio.format.channels,
    )?;
    println!("[OK] Audio saved: {}", audio_path.display());

    // 2. Capture and save vision (screenshot + OCR)
    let mut screenshot_info = None;
    let mut capture_result = None;

    if vision.is_enabled() {
        // Get window title for region lookup
        let window_title = window_utils::get_foreground_window_info()
            .map(|info| info.title)
            .unwrap_or_default();

        match vision.capture_with_region(&window_title, &config.vision.regions) {
            Ok(result) => {
                // Save screenshot
                match vision.save_screenshot(&result, &base_path) {
                    Ok(info) => screenshot_info = info,
                    Err(e) => eprintln!("[WARN] Failed to save screenshot: {}", e),
                }
                capture_result = Some(result);
            }
            Err(e) => {
                eprintln!("[WARN] Vision capture failed: {}", e);
            }
        }
    }

    // 3. Save metadata JSON if enabled
    if vision.is_metadata_enabled() {
        let capture_ref = capture_result.as_ref();
        if let Some(ref result) = capture_ref {
            let metadata = vision.create_metadata(
                &audio_filename,
                duration_secs,
                audio.format.sample_rate,
                audio.format.channels,
                result,
                screenshot_info,
            );

            if let Err(e) = vision.save_metadata(&metadata, &base_path) {
                eprintln!("[WARN] Failed to save metadata: {}", e);
            }
        }
    }

    if show_notification {
        notifications::notify_save_complete(&audio_path, duration_secs);
    }

    println!("\n[RECORDING] Continuing to record...\n");

    Ok(())
}

/// Handle screenshot-only hotkey (save + clipboard)
fn handle_screenshot_only(
    vision: &VisionCapture,
    save_dir: &PathBuf,
    show_notification: bool,
    config: &Config,
) -> Result<(), Box<dyn std::error::Error>> {
    println!("\n[SCREENSHOT] Taking screenshot...");

    if !vision.is_enabled() {
        println!("[WARN] Vision capture is disabled");
        if show_notification {
            notifications::notify_error("Vision capture is disabled");
        }
        return Ok(());
    }

    // Get window info for region lookup
    let window_info = window_utils::get_foreground_window_info()
        .map_err(|e| format!("Failed to get window info: {}", e))?;

    // Capture screenshot with region if configured
    let capture_result = vision.capture_with_region(&window_info.title, &config.vision.regions)?;

    let screenshot = capture_result
        .screenshot
        .ok_or("No screenshot captured")?;

    // Generate filename
    let timestamp = generate_timestamp();
    let filename = format!("screenshot_{}.png", timestamp);
    let path = save_dir.join(&filename);

    // Save to file
    screenshot.save_png(&path)?;
    println!("[OK] Screenshot saved: {}", path.display());

    // Copy to clipboard
    match clipboard::copy_to_clipboard(&screenshot) {
        Ok(()) => println!("[OK] Screenshot copied to clipboard"),
        Err(e) => eprintln!("[WARN] Failed to copy to clipboard: {}", e),
    }

    if show_notification {
        notifications::notify_screenshot_saved(&path);
    }

    Ok(())
}

/// Handle region selection hotkey
fn handle_region_select(config: &mut Config) -> Result<(), Box<dyn std::error::Error>> {
    println!("\n[REGION] Opening region selection...");

    // Get foreground window info
    let window_info = window_utils::get_foreground_window_info()
        .map_err(|e| format!("Failed to get window info: {}", e))?;

    println!("[INFO] Target window: {}", window_info.title);

    // Capture current window screenshot
    let screenshot = Screenshot::capture_foreground()
        .map_err(|e| format!("Failed to capture window: {}", e))?;

    // Run region selection overlay
    match region_overlay::select_region(screenshot, window_info.clone()) {
        Ok(region_overlay::RegionSelectionResult::Selected(region)) => {
            println!(
                "[OK] Region selected: {}x{} at ({}, {})",
                region.width, region.height, region.x, region.y
            );

            // Save region to config
            config
                .vision
                .regions
                .insert(window_info.title.clone(), region);

            if let Err(e) = config.save() {
                eprintln!("[WARN] Failed to save config: {}", e);
            } else {
                println!("[OK] Region saved to config");
            }

            notifications::notify_region_saved(&window_info.title);
        }
        Ok(region_overlay::RegionSelectionResult::Cancelled) => {
            println!("[INFO] Region selection cancelled");
        }
        Err(e) => {
            eprintln!("[ERROR] Region selection failed: {}", e);
        }
    }

    Ok(())
}

/// Generate a Unix timestamp for file naming
fn generate_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

/// Open the save folder in the file manager
fn open_folder(path: &PathBuf) {
    #[cfg(target_os = "windows")]
    {
        let _ = std::process::Command::new("explorer").arg(path).spawn();
    }

    #[cfg(target_os = "macos")]
    {
        let _ = std::process::Command::new("open").arg(path).spawn();
    }

    #[cfg(target_os = "linux")]
    {
        let _ = std::process::Command::new("xdg-open").arg(path).spawn();
    }
}

/// Open the config file in the default editor
fn open_config_file() {
    if let Some(config_path) = Config::config_path() {
        #[cfg(target_os = "windows")]
        {
            let _ = std::process::Command::new("notepad")
                .arg(&config_path)
                .spawn();
        }

        #[cfg(target_os = "macos")]
        {
            let _ = std::process::Command::new("open").arg(&config_path).spawn();
        }

        #[cfg(target_os = "linux")]
        {
            let _ = std::process::Command::new("xdg-open")
                .arg(&config_path)
                .spawn();
        }
    }
}

/// Hide the console window on Windows
#[cfg(all(windows, not(debug_assertions)))]
fn hide_console_window() {
    use windows::Win32::System::Console::{FreeConsole, GetConsoleWindow};

    unsafe {
        let window = GetConsoleWindow();
        if !window.0.is_null() {
            let _ = FreeConsole();
        }
    }
}
