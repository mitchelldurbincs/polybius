package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mitchelldurbin/polybius/brain/internal/skritter"
)

func skritterService() (*skritter.Service, error) {
	token, err := skritter.LoadToken()
	if err != nil {
		return nil, err
	}
	return skritter.NewService(skritter.NewClient(token)), nil
}
func skritterWordAdder() func(context.Context, string, string) (string, error) {
	service, err := skritterService()
	return func(ctx context.Context, word, reading string) (string, error) {
		if err != nil {
			return "", err
		}
		result, err := service.AddWords(ctx, []skritter.Word{{Writing: word, Reading: reading}})
		return result.String(), err
	}
}
func runSkritter(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: polybius skritter <init|add> [--pinyin reading] [words...]")
	}
	if args[0] != "init" && args[0] != "add" {
		return fmt.Errorf("unknown Skritter command: %s", args[0])
	}
	flags := flag.NewFlagSet("skritter "+args[0], flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	reading := flags.String("pinyin", "", "pinyin for a single word (e.g. xue2 xi2)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if args[0] == "init" && (flags.NArg() != 0 || *reading != "") {
		return fmt.Errorf("usage: polybius skritter init")
	}
	if args[0] == "add" && flags.NArg() == 0 {
		return fmt.Errorf("provide at least one word")
	}
	if *reading != "" && flags.NArg() != 1 {
		return fmt.Errorf("--pinyin requires exactly one word")
	}
	service, err := skritterService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if args[0] == "init" {
		list, err := service.EnsureList(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("Polybius list ready: https://skritter.com/vocablists/view/%s\n", list.ID)
		return nil
	}
	words := make([]skritter.Word, 0, flags.NArg())
	for _, word := range flags.Args() {
		words = append(words, skritter.Word{Writing: word, Reading: *reading})
	}
	result, err := service.AddWords(ctx, words)
	if err != nil {
		return err
	}
	fmt.Println(result.String())
	return nil
}
