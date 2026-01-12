//! The Miner v0.3.0 - Stage 3: System Tray & Multi-Duration
//!
//! Records system audio continuously via WASAPI loopback, with multiple
//! buffer durations (10s/30s/60s). Runs as a system tray application.
//!
//! Hotkeys:
//! - Ctrl+Alt+1: Save last 10 seconds
//! - Ctrl+Alt+2: Save last 30 seconds
//! - Ctrl+Alt+3: Save last 60 seconds

mod audio;
mod config;
mod hotkeys;
mod notifications;
mod tray;
mod wav;

use std::path::PathBuf;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use global_hotkey::{GlobalHotKeyEvent, HotKeyState};
use tray_icon::menu::MenuEvent;

use audio::{AudioCapture, BufferDuration};
use config::Config;
use hotkeys::HotkeyManager;
use tray::TrayManager;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("The Miner v0.3.0 - Stage 3 (System Tray)");
    println!("========================================\n");

    // Hide console window on Windows (release builds)
    #[cfg(all(windows, not(debug_assertions)))]
    hide_console_window();

    // 1. Load configuration
    let config = Config::load();

    // Ensure save directory exists
    let save_dir = config.ensure_save_dir()?;
    println!("[OK] Save directory: {}", save_dir.display());

    // 2. Determine which buffers to enable
    let mut enabled_buffers = Vec::new();
    if config.audio.buffer_10s {
        enabled_buffers.push(BufferDuration::Seconds10);
    }
    if config.audio.buffer_30s {
        enabled_buffers.push(BufferDuration::Seconds30);
    }
    if config.audio.buffer_60s {
        enabled_buffers.push(BufferDuration::Seconds60);
    }

    if enabled_buffers.is_empty() {
        return Err("No buffers enabled in configuration".into());
    }

    // 3. Initialize audio capture
    let mut audio = AudioCapture::new(&enabled_buffers)?;

    // 4. Register hotkeys
    let hotkeys = HotkeyManager::new(
        &config.hotkeys.save_10s,
        &config.hotkeys.save_30s,
        &config.hotkeys.save_60s,
        config.audio.buffer_10s,
        config.audio.buffer_30s,
        config.audio.buffer_60s,
    )?;

    // 5. Create system tray
    let tray = TrayManager::new(
        config.audio.buffer_10s,
        config.audio.buffer_30s,
        config.audio.buffer_60s,
    )?;

    println!("\n[RECORDING] Capturing audio to ring buffers...");
    println!("[RECORDING] Use hotkeys or tray menu to save\n");

    // 6. Main event loop
    let show_notifications = config.general.notifications;

    loop {
        // Check for hotkey events (non-blocking)
        if let Ok(event) = GlobalHotKeyEvent::receiver().try_recv() {
            if event.state == HotKeyState::Pressed {
                if let Some(duration) = hotkeys.duration_for_id(event.id) {
                    handle_save(&mut audio, duration, &save_dir, show_notifications)?;
                }
            }
        }

        // Check for menu events (non-blocking)
        if let Ok(event) = MenuEvent::receiver().try_recv() {
            match event.id.0.as_str() {
                tray::MENU_SAVE_10S => {
                    handle_save(&mut audio, BufferDuration::Seconds10, &save_dir, show_notifications)?;
                }
                tray::MENU_SAVE_30S => {
                    handle_save(&mut audio, BufferDuration::Seconds30, &save_dir, show_notifications)?;
                }
                tray::MENU_SAVE_60S => {
                    handle_save(&mut audio, BufferDuration::Seconds60, &save_dir, show_notifications)?;
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

/// Handle saving audio to a file
fn handle_save(
    audio: &mut AudioCapture,
    duration: BufferDuration,
    save_dir: &PathBuf,
    show_notification: bool,
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

    // Generate filename and save
    let filename = generate_filename();
    let path = save_dir.join(&filename);

    wav::write_wav(&path, &samples, audio.format.sample_rate, audio.format.channels)?;

    println!("[OK] Saved to {}", path.display());

    if show_notification {
        notifications::notify_save_complete(&path, duration_secs);
    }

    println!("\n[RECORDING] Continuing to record...\n");

    Ok(())
}

/// Generate a timestamped filename
fn generate_filename() -> String {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs();

    format!("audio_{}.wav", timestamp)
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
            let _ = std::process::Command::new("notepad").arg(&config_path).spawn();
        }

        #[cfg(target_os = "macos")]
        {
            let _ = std::process::Command::new("open").arg(&config_path).spawn();
        }

        #[cfg(target_os = "linux")]
        {
            let _ = std::process::Command::new("xdg-open").arg(&config_path).spawn();
        }
    }
}

/// Hide the console window on Windows
#[cfg(all(windows, not(debug_assertions)))]
fn hide_console_window() {
    use windows::Win32::System::Console::{FreeConsole, GetConsoleWindow};

    unsafe {
        let window = GetConsoleWindow();
        if window.0 != 0 {
            let _ = FreeConsole();
        }
    }
}
