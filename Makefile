# Marko — terminal Markdown editor
#
# Common targets:
#   make            build ./marko in the project dir
#   make install    build and install to ~/.local/bin/marko (on your PATH)
#   make test       run the test suite
#   make run        build and launch on a sample doc
#   make clean      remove build artifacts

BINARY   := marko
SRC      := main.go main_test.go
BINDIR   := $(HOME)/.local/bin

GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: all build install test run clean fmt vet

all: build

# Build the binary into the project directory.
build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Build and install as ~/.local/bin/marko (the name you run it as).
install: build
	@mkdir -p $(BINDIR)
	@cp $(BINARY) $(BINDIR)/$(BINARY)
	@chmod +x $(BINDIR)/$(BINARY)
	@echo "Installed $$ $(BINDIR)/$(BINARY)"

# Quick checks before shipping.
test:
	go vet ./...
	go test ./...

run: build
	./$(BINARY)

fmt:
	gofmt -w $(SRC)

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
