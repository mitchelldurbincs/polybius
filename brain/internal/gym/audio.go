// brain/internal/gym/audio.go
package gym

import (
	"os"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

type AudioPlayer struct {
	initialized bool
	sampleRate  beep.SampleRate
}

func NewAudioPlayer() *AudioPlayer {
	return &AudioPlayer{}
}

func (p *AudioPlayer) Play(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	streamer, format, err := wav.Decode(f)
	if err != nil {
		f.Close()
		return err
	}

	if !p.initialized {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		p.initialized = true
		p.sampleRate = format.SampleRate
	}

	// Resample if needed
	var toPlay beep.Streamer = streamer
	if format.SampleRate != p.sampleRate {
		toPlay = beep.Resample(4, format.SampleRate, p.sampleRate, streamer)
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(toPlay, beep.Callback(func() {
		done <- true
	})))

	<-done
	streamer.Close()
	f.Close()

	return nil
}

// PlayAsync plays audio without blocking
func (p *AudioPlayer) PlayAsync(filePath string) {
	go p.Play(filePath)
}
