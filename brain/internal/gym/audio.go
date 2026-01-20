// brain/internal/gym/audio.go
package gym

import (
	"os"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

type AudioPlayer struct {
	mu          sync.Mutex
	initialized bool
	sampleRate  beep.SampleRate

	// Playback control
	playing  bool
	stopChan chan struct{}

	// Error state for UI to read
	lastError error
}

func NewAudioPlayer() *AudioPlayer {
	return &AudioPlayer{}
}

// Play stops any current playback and starts playing the given file.
// This method is non-blocking - playback happens in a goroutine.
func (p *AudioPlayer) Play(filePath string) {
	p.mu.Lock()

	// Stop any current playback
	if p.playing && p.stopChan != nil {
		close(p.stopChan)
	}

	// Reset state for new playback
	p.playing = true
	p.stopChan = make(chan struct{})
	p.lastError = nil
	stopChan := p.stopChan // Local copy for goroutine

	p.mu.Unlock()

	go p.playInternal(filePath, stopChan)
}

func (p *AudioPlayer) playInternal(filePath string, stopChan chan struct{}) {
	defer func() {
		p.mu.Lock()
		// Only clear playing if this is still the active playback
		if p.stopChan == stopChan {
			p.playing = false
		}
		p.mu.Unlock()
	}()

	// Open file
	f, err := os.Open(filePath)
	if err != nil {
		p.mu.Lock()
		p.lastError = err
		p.mu.Unlock()
		return
	}
	defer f.Close()

	// Decode WAV
	streamer, format, err := wav.Decode(f)
	if err != nil {
		p.mu.Lock()
		p.lastError = err
		p.mu.Unlock()
		return
	}
	defer streamer.Close()

	// Initialize speaker once
	p.mu.Lock()
	if !p.initialized {
		err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		if err != nil {
			p.lastError = err
			p.mu.Unlock()
			return
		}
		p.initialized = true
		p.sampleRate = format.SampleRate
	}
	sampleRate := p.sampleRate
	p.mu.Unlock()

	// Resample if needed
	var toPlay beep.Streamer = streamer
	if format.SampleRate != sampleRate {
		toPlay = beep.Resample(4, format.SampleRate, sampleRate, streamer)
	}

	// Play with cancellation support
	done := make(chan struct{})
	speaker.Play(beep.Seq(toPlay, beep.Callback(func() {
		close(done)
	})))

	// Wait for either completion or stop signal
	select {
	case <-done:
		// Finished naturally
	case <-stopChan:
		speaker.Clear() // Stop all playing audio
	}
}

// Stop halts any current playback. Safe to call multiple times.
func (p *AudioPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playing && p.stopChan != nil {
		close(p.stopChan)
		p.stopChan = nil
		p.playing = false
	}
}

// Close stops playback and releases the speaker resource.
func (p *AudioPlayer) Close() {
	p.Stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		speaker.Close()
		p.initialized = false
	}
}

// HasError returns true if the last play attempt failed.
func (p *AudioPlayer) HasError() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastError != nil
}
