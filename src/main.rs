//! The Miner v0.2.1 - Stage 2: Ring Buffer Recording
//!
//! Records system audio continuously via WASAPI loopback, keeping only the
//! last 10 seconds in a ring buffer. Press Ctrl+Alt+C to save the buffer.
//!
//! Architecture notes:
//! - Producer has exclusive ownership by the audio thread (no mutex/priority inversion)
//! - Buffer is peeked, not drained, preserving full context for rapid successive saves

use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};

use cpal::traits::{DeviceTrait, HostTrait, StreamTrait};
use cpal::{FromSample, Sample, SampleFormat};
use global_hotkey::hotkey::{Code, HotKey, Modifiers};
use global_hotkey::{GlobalHotKeyEvent, GlobalHotKeyManager, HotKeyState};
use hound::{WavSpec, WavWriter};
use ringbuf::{
    traits::{Consumer, Producer, Split},
    HeapRb,
};

/// Duration of audio to keep in the ring buffer (in seconds)
const BUFFER_DURATION_SECS: usize = 10;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("The Miner v0.2.1 - Stage 2 (Ring Buffer)");
    println!("========================================\n");

    // 1. Setup global hotkey (Ctrl+Alt+C)
    let manager = GlobalHotKeyManager::new()?;
    let hotkey = HotKey::new(Some(Modifiers::CONTROL | Modifiers::ALT), Code::KeyC);
    manager.register(hotkey)?;
    println!("[OK] Registered hotkey: Ctrl+Alt+C");

    // 2. Get WASAPI host (Windows-specific)
    #[cfg(target_os = "windows")]
    let host = cpal::host_from_id(cpal::HostId::Wasapi)?;

    #[cfg(not(target_os = "windows"))]
    let host = cpal::default_host();

    // 3. Get default output device (we'll capture from it via loopback)
    let device = host
        .default_output_device()
        .ok_or("No output device found")?;

    println!("[OK] Using device: {}", device.name().unwrap_or_default());

    // 4. Get supported config
    let config = device.default_output_config()?;
    let sample_format = config.sample_format();
    let sample_rate = config.sample_rate().0;
    let channels = config.channels();

    println!(
        "[OK] Config: {} Hz, {} channels, {:?}",
        sample_rate, channels, sample_format
    );

    // 5. Calculate ring buffer size for BUFFER_DURATION_SECS seconds of audio
    let buffer_size = sample_rate as usize * channels as usize * BUFFER_DURATION_SECS;
    let memory_mb = (buffer_size * std::mem::size_of::<f32>()) as f32 / (1024.0 * 1024.0);
    println!(
        "[OK] Ring buffer: {} samples ({:.2} MB, {} seconds)",
        buffer_size, memory_mb, BUFFER_DURATION_SECS
    );

    // 6. Create lock-free ring buffer
    let ring = HeapRb::<f32>::new(buffer_size);
    let (producer, consumer) = ring.split();

    // 7. Flag to signal when to stop
    let running = Arc::new(AtomicBool::new(true));
    let running_clone = running.clone();

    // 8. Build input stream based on sample format
    // Producer is moved directly into the audio callback - no mutex needed!
    // This eliminates priority inversion risk since the audio thread has exclusive ownership.
    let stream = match sample_format {
        SampleFormat::F32 => {
            build_input_stream::<f32>(&device, &config.into(), producer, running_clone)?
        }
        SampleFormat::I16 => {
            build_input_stream::<i16>(&device, &config.into(), producer, running_clone)?
        }
        SampleFormat::U16 => {
            build_input_stream::<u16>(&device, &config.into(), producer, running_clone)?
        }
        format => return Err(format!("Unsupported sample format: {:?}", format).into()),
    };

    // 9. Start recording
    stream.play()?;
    println!(
        "\n[RECORDING] Continuously capturing last {} seconds...",
        BUFFER_DURATION_SECS
    );
    println!("[RECORDING] Press Ctrl+Alt+C to save buffer to WAV\n");

    // Store consumer for access in event loop
    // Note: Consumer stays on main thread only - no mutex needed for single-threaded access
    let mut consumer = consumer;

    // 10. Event loop - wait for hotkey
    loop {
        if let Ok(event) = GlobalHotKeyEvent::receiver().recv() {
            if event.id == hotkey.id() && event.state == HotKeyState::Pressed {
                println!("\n[SAVE] Hotkey pressed, saving buffer...");

                // PEEK at samples without consuming - this preserves the buffer!
                // The buffer acts as a sliding window, not a queue to be drained.
                // This allows rapid successive saves to each capture the full context.
                let samples: Vec<f32> = {
                    let (head, tail) = consumer.as_slices();
                    // Ring buffer data may be split across two contiguous regions
                    let mut samples = Vec::with_capacity(head.len() + tail.len());
                    samples.extend_from_slice(head);
                    samples.extend_from_slice(tail);
                    samples
                };

                let duration_secs = samples.len() as f32 / (sample_rate as f32 * channels as f32);

                println!(
                    "[INFO] Captured {} samples ({:.2} seconds)",
                    samples.len(),
                    duration_secs
                );

                if samples.is_empty() {
                    println!("[WARN] No audio in buffer! Check your audio output device.");
                } else {
                    // Generate timestamped filename
                    let filename = generate_filename();
                    write_wav(&filename, &samples, sample_rate, channels)?;
                    println!("[OK] Saved to {}", filename);
                }

                println!("\n[RECORDING] Continuing to record... Press Ctrl+Alt+C again to save\n");
            }
        }
    }
}

/// Generate a timestamped filename for the WAV file
fn generate_filename() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};

    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs();

    format!("audio_{}.wav", timestamp)
}

/// Build an input stream that captures audio and stores samples in the ring buffer
///
/// The producer is moved into the audio callback, giving the audio thread
/// exclusive ownership. This eliminates priority inversion - no mutex contention.
fn build_input_stream<T>(
    device: &cpal::Device,
    config: &cpal::StreamConfig,
    producer: ringbuf::HeapProd<f32>,
    running: Arc<AtomicBool>,
) -> Result<cpal::Stream, cpal::BuildStreamError>
where
    T: Sample + cpal::SizedSample,
    f32: FromSample<T>,
{
    // Producer is moved into closure - audio thread has exclusive ownership
    let mut producer = producer;

    device.build_input_stream(
        config,
        move |data: &[T], _: &cpal::InputCallbackInfo| {
            if !running.load(Ordering::Relaxed) {
                return;
            }

            for &sample in data {
                let sample_f32 = f32::from_sample(sample);
                // Push with overwrite - if buffer is full, oldest sample is discarded
                producer.push_overwrite(sample_f32);
            }
        },
        |err| eprintln!("[ERROR] Audio stream error: {}", err),
        None, // No timeout
    )
}

/// Write samples to a WAV file
fn write_wav(
    path: &str,
    samples: &[f32],
    sample_rate: u32,
    channels: u16,
) -> Result<(), Box<dyn std::error::Error>> {
    let spec = WavSpec {
        channels,
        sample_rate,
        bits_per_sample: 16,
        sample_format: hound::SampleFormat::Int,
    };

    let mut writer = WavWriter::create(path, spec)?;

    // Convert f32 samples to i16
    for &sample in samples {
        // Clamp to prevent overflow
        let clamped = sample.clamp(-1.0, 1.0);
        let amplitude = (clamped * i16::MAX as f32) as i16;
        writer.write_sample(amplitude)?;
    }

    writer.finalize()?;
    Ok(())
}
