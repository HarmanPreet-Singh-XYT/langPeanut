.PHONY: all build install test clean web tui reset benchmark

all: build

build:
	@mkdir -p bin
	go build -ldflags="-s -w" -o bin/langPeanut ./cmd/langPeanut
	@ln -sf bin/langPeanut langPeanut
	@echo "Built binary: bin/langPeanut"

install:
	@./install.sh

uninstall:
	@./uninstall.sh

test:
	go test -v ./...

benchmark:
	go run ./cmd/langPeanut benchmark

web:
	go run ./cmd/langPeanut web

tui:
	go run ./cmd/langPeanut

reset:
	@./scripts/reset_examples.sh

clean:
	rm -rf bin/ langPeanut
