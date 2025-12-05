# Contributing to Vault File Encryption

Thank you for considering contributing to this project! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include:

- **Clear title and description**
- **Steps to reproduce** the behavior
- **Expected behavior**
- **Actual behavior**
- **Environment details** (OS, Go version, Vault version)
- **Relevant logs** or error messages

### Suggesting Enhancements

Enhancement suggestions are welcome! Please include:

- **Clear title and description**
- **Use case** and motivation
- **Possible implementation** (if you have ideas)
- **Alternative solutions** you've considered

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run all validation checks (`make validate-all`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to your branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.25.0 or later
- Make
- golangci-lint
- gosec
- staticcheck

### Setup

```bash
# Unix/Linux/macOS
# Clone your fork
git clone https://github.com/YOUR_USERNAME/vault-file-encryption.git
cd vault-file-encryption

# Add upstream remote
git remote add upstream https://github.com/gitrgoliveira/vault-file-encryption.git

# Install dependencies
make deps

# Install linting tools
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

```powershell
# Windows (using Git Bash or WSL for Make, or use go commands directly)
# Clone your fork
git clone https://github.com/YOUR_USERNAME/vault-file-encryption.git
cd vault-file-encryption

# Add upstream remote
git remote add upstream https://github.com/gitrgoliveira/vault-file-encryption.git

# Install dependencies
go mod download
go mod tidy

# Install linting tools
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/my-feature
```

### 2. Make Changes

Follow the existing code style and conventions:

- Use structured logging with key-value pairs
- Add comments for exported functions and types
- Keep functions focused and single-purpose
- Use meaningful variable names

### 3. Write Tests

- Add unit tests for new functionality
- Ensure existing tests still pass
- Aim for 80%+ code coverage
- Use table-driven tests when appropriate

Example test structure:

```go
func TestMyFunction(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		shouldErr bool
	}{
		{
			name:      "valid input",
			input:     "test",
			expected:  "TEST",
			shouldErr: false,
		},
		// Add more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MyFunction(tt.input)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
```

### 4. Run Validation

Before committing, run all validation checks:

```bash
# Run all checks
make validate-all

# This runs:
# - Code formatting check
# - go vet
# - staticcheck
# - golangci-lint
# - gosec security scan
# - All tests with race detector
```

### 5. Fix Issues

If validation fails:

```bash
# Fix formatting
make fmt

# Address specific issues
make vet
make staticcheck
make lint
make gosec

# Re-run validation
make validate-all
```

### 6. Commit Changes

Use clear, descriptive commit messages:

```
Add feature to support custom encryption algorithms

- Implemented AES-256-GCM encryption
- Added tests for new encryption methods
- Updated documentation

Fixes #123
```

### 7. Push and Create PR

```bash
git push origin feature/my-feature
```

Then open a Pull Request on GitHub.

## Code Style Guidelines

### General Go Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting (enforced by CI)
- Keep line length reasonable (preferably under 100 characters)
- Use early returns to reduce nesting

### Project-Specific Conventions

#### Logging

Always use structured logging:

```go
// Good
log.Info("processing file", "path", filePath, "size", fileSize)

// Bad
log.Info("Processing file: " + filePath + " size: " + strconv.Itoa(fileSize))
```

#### Error Handling

Wrap errors with context:

```go
// Good
if err != nil {
	return fmt.Errorf("failed to encrypt file: %w", err)
}

// Bad
if err != nil {
	return err
}
```

#### Memory Security

Always zero sensitive data and lock memory when handling keys:

```go
// Get plaintext key from Vault
plaintextKey, err := dataKey.PlaintextBytes()
if err != nil {
	return fmt.Errorf("failed to decode key: %w", err)
}

// Lock key in memory to prevent swapping to disk (best effort)
unlock, _ := crypto.LockMemory(plaintextKey)
defer unlock()

// Ensure the key is zeroed when done (constant-time operation)
defer crypto.SecureZero(plaintextKey)

// Use the key for encryption/decryption...
```

**Important Security Patterns**:
- Always use `defer` immediately after obtaining sensitive data
- Use `crypto.SecureZero()` for constant-time zeroing (prevents compiler optimization)
- Use `crypto.LockMemory()` to prevent keys from being swapped to disk
- Never store plaintext keys on disk
- Zero buffers containing sensitive data before returning

#### Configuration

Use HCL tags for configuration structs:

```go
type Config struct {
	Timeout string `hcl:"timeout"`
	MaxRetries int `hcl:"max_retries"`
}
```

## Testing Guidelines

### Unit Tests

- Place tests in `*_test.go` files alongside the code
- Use `testify/assert` and `testify/require`
- Test happy paths and error cases
- Use table-driven tests for multiple scenarios

### Integration Tests

- Place in `test/integration/`
- Use build tag `//go:build integration`
- Require Vault environment for execution
- Can be skipped in short mode

### Running Tests

```bash
# All tests
make test

# Integration tests (requires Vault)
make test-integration

# Coverage report
make coverage

# Specific package
go test -v ./internal/config

# Specific test
go test -v ./internal/config -run TestLoadConfig
```

## Pull Request Process

### Before Submitting

- [ ] All validation checks pass (`make validate-all`)
- [ ] Tests added for new functionality
- [ ] Documentation updated if needed
- [ ] CHANGELOG.md updated (for significant changes)
- [ ] No sensitive data in code or commits

### PR Guidelines

- **Title**: Clear, concise description of changes
- **Description**: Explain what and why
- **Link issues**: Reference related issues with `Fixes #123`
- **Keep focused**: One feature or fix per PR
- **Size**: Keep PRs reasonably sized for easier review

### Review Process

1. Automated checks run (CI/CD)
2. Code review by maintainers
3. Address feedback if requested
4. Approval from at least one maintainer
5. Squash and merge

## Security

### Reporting Security Issues

**Do not open public issues for security vulnerabilities.**

Instead, please report security issues by:
1. Opening a GitHub Security Advisory at https://github.com/gitrgoliveira/vault-file-encryption/security/advisories/new
2. Or emailing the repository owner (see GitHub profile for contact information)

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if you have one)

### Security Best Practices

- Never commit secrets or tokens
- Use `.env` for local development (git-ignored)
- Zero sensitive data from memory after use
- Validate all inputs
- Use prepared statements for any queries
- Keep dependencies up to date

## Documentation

### Code Documentation

- Add godoc comments for exported types and functions
- Include usage examples in comments
- Keep comments up to date with code changes

Example:

```go
// EncryptFile encrypts the input file using AES-256-GCM with the provided key.
// The encrypted output is written to the output file with the following format:
//   [12-byte nonce][4-byte chunk size][encrypted chunk]...
//
// Example:
//   err := EncryptFile("input.txt", "output.enc", dataKey)
func EncryptFile(inputPath, outputPath string, key []byte) error {
	// Implementation
}
```

### User Documentation

- Update README.md for user-facing changes
- Add examples for new features
- Update configuration examples if needed
- Keep architecture diagrams current

## Questions?

If you have questions about contributing:

- Check existing issues and PRs
- Review [Architecture](ARCHITECTURE.md) documentation
- Check the [Guides](guides/) for feature-specific documentation
- Open a discussion issue

## License

By contributing, you agree that your contributions will be licensed under the Mozilla Public License 2.0 (MPL-2.0).

---

Thank you for contributing to Vault File Encryption!
