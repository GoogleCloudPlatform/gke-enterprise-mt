.PHONY: help build test clean tidy fmt lint

# Build targets
help:
	@echo "Available targets:"
	@echo "  build          - Build the project"
	@echo "  test           - Run all tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  tidy           - Tidy Go modules"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linters"
	@echo "  vet            - Run go vet"
	@echo "  generate       - Run go generate"

build:
	go build ./...

test:
	go test -v -race ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	go clean
	rm -f coverage.out coverage.html

tidy:
	go mod tidy

fmt:
	gofmt -w -s .
	go mod tidy

lint:
	golangci-lint run ./...

vet:
	go vet ./...

generate:
	go generate ./...
