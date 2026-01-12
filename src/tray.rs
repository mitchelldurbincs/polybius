//! System tray icon and menu

use tray_icon::{
    menu::{Menu, MenuItem, PredefinedMenuItem},
    TrayIcon, TrayIconBuilder,
};

/// Menu item IDs
pub const MENU_SAVE_10S: &str = "save_10s";
pub const MENU_SAVE_30S: &str = "save_30s";
pub const MENU_SAVE_60S: &str = "save_60s";
pub const MENU_PAUSE: &str = "pause";
pub const MENU_OPEN_FOLDER: &str = "open_folder";
pub const MENU_SETTINGS: &str = "settings";
pub const MENU_QUIT: &str = "quit";

/// System tray manager
pub struct TrayManager {
    _tray: TrayIcon,
    menu_pause: MenuItem,
}

impl TrayManager {
    /// Create the system tray icon and menu
    pub fn new(
        enable_10s: bool,
        enable_30s: bool,
        enable_60s: bool,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        // Create menu items
        let menu_save_10s = MenuItem::with_id(
            MENU_SAVE_10S,
            "Save Last 10 seconds\tCtrl+Alt+1",
            enable_10s,
            None,
        );
        let menu_save_30s = MenuItem::with_id(
            MENU_SAVE_30S,
            "Save Last 30 seconds\tCtrl+Alt+2",
            enable_30s,
            None,
        );
        let menu_save_60s = MenuItem::with_id(
            MENU_SAVE_60S,
            "Save Last 60 seconds\tCtrl+Alt+3",
            enable_60s,
            None,
        );
        let menu_pause = MenuItem::with_id(MENU_PAUSE, "⏸ Pause Recording", true, None);
        let menu_open_folder =
            MenuItem::with_id(MENU_OPEN_FOLDER, "📁 Open Save Folder", true, None);
        let menu_settings = MenuItem::with_id(MENU_SETTINGS, "⚙️ Settings...", true, None);
        let menu_quit = MenuItem::with_id(MENU_QUIT, "🚪 Quit", true, None);

        // Build the menu
        let menu = Menu::new();
        menu.append(&menu_save_10s)?;
        menu.append(&menu_save_30s)?;
        menu.append(&menu_save_60s)?;
        menu.append(&PredefinedMenuItem::separator())?;
        menu.append(&menu_pause)?;
        menu.append(&menu_open_folder)?;
        menu.append(&menu_settings)?;
        menu.append(&PredefinedMenuItem::separator())?;
        menu.append(&menu_quit)?;

        // Create the tray icon
        // Using a simple colored icon for now (will be replaced with proper icon)
        let icon = create_default_icon()?;

        let tray = TrayIconBuilder::new()
            .with_icon(icon)
            .with_tooltip("The Miner - Recording")
            .with_menu(Box::new(menu))
            .build()?;

        println!("[OK] System tray icon created");

        Ok(Self {
            _tray: tray,
            menu_pause,
        })
    }

    /// Update the pause menu item text based on recording state
    pub fn set_paused(&self, paused: bool) {
        if paused {
            self.menu_pause.set_text("▶ Resume Recording");
        } else {
            self.menu_pause.set_text("⏸ Pause Recording");
        }
    }
}

/// Create a simple default icon (red circle for recording)
fn create_default_icon() -> Result<tray_icon::Icon, Box<dyn std::error::Error>> {
    // Create a simple 32x32 RGBA icon (red circle)
    let size = 32u32;
    let mut rgba = vec![0u8; (size * size * 4) as usize];

    let center = size as f32 / 2.0;
    let radius = center - 2.0;

    for y in 0..size {
        for x in 0..size {
            let dx = x as f32 - center;
            let dy = y as f32 - center;
            let dist = (dx * dx + dy * dy).sqrt();

            let idx = ((y * size + x) * 4) as usize;

            if dist <= radius {
                // Red circle
                rgba[idx] = 220; // R
                rgba[idx + 1] = 50; // G
                rgba[idx + 2] = 50; // B
                rgba[idx + 3] = 255; // A
            } else if dist <= radius + 1.0 {
                // Anti-aliased edge
                let alpha = ((radius + 1.0 - dist) * 255.0) as u8;
                rgba[idx] = 220;
                rgba[idx + 1] = 50;
                rgba[idx + 2] = 50;
                rgba[idx + 3] = alpha;
            }
            // else: transparent (already 0)
        }
    }

    Ok(tray_icon::Icon::from_rgba(rgba, size, size)?)
}
