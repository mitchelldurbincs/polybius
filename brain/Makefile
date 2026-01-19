# brain/Makefile
.PHONY: build run-brain run-gym test clean

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
