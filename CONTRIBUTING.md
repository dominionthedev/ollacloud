# Contributing to ollacloud

Thanks for your interest. This is a small, focused tool — contributions that keep it that way are most welcome.

## What we want

- Bug fixes with a failing test that proves the fix
- New Ollama API endpoints as Ollama Cloud exposes them
- Better TUI polish (run command experience)
- Tooling and Agentic system
- Documentation improvements

## What we don't want (yet)

- New config formats / persistence backends
- Support for non-Ollama-Cloud upstreams (future roadmap item)
- Heavy framework dependencies

## Setup

```sh
git clone https://github.com/dominionthedev/ollacloud
cd ollacloud
go mod tidy
go build -o bin/ollacloud .
go test ./...
```

Requires Go 1.22+.

## Making changes

1. Fork the repo and create a branch: `git checkout -b fix/my-fix`
2. Make your change. If it touches logic, add or update a test.
3. Run `go test ./... -race` — all tests must pass.
4. Run `go vet ./...` — no vet errors.
5. Open a PR with a clear description of what and why.

## Commit style

```
type: short description

Longer explanation if needed.
```

Types: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`.

## Code style

- Standard `gofmt` formatting
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- Context passed as first argument to all I/O functions
- No global state outside the `env` package

## Testing

Every PR must pass CI (tests on Linux, macOS, Windows + race detector). The proxy and streaming packages have integration tests that use `httptest` — add to those when touching network code.

