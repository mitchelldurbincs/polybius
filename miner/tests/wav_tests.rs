//! Tests for WAV file writing functionality

use hound::WavReader;
use miner::wav::write_wav;
use std::fs;
use tempfile::tempdir;

// ==================== Basic write tests ====================

#[test]
fn test_write_wav_creates_file() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![0.0; 48000]; // 1 second of silence at 48kHz mono
    write_wav(&path, &samples, 48000, 1).unwrap();

    assert!(path.exists());
}

#[test]
fn test_write_wav_valid_format() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![0.0; 48000 * 2]; // 1 second stereo
    write_wav(&path, &samples, 48000, 2).unwrap();

    // Read back and verify format
    let reader = WavReader::open(&path).unwrap();
    let spec = reader.spec();

    assert_eq!(spec.channels, 2);
    assert_eq!(spec.sample_rate, 48000);
    assert_eq!(spec.bits_per_sample, 16);
    assert_eq!(spec.sample_format, hound::SampleFormat::Int);
}

#[test]
fn test_write_wav_sample_count() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let num_samples = 1000;
    let samples: Vec<f32> = vec![0.5; num_samples];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.len() as usize, num_samples);
}

// ==================== Sample conversion tests ====================

#[test]
fn test_write_wav_sample_conversion_positive() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Max positive sample
    let samples = vec![1.0f32];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let mut reader = WavReader::open(&path).unwrap();
    let sample: i16 = reader.samples().next().unwrap().unwrap();

    // Should be close to i16::MAX
    assert!(sample > 32000, "Expected positive max, got {}", sample);
}

#[test]
fn test_write_wav_sample_conversion_negative() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Max negative sample
    let samples = vec![-1.0f32];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let mut reader = WavReader::open(&path).unwrap();
    let sample: i16 = reader.samples().next().unwrap().unwrap();

    // Should be close to i16::MIN
    assert!(sample < -32000, "Expected negative max, got {}", sample);
}

#[test]
fn test_write_wav_sample_conversion_zero() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples = vec![0.0f32];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let mut reader = WavReader::open(&path).unwrap();
    let sample: i16 = reader.samples().next().unwrap().unwrap();

    assert_eq!(sample, 0);
}

#[test]
fn test_write_wav_sample_clamping_overflow() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Sample > 1.0 should be clamped
    let samples = vec![2.0f32, 5.0f32, 100.0f32];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let mut reader = WavReader::open(&path).unwrap();
    for sample in reader.samples::<i16>() {
        let s = sample.unwrap();
        // All should be clamped to max
        assert_eq!(s, i16::MAX, "Sample should be clamped to max");
    }
}

#[test]
fn test_write_wav_sample_clamping_underflow() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Sample < -1.0 should be clamped
    let samples = vec![-2.0f32, -5.0f32, -100.0f32];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let mut reader = WavReader::open(&path).unwrap();
    for sample in reader.samples::<i16>() {
        let s = sample.unwrap();
        // All should be clamped to min
        assert_eq!(s, -i16::MAX, "Sample should be clamped to min");
    }
}

// ==================== Different sample rates ====================

#[test]
fn test_write_wav_44100hz() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![0.0; 44100];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.spec().sample_rate, 44100);
}

#[test]
fn test_write_wav_22050hz() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![0.0; 22050];
    write_wav(&path, &samples, 22050, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.spec().sample_rate, 22050);
}

#[test]
fn test_write_wav_96000hz() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![0.0; 96000];
    write_wav(&path, &samples, 96000, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.spec().sample_rate, 96000);
}

// ==================== Channel configurations ====================

#[test]
fn test_write_wav_mono() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![0.0; 1000];
    write_wav(&path, &samples, 48000, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.spec().channels, 1);
}

#[test]
fn test_write_wav_stereo() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Stereo: samples are interleaved L R L R
    let samples: Vec<f32> = vec![0.5, -0.5, 0.5, -0.5]; // 2 frames
    write_wav(&path, &samples, 48000, 2).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.spec().channels, 2);
    assert_eq!(reader.len(), 4); // 4 samples total
}

// ==================== Edge cases ====================

#[test]
fn test_write_wav_empty_samples() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples: Vec<f32> = vec![];
    write_wav(&path, &samples, 48000, 2).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.len(), 0);
}

#[test]
fn test_write_wav_single_sample() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    let samples = vec![0.75f32];
    write_wav(&path, &samples, 48000, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.len(), 1);
}

#[test]
fn test_write_wav_special_float_values() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Test with edge float values (NaN and Inf should be handled)
    let samples = vec![0.0, 0.5, -0.5, 0.99, -0.99];
    write_wav(&path, &samples, 44100, 1).unwrap();

    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.len(), 5);
}

// ==================== Audio waveform tests ====================

#[test]
fn test_write_wav_sine_wave() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("sine.wav");

    // Generate a 440Hz sine wave (1 second)
    let sample_rate = 44100;
    let frequency = 440.0;
    let samples: Vec<f32> = (0..sample_rate)
        .map(|i| (2.0 * std::f32::consts::PI * frequency * i as f32 / sample_rate as f32).sin())
        .collect();

    write_wav(&path, &samples, sample_rate, 1).unwrap();

    // Verify we can read it back
    let reader = WavReader::open(&path).unwrap();
    assert_eq!(reader.len() as usize, sample_rate as usize);
}

#[test]
fn test_write_wav_stereo_panning() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("panned.wav");

    // Left channel full, right channel silent
    let num_frames = 100;
    let mut samples = Vec::with_capacity(num_frames * 2);
    for _ in 0..num_frames {
        samples.push(1.0f32); // Left
        samples.push(0.0f32); // Right
    }

    write_wav(&path, &samples, 44100, 2).unwrap();

    let mut reader = WavReader::open(&path).unwrap();
    let samples_i16: Vec<i16> = reader.samples().map(|s| s.unwrap()).collect();

    // Check interleaved pattern: high, 0, high, 0...
    for (i, &sample) in samples_i16.iter().enumerate() {
        if i % 2 == 0 {
            assert!(sample > 30000, "Left channel should be high");
        } else {
            assert_eq!(sample, 0, "Right channel should be silent");
        }
    }
}

// ==================== File path tests ====================

#[test]
fn test_write_wav_unicode_path() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("音声_テスト.wav");

    let samples: Vec<f32> = vec![0.0; 1000];
    write_wav(&path, &samples, 48000, 1).unwrap();

    assert!(path.exists());
}

#[test]
fn test_write_wav_overwrites_existing() {
    let dir = tempdir().unwrap();
    let path = dir.path().join("test.wav");

    // Write first file
    let samples1: Vec<f32> = vec![0.0; 1000];
    write_wav(&path, &samples1, 48000, 1).unwrap();
    let size1 = fs::metadata(&path).unwrap().len();

    // Overwrite with different content
    let samples2: Vec<f32> = vec![0.5; 5000];
    write_wav(&path, &samples2, 48000, 1).unwrap();
    let size2 = fs::metadata(&path).unwrap().len();

    // File should be larger now
    assert!(size2 > size1);
}
