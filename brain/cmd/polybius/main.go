// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mitchelldurbin/polybius/brain/internal/brain"
	"github.com/mitchelldurbin/polybius/brain/internal/config"
	"github.com/mitchelldurbin/polybius/brain/internal/gym"
	"github.com/mitchelldurbin/polybius/brain/internal/storage"
	"github.com/mitchelldurbin/polybius/brain/internal/vocab"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym|vocab>")
		os.Exit(1)
	}

	// Load config once at startup
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "brain":
		runBrain(cfg)
	case "gym":
		runGym(cfg)
	case "vocab":
		runVocab(cfg)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBrain(cfg *config.Config) {
	// Ensure directories exist
	os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)

	brainCfg := brain.Config{
		DBPath:     cfg.DBPath,
		CEDICTPath: cfg.CEDICTPath,
		MinerDir:   cfg.MinerDir,
	}

	svc, err := brain.NewService(brainCfg)
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

func runGym(cfg *config.Config) {
	db, err := storage.OpenDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	session := gym.NewSession(db)

	for {
		// Load stats and show home screen
		stats, err := session.LoadHomeStats()
		if err != nil {
			log.Fatalf("Failed to load stats: %v", err)
		}

		homeModel := gym.NewHomeModel(stats)
		p := tea.NewProgram(homeModel)
		finalModel, err := p.Run()
		if err != nil {
			log.Fatalf("Error running home screen: %v", err)
		}

		hm := finalModel.(gym.HomeModel)
		action := hm.Action()

		switch action {
		case gym.ActionQuit:
			return

		case gym.ActionTriage:
			runTriage(db)

		case gym.ActionReview:
			runReview(db, session)

		case gym.ActionCards:
			runCardsList(session)
		}
	}
}

func runTriage(db *storage.DB) {
	triageModel := gym.NewTriageModel(db)
	p := tea.NewProgram(triageModel)
	p.Run()
}

func runReview(db *storage.DB, session *gym.Session) {
	cards, err := session.GetDueCards(20)
	if err != nil {
		log.Printf("Failed to get cards: %v", err)
		return
	}

	if len(cards) == 0 {
		fmt.Println("No cards due for review.")
		time.Sleep(1 * time.Second)
		return
	}

	onRate := func(cardID int64, rating int) error {
		return session.SubmitRating(cardID, rating)
	}

	model := gym.NewModel(cards, onRate)
	p := tea.NewProgram(model)
	p.Run()
}

func runCardsList(session *gym.Session) {
	cards, err := session.GetAllCardsForList()
	if err != nil {
		log.Printf("Failed to get cards: %v", err)
		return
	}

	model := gym.NewCardsModel(cards)
	p := tea.NewProgram(model)
	p.Run()
}

func runVocab(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: polybius vocab <import> [args]")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "import":
		runVocabImport(cfg)
	default:
		fmt.Printf("Unknown vocab command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func runVocabImport(cfg *config.Config) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: polybius vocab import <file.tsv>")
		os.Exit(1)
	}

	filePath := os.Args[3]

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)

	db, err := storage.OpenDatabase(cfg.DBPath)
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
