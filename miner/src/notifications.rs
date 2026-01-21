//! Desktop notification helpers

use std::path::Path;

/// Internal helper to show a notification
fn show_notification(summary: &str, body: &str) {
    if let Err(e) = notify_rust::Notification::new()
        .summary(summary)
        .body(body)
        .show()
    {
        eprintln!("[WARN] Failed to show notification: {}", e);
    }
}

/// Extract filename from path with a default fallback
fn filename_or_default(path: &Path, default: &str) -> String {
    path.file_name()
        .map(|s| s.to_string_lossy().to_string())
        .unwrap_or_else(|| default.to_string())
}

/// Send a notification that audio was saved
pub fn notify_save_complete(path: &Path, duration_secs: f32) {
    let filename = filename_or_default(path, "audio.wav");
    show_notification("Audio Saved", &format!("Saved {:.1}s to {}", duration_secs, filename));
}

/// Send an error notification
pub fn notify_error(message: &str) {
    show_notification("Miner Error", message);
}

/// Send a notification that a screenshot was saved
pub fn notify_screenshot_saved(path: &Path) {
    let filename = filename_or_default(path, "screenshot.png");
    show_notification("Screenshot Saved", &format!("Saved {} (copied to clipboard)", filename));
}

/// Send a notification that a region was saved
pub fn notify_region_saved(window_title: &str) {
    show_notification("Region Saved", &format!("Region set for '{}'", window_title));
}
