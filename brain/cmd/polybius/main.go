// brain/cmd/polybius/main.go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: polybius <brain|gym>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "brain":
		fmt.Println("Starting The Brain...")
	case "gym":
		fmt.Println("Starting The Gym...")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
