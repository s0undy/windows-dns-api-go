.PHONY: build run test clean fmt vet

# Binary name
BINARY_NAME=windows-dns-api-server
BINARY_WINDOWS=$(BINARY_NAME).exe

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) ./cmd/server

# Build for Windows
build-windows:
	@echo "Building $(BINARY_WINDOWS) for Windows..."
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_WINDOWS) ./cmd/server

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	go run ./cmd/server

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download

# Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "All checks passed!"
