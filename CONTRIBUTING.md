# Contributing to Overflow Go SDK

Thanks for your interest in contributing! This guide will help you get started.

## Development Setup

1. Fork and clone the repository

2. Ensure you have Go 1.21+ installed:

   ```bash
   go version
   ```

3. Build and verify:

   ```bash
   go build ./...
   go vet ./...
   ```

4. Run tests:

   ```bash
   go test -race ./...
   ```

## Making Changes

1. Create a branch from `main`:

   ```bash
   git checkout -b feat/my-feature
   ```

2. Make your changes

3. Ensure everything builds, passes vet, and tests pass:

   ```bash
   go build ./...
   go vet ./...
   go test -race ./...
   ```

4. Commit your changes with a descriptive message:

   ```plaintext
   feat: add support for custom transports
   fix: prevent race condition in event flush
   docs: clarify middleware setup instructions
   ```

5. Push and open a pull request

## Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```plaintext
<type>: <description>
```

**Types:** `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

## Pull Requests

- Keep PRs focused on a single change
- Update the README if you're changing public API
- Add a clear description of what changed and why
- Link any related issues

## Reporting Bugs

Please use the [bug report template](https://github.com/Jaggle-AI-HQ/overflow-go/issues/new?template=bug_report.md) and include:

- SDK version
- Go version
- Steps to reproduce
- Expected vs actual behavior

## Requesting Features

Open an issue using the [feature request template](https://github.com/Jaggle-AI-HQ/overflow-go/issues/new?template=feature_request.md) and describe the use case.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
