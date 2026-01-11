//! The Miner - Stage 1: Basic Audio Recorder
//!
//! Records system audio via WASAPI loopback and saves to WAV on Ctrl+Alt+C.

use std::sync::{Arc, Mutex};

use cpal::traits::{DeviceTrait, HostTrait, StreamTrait};
use cpal::{FromSample, Sample, SampleFormat};
use global_hotkey::{GlobalHotKeyEvent, GlobalHotKeyManager, HotKeyState};
use global_hotkey::hotkey::{Code, HotKey, Modifiers};
use hound::{WavSpec, WavWriter};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("The Miner v0.1 - Stage 1");
    println!("========================\n");

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

    println!("[OK] Config: {} Hz, {} channels, {:?}", sample_rate, channels, sample_format);

    // 5. Create shared sample buffer
    let samples: Arc<Mutex<Vec<f32>>> = Arc::new(Mutex::new(Vec::new()));
    let samples_clone = samples.clone();

    // 6. Build input stream based on sample format
    let stream = match sample_format {
        SampleFormat::F32 => build_input_stream::<f32>(&device, &config.into(), samples_clone)?,
        SampleFormat::I16 => build_input_stream::<i16>(&device, &config.into(), samples_clone)?,
        SampleFormat::U16 => build_input_stream::<u16>(&device, &config.into(), samples_clone)?,
        format => return Err(format!("Unsupported sample format: {:?}", format).into()),
    };

    // 7. Start recording
    stream.play()?;
    println!("\n[RECORDING] Press Ctrl+Alt+C to stop and save...\n");

    // 8. Event loop - wait for hotkey
    loop {
        if let Ok(event) = GlobalHotKeyEvent::receiver().recv() {
            if event.id == hotkey.id() && event.state == HotKeyState::Pressed {
                println!("\n[STOP] Hotkey pressed, saving audio...");
                break;
            }
        }
    }

    // 9. Stop stream
    drop(stream);

    // 10. Write WAV file
    let samples_data = samples.lock().unwrap();
    let duration_secs = samples_data.len() as f32 / (sample_rate as f32 * channels as f32);

    println!("[INFO] Captured {} samples ({:.2} seconds)", samples_data.len(), duration_secs);

    if samples_data.is_empty() {
        println!("[WARN] No audio captured! Check your audio output device.");
        return Ok(());
    }

    write_wav("audio.wav", &samples_data, sample_rate, channels)?;
    println!("[OK] Saved to audio.wav");

    Ok(())
}

/// Build an input stream that captures audio and stores samples in the buffer
fn build_input_stream<T>(
    device: &cpal::Device,
    config: &cpal::StreamConfig,
    samples: Arc<Mutex<Vec<f32>>>,
) -> Result<cpal::Stream, cpal::BuildStreamError>
where
    T: Sample + cpal::SizedSample,
    f32: FromSample<T>,
{
    device.build_input_stream(
        config,
        move |data: &[T], _: &cpal::InputCallbackInfo| {
            if let Ok(mut buffer) = samples.try_lock() {
                for &sample in data {
                    buffer.push(f32::from_sample(sample));
                }
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
        let amplitude = (sample * i16::MAX as f32) as i16;
        writer.write_sample(amplitude)?;
    }

    writer.finalize()?;
    Ok(())
}
