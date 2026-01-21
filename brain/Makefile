# brain/Makefile
.PHONY: build run-brain run-gym test clean reset reset-all

build:
	go build -o bin/polybius ./cmd/polybius

run-brain: build
	./bin/polybius brain

run-gym: build
	./bin/polybius gym

test:
	go test ./...

clean:
	rm -rf bin/

reset:
ifeq ($(OS),Windows_NT)
	powershell -Command "Remove-Item -Force -ErrorAction SilentlyContinue \"$$env:USERPROFILE\\.polybius\\brain.db\""
else
	rm -f ~/.polybius/brain.db
endif
	@echo "Database reset. Run 'make run-brain' to create a fresh one."

reset-all: reset
ifeq ($(OS),Windows_NT)
	powershell -Command "Remove-Item -Force -ErrorAction SilentlyContinue \"$$env:USERPROFILE\\Music\\Miner\\*\""
else
	rm -f ~/Music/Miner/*
endif
	@echo "All captures cleared."
