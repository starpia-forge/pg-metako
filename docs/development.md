# Development Guide

This guide covers development setup, project structure, and development workflows for pg-metako.

## Project Structure

The project follows the [golang-standards/project-layout](https://github.com/golang-standards/project-layout):

```
├── cmd/pg-metako/          # Main application
├── internal/               # Private application packages
│   ├── config/            # Configuration management
│   ├── database/          # Database connection handling
│   ├── health/            # Health checking
│   ├── metako/            # Main application orchestration
│   ├── replication/       # Replication and failover
│   └── routing/           # Query routing and load balancing
├── configs/               # Configuration examples
├── docs/                  # Documentation
├── bin/                   # Compiled binaries
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
└── README.md              # Project documentation
```

## Prerequisites

- Go 1.24 or later
- Docker and Docker Compose (for containerized development)
- PostgreSQL 12+ with replication configured (for local development)

## Development Setup

### Local Development

1. Clone the repository:
```bash
git clone <repository-url>
cd pg-metako
```

2. Build the application:
```bash
# Using Makefile (recommended)
make build

# Or using Go directly
go build -o bin/pg-metako ./cmd/pg-metako
```

3. Run with example configuration:
```bash
./bin/pg-metako --config configs/config.yaml
```

## Makefile

The project includes a comprehensive Makefile for build automation. View all available targets:

```bash
make help
```

### Common Commands

**Build and Test:**
```bash
make build          # Build the application
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make check          # Run format, vet, and tests
```

**Development:**
```bash
make setup          # Setup development environment
make dev-setup      # Setup with additional development tools
make fmt            # Format Go code
make vet            # Run go vet
make lint           # Run linter (requires golangci-lint)
```

**Docker:**
```bash
make docker-build   # Build Docker image
make docker-run     # Run Docker container
make docker-compose-up    # Start services with Docker Compose
make docker-compose-down  # Stop services with Docker Compose
```

**Cross-platform Builds:**
```bash
make build-all      # Build for all platforms
make build-linux    # Build for Linux
make build-windows  # Build for Windows
make build-darwin   # Build for macOS
```

**Release and Cleanup:**
```bash
make release        # Create release build
make clean          # Clean build artifacts
make ci             # Run CI pipeline
```

## Testing

### Running Tests

Run all tests:
```bash
# Using Makefile (recommended)
make test

# Or using Go directly
go test ./...
```

Run tests with verbose output:
```bash
# Using Makefile
make test-verbose

# Or using Go directly
go test -v ./...
```

Run tests for a specific package:
```bash
go test -v ./internal/config
```

### Test Coverage

Generate test coverage report:
```bash
make test-coverage
```

This will generate a coverage report and open it in your browser.

### Writing Tests

Follow Go testing conventions:
- Test files should end with `_test.go`
- Test functions should start with `Test`
- Use table-driven tests for multiple test cases
- Mock external dependencies

Example test structure:
```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
    }{
        {
            name: "valid config",
            config: Config{
                // valid config here
            },
            wantErr: false,
        },
        // more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Code Quality

### Formatting

Format code using:
```bash
make fmt
```

### Linting

Run linter (requires golangci-lint):
```bash
make lint
```

Install golangci-lint:
```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
```

### Static Analysis

Run static analysis:
```bash
make vet
```

## Development Workflow

### TDD Approach

Follow Test-Driven Development:
1. Write a failing test
2. Write minimal code to make it pass
3. Refactor while keeping tests green

### Git Workflow

1. Create feature branch from main
2. Make changes with descriptive commits
3. Run tests and linting before committing
4. Create pull request with clear description

### Commit Messages

Use conventional commit format:
```
type(scope): description

[optional body]

[optional footer]
```

Examples:
- `feat(config): add distributed configuration support`
- `fix(health): resolve connection timeout issue`
- `docs(readme): update installation instructions`

## Debugging

### Local Debugging

Use Go's built-in debugging tools:
```bash
# Run with race detection
go run -race ./cmd/pg-metako --config configs/config.yaml

# Build with debug symbols
go build -gcflags="all=-N -l" -o bin/pg-metako-debug ./cmd/pg-metako
```

### Using Delve Debugger

Install and use Delve:
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug the application
dlv debug ./cmd/pg-metako -- --config configs/config.yaml
```

### Logging

The application uses structured JSON logging. Set log level:
```bash
export LOG_LEVEL=debug
./bin/pg-metako --config configs/config.yaml
```

## Performance Testing

### Benchmarking

Run benchmarks:
```bash
go test -bench=. ./...
```

### Profiling

Generate CPU profile:
```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/routing
go tool pprof cpu.prof
```

Generate memory profile:
```bash
go test -memprofile=mem.prof -bench=. ./internal/routing
go tool pprof mem.prof
```

## Contributing

### Code Review Checklist

- [ ] Tests pass locally
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated
- [ ] Commit messages follow convention
- [ ] No sensitive information in code

### Pull Request Process

1. Fork the repository
2. Create feature branch
3. Make changes with tests
4. Update documentation
5. Submit pull request
6. Address review feedback

## IDE Setup

### VS Code

Recommended extensions:
- Go extension by Google
- golangci-lint extension
- Test Explorer for Go

### GoLand/IntelliJ

Configure Go SDK and enable:
- Go modules support
- Code inspections
- Test runner integration