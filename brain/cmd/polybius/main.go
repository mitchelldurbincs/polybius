// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mitchelldurbin/polybius/brain/internal/brain"
	"github.com/mitchelldurbin/polybius/brain/internal/gym"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
	"github.com/mitchelldurbin/polybius/brain/internal/vocab"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym|vocab>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "brain":
		runBrain()
	case "gym":
		runGym()
	case "vocab":
		runVocab()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(envVar, defaultVal string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultVal
}

func runBrain() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot determine home directory: %v", err)
	}

	// Allow environment variable overrides for paths
	minerDir := getEnvOrDefault("POLYBIUS_MINER_DIR", filepath.Join(homeDir, "Music", "Miner"))
	dbPath := getEnvOrDefault("POLYBIUS_DB", filepath.Join(homeDir, ".polybius", "brain.db"))
	cedictPath := getEnvOrDefault("POLYBIUS_CEDICT", filepath.Join(homeDir, ".polybius", "cedict_ts.u8"))

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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot determine home directory: %v", err)
	}
	dbPath := getEnvOrDefault("POLYBIUS_DB", filepath.Join(homeDir, ".polybius", "brain.db"))

	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check for drafts first - show triage if any exist
	draftCount, err := db.CountDraftCards()
	if err != nil {
		log.Printf("Warning: failed to count draft cards: %v", err)
		draftCount = 0
	}
	if draftCount > 0 {
		triageModel := gym.NewTriageModel(db)
		p := tea.NewProgram(triageModel)

		finalModel, err := p.Run()
		if err != nil {
			log.Fatalf("Error running triage: %v", err)
		}

		// Check if user wants to continue to review
		if tm, ok := finalModel.(gym.TriageModel); ok && !tm.StartReview() {
			return // User quit without wanting review
		}
	}

	// Start review session
	session := gym.NewSession(db)
	cards, err := session.GetDueCards(20)
	if err != nil {
		log.Fatalf("Failed to get cards: %v", err)
	}

	if len(cards) == 0 {
		fmt.Println("No cards due for review. Great job!")
		return
	}

	// Create rating callback
	onRate := func(cardID int64, rating int) error {
		return session.SubmitRating(cardID, rating)
	}

	model := gym.NewModel(cards, onRate)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

func runVocab() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: polybius vocab <import> [args]")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "import":
		runVocabImport()
	default:
		fmt.Printf("Unknown vocab command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func runVocabImport() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: polybius vocab import <file.tsv>")
		os.Exit(1)
	}

	filePath := os.Args[3]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot determine home directory: %v", err)
	}
	dbPath := getEnvOrDefault("POLYBIUS_DB", filepath.Join(homeDir, ".polybius", "brain.db"))

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	result, err := vocab.ImportTSV(db, filePath)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Printf("\nImported %d words (%d new, %d already known)\n",
		result.Added+result.Skipped, result.Added, result.Skipped)
}
