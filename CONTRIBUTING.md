# Contributing to Bud

Thank you for your interest in contributing! This document provides guidelines for contributing to the project.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When creating a bug report, include:

- **Clear title and description**
- **Steps to reproduce** the issue
- **Expected behavior** vs actual behavior
- **Environment details** (OS, Go version, AWS region)
- **Relevant logs or error messages**

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

- **Clear title and description**
- **Use case** - why is this enhancement needed?
- **Proposed solution** (if you have one)
- **Alternative solutions** you've considered

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Make your changes** following the code style guidelines
3. **Add tests** if you've added code that should be tested
4. **Ensure tests pass** with `go test ./...`
5. **Update documentation** if needed
6. **Write a clear commit message**

## Development Setup

### Prerequisites

- Go 1.23 or higher
- AWS credentials configured
- Access to an AWS Organization (for testing)

### Local Development

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/Bud.git
cd Bud

# Install dependencies
go mod download

# Build
go build -o Bud ./cmd/bud

# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Format code
go fmt ./...

# Vet code
go vet ./...
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/analyzer

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Code Style Guidelines

### Go Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` to format your code
- Run `go vet` to catch common mistakes
- Keep functions small and focused
- Write clear, descriptive variable names
- Add comments for exported functions and types

### Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit first line to 72 characters
- Reference issues and pull requests when relevant

Example:
```
Add support for tag-based policies

- Implement tag policy resolution
- Add tests for tag matching
- Update documentation

Fixes #123
```

### Testing

- Write unit tests for new functionality
- Maintain or improve test coverage
- Use table-driven tests where appropriate
- Mock external dependencies (AWS APIs)

Example test structure:
```go
func TestAnalyzer_AnalyzeAccount(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    float64
        wantErr bool
    }{
        {
            name:    "valid account",
            input:   "123456789012",
            want:    100.50,
            wantErr: false,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Project Structure

```
Bud/
├── cmd/
│   └── bud/    # CLI entry point
├── internal/
│   ├── analyzer/                # Spending analysis
│   ├── budgets/                 # AWS Budgets client
│   ├── cmd/                     # Cobra commands
│   ├── costexplorer/            # Cost Explorer client
│   ├── policy/                  # Policy resolution
│   ├── recommender/             # Recommendation engine
│   └── reporter/                # Report generation
├── pkg/
│   └── types/                   # Public shared types
├── docs/                        # Documentation
└── examples/                    # Example configurations
```

## Documentation

- Update README.md for user-facing changes
- Add godoc comments for exported functions
- Update CHANGELOG.md following [Keep a Changelog](https://keepachangelog.com/)
- Add examples for new features

## Release Process

Releases are automated via GitHub Actions and goreleaser:

1. Update CHANGELOG.md
2. Create and push a version tag: `git tag v1.2.3 && git push origin v1.2.3`
3. GitHub Actions will build and publish the release

## Questions?

Feel free to open an issue for questions or discussions about contributing.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
