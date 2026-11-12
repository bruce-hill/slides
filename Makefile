PREFIX=${HOME}/.local

all: slides

slides: $(shell find . -name '*.go') go.mod go.sum
	go build -o slides ./cmd/slides

test:
	go vet ./...
	go test ./...

install: slides
	install -d $(PREFIX)/bin
	install -m 755 slides $(PREFIX)/bin/slides

clean:
	rm -f slides

.PHONY: all test install clean
