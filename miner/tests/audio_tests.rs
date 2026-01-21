//! Tests for audio module types
//!
//! Note: AudioCapture itself requires hardware access and cannot be easily unit tested.
//! These tests focus on the supporting types like BufferDuration and AudioFormat.

use miner::audio::{AudioFormat, BufferDuration};
use std::collections::HashSet;

// ==================== BufferDuration tests ====================

#[test]
fn test_buffer_duration_as_secs() {
    assert_eq!(BufferDuration::Seconds5.as_secs(), 5);
    assert_eq!(BufferDuration::Seconds10.as_secs(), 10);
    assert_eq!(BufferDuration::Seconds15.as_secs(), 15);
}

#[test]
fn test_buffer_duration_equality() {
    assert_eq!(BufferDuration::Seconds5, BufferDuration::Seconds5);
    assert_eq!(BufferDuration::Seconds10, BufferDuration::Seconds10);
    assert_eq!(BufferDuration::Seconds15, BufferDuration::Seconds15);

    assert_ne!(BufferDuration::Seconds5, BufferDuration::Seconds10);
    assert_ne!(BufferDuration::Seconds5, BufferDuration::Seconds15);
    assert_ne!(BufferDuration::Seconds10, BufferDuration::Seconds15);
}

#[test]
fn test_buffer_duration_clone() {
    let duration = BufferDuration::Seconds10;
    let cloned = duration;
    assert_eq!(duration, cloned);
}

#[test]
fn test_buffer_duration_copy() {
    let duration = BufferDuration::Seconds15;
    let copied = duration; // Copy, not move
    assert_eq!(copied, BufferDuration::Seconds15);
    assert_eq!(duration, BufferDuration::Seconds15); // Original still valid
}

#[test]
fn test_buffer_duration_debug() {
    // Verify Debug is implemented
    let duration = BufferDuration::Seconds5;
    let debug_str = format!("{:?}", duration);
    assert!(debug_str.contains("Seconds5"));
}

#[test]
fn test_buffer_duration_hash() {
    // BufferDuration should be usable as a HashMap key
    let mut set = HashSet::new();
    set.insert(BufferDuration::Seconds5);
    set.insert(BufferDuration::Seconds10);
    set.insert(BufferDuration::Seconds15);

    assert_eq!(set.len(), 3);
    assert!(set.contains(&BufferDuration::Seconds5));
    assert!(set.contains(&BufferDuration::Seconds10));
    assert!(set.contains(&BufferDuration::Seconds15));
}

#[test]
fn test_buffer_duration_in_vec() {
    let durations = [
        BufferDuration::Seconds5,
        BufferDuration::Seconds10,
        BufferDuration::Seconds15,
    ];

    assert_eq!(durations.len(), 3);
    assert!(durations.contains(&BufferDuration::Seconds10));
}

// ==================== AudioFormat tests ====================

#[test]
fn test_audio_format_creation() {
    let format = AudioFormat {
        sample_rate: 48000,
        channels: 2,
    };

    assert_eq!(format.sample_rate, 48000);
    assert_eq!(format.channels, 2);
}

#[test]
fn test_audio_format_clone() {
    let original = AudioFormat {
        sample_rate: 44100,
        channels: 1,
    };

    let cloned = original.clone();
    assert_eq!(cloned.sample_rate, 44100);
    assert_eq!(cloned.channels, 1);
}

#[test]
fn test_audio_format_debug() {
    let format = AudioFormat {
        sample_rate: 48000,
        channels: 2,
    };

    let debug_str = format!("{:?}", format);
    assert!(debug_str.contains("48000"));
    assert!(debug_str.contains("2"));
}

#[test]
fn test_audio_format_common_rates() {
    // Test common sample rates
    let rates = [8000, 11025, 22050, 44100, 48000, 96000, 192000];

    for rate in rates {
        let format = AudioFormat {
            sample_rate: rate,
            channels: 2,
        };
        assert_eq!(format.sample_rate, rate);
    }
}

#[test]
fn test_audio_format_channel_configs() {
    // Mono
    let mono = AudioFormat {
        sample_rate: 48000,
        channels: 1,
    };
    assert_eq!(mono.channels, 1);

    // Stereo
    let stereo = AudioFormat {
        sample_rate: 48000,
        channels: 2,
    };
    assert_eq!(stereo.channels, 2);

    // Surround (5.1)
    let surround = AudioFormat {
        sample_rate: 48000,
        channels: 6,
    };
    assert_eq!(surround.channels, 6);
}

// ==================== Buffer size calculation tests ====================

#[test]
fn test_buffer_size_calculation() {
    // Test the formula: sample_rate * channels * duration_seconds
    let format = AudioFormat {
        sample_rate: 48000,
        channels: 2,
    };

    let buffer_size_5s =
        format.sample_rate as usize * format.channels as usize * BufferDuration::Seconds5.as_secs();
    let buffer_size_10s = format.sample_rate as usize
        * format.channels as usize
        * BufferDuration::Seconds10.as_secs();
    let buffer_size_15s = format.sample_rate as usize
        * format.channels as usize
        * BufferDuration::Seconds15.as_secs();

    assert_eq!(buffer_size_5s, 480000); // 48000 * 2 * 5
    assert_eq!(buffer_size_10s, 960000); // 48000 * 2 * 10
    assert_eq!(buffer_size_15s, 1440000); // 48000 * 2 * 15
}

#[test]
fn test_buffer_memory_estimation() {
    // Each sample is f32 (4 bytes)
    let format = AudioFormat {
        sample_rate: 48000,
        channels: 2,
    };

    let samples_per_second = format.sample_rate as usize * format.channels as usize;
    let bytes_per_second = samples_per_second * std::mem::size_of::<f32>();
    let mb_per_second = bytes_per_second as f64 / (1024.0 * 1024.0);

    // ~0.366 MB per second for 48kHz stereo
    assert!(mb_per_second > 0.3 && mb_per_second < 0.4);

    // 5 second buffer should be ~1.83 MB
    let mb_5s = mb_per_second * 5.0;
    assert!(mb_5s > 1.5 && mb_5s < 2.0);
}

// ==================== Duration conversion tests ====================

#[test]
fn test_duration_to_samples() {
    let format = AudioFormat {
        sample_rate: 44100,
        channels: 1,
    };

    // 1 second = 44100 samples (mono)
    let samples_1s = format.sample_rate as usize;
    assert_eq!(samples_1s, 44100);

    // With stereo
    let stereo = AudioFormat {
        sample_rate: 44100,
        channels: 2,
    };
    let stereo_samples_1s = stereo.sample_rate as usize * stereo.channels as usize;
    assert_eq!(stereo_samples_1s, 88200);
}

#[test]
fn test_samples_to_duration() {
    let format = AudioFormat {
        sample_rate: 48000,
        channels: 2,
    };

    let num_samples = 480000; // 5 seconds of stereo audio
    let duration_secs = num_samples as f32 / (format.sample_rate as f32 * format.channels as f32);

    assert!((duration_secs - 5.0).abs() < 0.001);
}
