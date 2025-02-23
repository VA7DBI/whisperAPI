# Contributing to WhisperAPI

Thank you for your interest in contributing to WhisperAPI! This guide will help you understand our development process and standards.

## Communication

- All project discussions, bug reports, and feature requests should be done through GitHub Issues
- Please search existing issues before creating new ones
- Use appropriate issue templates when available

## Project Structure

```
whisperAPI/
├── api/            # API handlers and route definitions
├── cmd/            # Application entry points
├── config/         # Configuration management
├── docs/           # Documentation and Swagger files
├── internal/       # Internal packages
│   ├── models/     # Data models
│   └── services/   # Business logic services
├── pkg/            # Public packages that can be imported
└── tests/          # Test files
```

## Development Requirements

### Prerequisites

- Go 1.19 or higher
- `swag` CLI tool for Swagger documentation
- Docker (for containerization)

### Setting Up Development Environment

1. Fork and clone the repository
2. Install dependencies: `go mod download`
3. Install swag: `go install github.com/swaggo/swag/cmd/swag@latest`

## Building and Testing

### Building the Project

```bash
# Generate Swagger docs
swag init -g cmd/api/main.go

# Build the application
go build -o whisperapi ./cmd/api
```

### Running Tests

- All new code must include unit tests
- Test files should be named `*_test.go`
- Run tests: `go test ./...`
- Aim for at least 80% test coverage: `go test -cover ./...`

## Documentation Standards

### Code Documentation

- All exported functions, types, and constants must have proper GoDoc comments
- Include examples in documentation where appropriate
- Keep comments clear and concise

### API Documentation

- Use Swagger annotations in your code
- Update API documentation using swag:
  ```bash
  swag init -g cmd/api/main.go
  ```
- Ensure all endpoints are properly documented with:
  - Description
  - Request/Response schemas
  - Example requests
  - Possible error responses

## Pull Request Process

1. Create a new branch for your feature/fix
2. Write tests for new functionality
3. Update documentation as needed
4. Run `swag init` if API changes were made
5. Ensure all tests pass
6. Create a Pull Request with a clear description
7. Wait for code review

## Code Style

- Follow standard Go formatting guidelines
- Run `go fmt` before committing
- Use `golint` and `go vet` to check your code

## Release Process

1. Version numbers follow [SemVer](https://semver.org/)
2. Changes are documented in CHANGELOG.md
3. Releases are tagged in Git and published on GitHub

## Questions or Problems?

- Open an issue in the GitHub repository
- Check existing issues and documentation first
- Provide as much context as possible

## License

By contributing to this project, you agree that your contributions will be licensed under its LICENSE file terms.