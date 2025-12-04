# LetsGoIntunePackager Makefile

BINARY_NAME=letsgointunepackager
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet

# Build targets
.PHONY: all build build-all clean test test-coverage lint vet fmt deps help winres

all: clean deps build

# Build for current platform
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

# Generate Windows resources (icon, manifest, version info)
winres:
	@which go-winres > /dev/null || (echo "Installing go-winres..." && go install github.com/tc-hib/go-winres@latest)
	@if [ -f winres/icon.png ]; then \
		echo "Generating Windows resources..."; \
		$(shell go env GOPATH)/bin/go-winres make --in winres/winres.json --out rsrc_windows; \
	else \
		echo "Warning: winres/icon.png not found. Skipping resource generation."; \
		echo "Add icon.png (256x256 recommended) to winres/ folder and run 'make winres' again."; \
	fi

# Build for all platforms (with Windows icon if available)
build-all: clean winres
	@mkdir -p dist
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .
	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Building for Windows (arm64)..."
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-arm64.exe .
	@echo "All builds complete!"

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run go vet
vet:
	$(GOVET) ./...

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME).exe
	rm -rf dist/
	rm -f coverage.out coverage.html
	rm -f rsrc_windows_*.syso

# Install binary to GOPATH/bin
install: build
	mv $(BINARY_NAME) $(GOPATH)/bin/

# Run the application
run: build
	./$(BINARY_NAME)

# Run in quiet mode with test data
run-quiet: build
	./$(BINARY_NAME) -c ./testdata/source -s setup.exe -o ./testdata/output -q

# Show help
help:
	@echo "LetsGoIntunePackager - Create .intunewin packages for Microsoft Intune"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build the application"
	@echo "  make build        Build for current platform"
	@echo "  make build-all    Build for all platforms (Linux, macOS, Windows)"
	@echo "  make winres       Generate Windows resources (icon, manifest)"
	@echo "  make test         Run tests"
	@echo "  make test-coverage Run tests with coverage report"
	@echo "  make vet          Run go vet"
	@echo "  make fmt          Format code"
	@echo "  make lint         Run linter (requires golangci-lint)"
	@echo "  make deps         Download dependencies"
	@echo "  make clean        Clean build artifacts"
	@echo "  make install      Install binary to GOPATH/bin"
	@echo "  make run          Build and run the application"
	@echo "  make help         Show this help message"
	@echo ""
	@echo "To add an icon to Windows builds:"
	@echo "  1. Add icon.png (256x256) to winres/ folder"
	@echo "  2. Run 'make winres' to generate resources"
	@echo "  3. Run 'make build-all' to build with icon"
