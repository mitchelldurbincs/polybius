//! System tray icon and menu

use tray_icon::{
    menu::{Menu, MenuItem, PredefinedMenuItem},
    TrayIcon, TrayIconBuilder,
};

/// Menu item IDs
pub const MENU_SAVE_5S: &str = "save_5s";
pub const MENU_SAVE_10S: &str = "save_10s";
pub const MENU_SAVE_15S: &str = "save_15s";
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
        enable_5s: bool,
        enable_10s: bool,
        enable_15s: bool,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        // Create menu items
        let menu_save_5s = MenuItem::with_id(
            MENU_SAVE_5S,
            "Save Last 5 seconds\tCtrl+Alt+1",
            enable_5s,
            None,
        );
        let menu_save_10s = MenuItem::with_id(
            MENU_SAVE_10S,
            "Save Last 10 seconds\tCtrl+Alt+2",
            enable_10s,
            None,
        );
        let menu_save_15s = MenuItem::with_id(
            MENU_SAVE_15S,
            "Save Last 15 seconds\tCtrl+Alt+3",
            enable_15s,
            None,
        );
        let menu_pause = MenuItem::with_id(MENU_PAUSE, "Pause Recording", true, None);
        let menu_open_folder = MenuItem::with_id(MENU_OPEN_FOLDER, "Open Save Folder", true, None);
        let menu_settings = MenuItem::with_id(MENU_SETTINGS, "Settings...", true, None);
        let menu_quit = MenuItem::with_id(MENU_QUIT, "Quit", true, None);

        // Build the menu
        let menu = Menu::new();
        menu.append(&menu_save_5s)?;
        menu.append(&menu_save_10s)?;
        menu.append(&menu_save_15s)?;
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

/// Create a simple default icon (pickaxe inside red circle)
fn create_default_icon() -> Result<tray_icon::Icon, Box<dyn std::error::Error>> {
    // Create a 32x32 RGBA icon (pickaxe inside red circle)
    let size = 32u32;
    let mut rgba = vec![0u8; (size * size * 4) as usize];

    let center = size as f32 / 2.0;
    let radius = center - 2.0;

    // Helper to set a pixel with color
    let set_pixel = |rgba: &mut [u8], x: i32, y: i32, r: u8, g: u8, b: u8, a: u8| {
        if x >= 0 && x < size as i32 && y >= 0 && y < size as i32 {
            let idx = ((y as u32 * size + x as u32) * 4) as usize;
            rgba[idx] = r;
            rgba[idx + 1] = g;
            rgba[idx + 2] = b;
            rgba[idx + 3] = a;
        }
    };

    // Helper to check if point is inside circle
    let in_circle = |x: f32, y: f32| -> bool {
        let dx = x - center;
        let dy = y - center;
        (dx * dx + dy * dy).sqrt() <= radius
    };

    // First pass: draw the red circle
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
        }
    }

    // Second pass: draw the pickaxe in white
    // Pickaxe handle (diagonal from bottom-left to center-ish)
    for i in 0..14 {
        let x = 8 + i;
        let y = 24 - i;
        if in_circle(x as f32, y as f32) {
            set_pixel(&mut rgba, x, y, 255, 255, 255, 255);
            set_pixel(&mut rgba, x + 1, y, 255, 255, 255, 255);
            set_pixel(&mut rgba, x, y + 1, 255, 255, 255, 255);
        }
    }

    // Pick head (horizontal-ish line going right from top of handle)
    for i in 0..10 {
        let x = 18 + i;
        let y = 10 + i / 3;
        if in_circle(x as f32, y as f32) {
            set_pixel(&mut rgba, x, y, 255, 255, 255, 255);
            set_pixel(&mut rgba, x, y + 1, 255, 255, 255, 255);
            set_pixel(&mut rgba, x, y - 1, 255, 255, 255, 255);
        }
    }

    // Pick point (going left from top of handle)
    for i in 0..8 {
        let x = 18 - i;
        let y = 10 - i / 2;
        if in_circle(x as f32, y as f32) {
            set_pixel(&mut rgba, x, y, 255, 255, 255, 255);
            set_pixel(&mut rgba, x, y + 1, 255, 255, 255, 255);
        }
    }

    Ok(tray_icon::Icon::from_rgba(rgba, size, size)?)
}
