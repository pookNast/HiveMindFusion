# Contributing

Contributions are welcome. Keep it minimal.

## Guidelines

1. **Follow the ponytail ladder** — before adding code, check if stdlib, a platform feature, or an existing dependency already does it.
2. **No speculative abstractions** — solve the problem at hand, nothing more.
3. **Mark intentional shortcuts** — use `# ponytail: <what> — upgrade: <path>` ceiling comments.
4. **Test at system boundaries** — integration tests over mocks where possible.
5. **No new dependencies** without justification — check `go.mod` and `requirements.txt` first.

## Process

1. Fork the repo
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## Reporting Issues

Open a GitHub issue with:
- What you expected
- What happened
- Steps to reproduce
- Config (redact any secrets)
