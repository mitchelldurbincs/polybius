// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mitchelldurbin/polybius/brain/internal/brain"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "brain":
		runBrain()
	case "gym":
		runGym()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBrain() {
	// Default paths - should come from config
	homeDir, _ := os.UserHomeDir()
	minerDir := filepath.Join(homeDir, "Music", "Miner")
	dbPath := filepath.Join(homeDir, ".polybius", "brain.db")
	cedictPath := filepath.Join(homeDir, ".polybius", "cedict_ts.u8")

	// Ensure directories exist
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	cfg := brain.Config{
		DBPath:     dbPath,
		CEDICTPath: cedictPath,
		MinerDir:   minerDir,
	}

	svc, err := brain.NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to start Brain: %v", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go svc.Start()

	fmt.Println("The Brain is running. Press Ctrl+C to stop.")
	<-sigChan

	fmt.Println("\nShutting down...")
	svc.Stop()
}

func runGym() {
	fmt.Println("The Gym - Coming soon!")
	// TODO: Implement TUI
}
