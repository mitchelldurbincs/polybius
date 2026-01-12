//! Desktop notification helpers

use std::path::Path;

/// Send a notification that audio was saved
pub fn notify_save_complete(path: &Path, duration_secs: f32) {
    let filename = path
        .file_name()
        .map(|s| s.to_string_lossy().to_string())
        .unwrap_or_else(|| "audio.wav".to_string());

    let body = format!("Saved {:.1}s to {}", duration_secs, filename);

    #[cfg(not(target_os = "windows"))]
    {
        if let Err(e) = notify_rust::Notification::new()
            .summary("Audio Saved")
            .body(&body)
            .timeout(notify_rust::Timeout::Milliseconds(3000))
            .show()
        {
            eprintln!("[WARN] Failed to show notification: {}", e);
        }
    }

    #[cfg(target_os = "windows")]
    {
        // On Windows, notify-rust may require additional setup
        // For now, just use the default notification
        if let Err(e) = notify_rust::Notification::new()
            .summary("Audio Saved")
            .body(&body)
            .show()
        {
            eprintln!("[WARN] Failed to show notification: {}", e);
        }
    }
}

/// Send an error notification
pub fn notify_error(message: &str) {
    if let Err(e) = notify_rust::Notification::new()
        .summary("Miner Error")
        .body(message)
        .show()
    {
        eprintln!("[WARN] Failed to show error notification: {}", e);
    }
}
