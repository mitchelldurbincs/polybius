// brain/internal/gym/imagewin.go
package gym

import (
	"os/exec"
	"runtime"
)

type ImageWindow struct{}

func NewImageWindow() *ImageWindow {
	return &ImageWindow{}
}

// Show opens the image in the system default viewer
// This is a simple approach - for better UX, could use a dedicated window
func (w *ImageWindow) Show(imagePath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", imagePath)
	case "darwin":
		cmd = exec.Command("open", imagePath)
	default: // linux
		cmd = exec.Command("xdg-open", imagePath)
	}

	return cmd.Start()
}
