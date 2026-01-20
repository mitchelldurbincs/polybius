//! Audio capture and ring buffer management

use cpal::traits::{DeviceTrait, HostTrait, StreamTrait};
use cpal::{Device, FromSample, Sample, SampleFormat, Stream, StreamConfig};
use ringbuf::{
    traits::{Consumer, Producer, Split},
    HeapCons, HeapProd, HeapRb,
};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};

/// Duration options for audio buffers
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum BufferDuration {
    Seconds5,
    Seconds10,
    Seconds15,
}

impl BufferDuration {
    pub fn as_secs(&self) -> usize {
        match self {
            BufferDuration::Seconds5 => 5,
            BufferDuration::Seconds10 => 10,
            BufferDuration::Seconds15 => 15,
        }
    }
}

/// Audio format information
#[derive(Debug, Clone)]
pub struct AudioFormat {
    pub sample_rate: u32,
    pub channels: u16,
}

/// Manages audio capture with multiple ring buffers
pub struct AudioCapture {
    pub format: AudioFormat,
    _stream: Stream,
    consumers: Vec<(BufferDuration, HeapCons<f32>)>,
    running: Arc<AtomicBool>,
}

impl AudioCapture {
    /// Initialize audio capture with specified buffer durations
    pub fn new(enabled_buffers: &[BufferDuration]) -> Result<Self, Box<dyn std::error::Error>> {
        // Get WASAPI host on Windows, default host otherwise
        #[cfg(target_os = "windows")]
        let host = cpal::host_from_id(cpal::HostId::Wasapi)?;

        #[cfg(not(target_os = "windows"))]
        let host = cpal::default_host();

        // Get default output device (for loopback capture)
        let device = host
            .default_output_device()
            .ok_or("No output device found")?;

        println!("[OK] Using device: {}", device.name().unwrap_or_default());

        // Get supported config
        let config = device.default_output_config()?;
        let sample_format = config.sample_format();
        let sample_rate = config.sample_rate().0;
        let channels = config.channels();

        println!(
            "[OK] Config: {} Hz, {} channels, {:?}",
            sample_rate, channels, sample_format
        );

        let format = AudioFormat {
            sample_rate,
            channels,
        };

        // Create ring buffers for each enabled duration
        let mut producers: Vec<(BufferDuration, HeapProd<f32>)> = Vec::new();
        let mut consumers: Vec<(BufferDuration, HeapCons<f32>)> = Vec::new();

        for &duration in enabled_buffers {
            let buffer_size = sample_rate as usize * channels as usize * duration.as_secs();
            let memory_mb = (buffer_size * std::mem::size_of::<f32>()) as f32 / (1024.0 * 1024.0);

            println!(
                "[OK] {} second buffer: {} samples ({:.2} MB)",
                duration.as_secs(),
                buffer_size,
                memory_mb
            );

            let ring = HeapRb::<f32>::new(buffer_size);
            let (producer, consumer) = ring.split();
            producers.push((duration, producer));
            consumers.push((duration, consumer));
        }

        let running = Arc::new(AtomicBool::new(true));

        // Build the audio stream
        let stream = Self::build_stream(
            &device,
            &config.into(),
            sample_format,
            producers,
            running.clone(),
        )?;

        stream.play()?;

        Ok(Self {
            format,
            _stream: stream,
            consumers,
            running,
        })
    }

    /// Build input stream that feeds all producers
    fn build_stream(
        device: &Device,
        config: &StreamConfig,
        sample_format: SampleFormat,
        producers: Vec<(BufferDuration, HeapProd<f32>)>,
        running: Arc<AtomicBool>,
    ) -> Result<Stream, cpal::BuildStreamError> {
        match sample_format {
            SampleFormat::F32 => {
                Self::build_typed_stream::<f32>(device, config, producers, running)
            }
            SampleFormat::I16 => {
                Self::build_typed_stream::<i16>(device, config, producers, running)
            }
            SampleFormat::U16 => {
                Self::build_typed_stream::<u16>(device, config, producers, running)
            }
            _ => Err(cpal::BuildStreamError::StreamConfigNotSupported),
        }
    }

    fn build_typed_stream<T>(
        device: &Device,
        config: &StreamConfig,
        mut producers: Vec<(BufferDuration, HeapProd<f32>)>,
        running: Arc<AtomicBool>,
    ) -> Result<Stream, cpal::BuildStreamError>
    where
        T: Sample + cpal::SizedSample,
        f32: FromSample<T>,
    {
        device.build_input_stream(
            config,
            move |data: &[T], _: &cpal::InputCallbackInfo| {
                if !running.load(Ordering::Relaxed) {
                    return;
                }

                // Push each sample to ALL producers
                for &sample in data {
                    let sample_f32 = f32::from_sample(sample);
                    for (_, producer) in &mut producers {
                        // If buffer is full, oldest sample is automatically discarded
                        let _ = producer.try_push(sample_f32);
                    }
                }
            },
            |err| eprintln!("[ERROR] Audio stream error: {}", err),
            None,
        )
    }

    /// Peek at buffer contents without consuming (returns copy of samples)
    pub fn peek_buffer(&mut self, duration: BufferDuration) -> Option<Vec<f32>> {
        for (dur, consumer) in &mut self.consumers {
            if *dur == duration {
                let (head, tail) = consumer.as_slices();
                let mut samples = Vec::with_capacity(head.len() + tail.len());
                samples.extend_from_slice(head);
                samples.extend_from_slice(tail);
                return Some(samples);
            }
        }
        None
    }

    /// Check if a specific buffer duration is available
    pub fn has_buffer(&self, duration: BufferDuration) -> bool {
        self.consumers.iter().any(|(d, _)| *d == duration)
    }

    /// Pause recording
    pub fn pause(&self) {
        self.running.store(false, Ordering::Relaxed);
    }

    /// Resume recording
    pub fn resume(&self) {
        self.running.store(true, Ordering::Relaxed);
    }

    /// Check if currently recording
    pub fn is_recording(&self) -> bool {
        self.running.load(Ordering::Relaxed)
    }
}
