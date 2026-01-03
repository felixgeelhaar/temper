# Contributing

We welcome contributions to Temper!

## Getting Started

1. Fork the repository
2. Clone your fork
3. Install dependencies:
   ```bash
   go mod download
   cd web && npm install
   ```

## Development

```bash
# Run backend
go run ./cmd/temper start

# Run frontend (separate terminal)
cd web && npm run dev

# Run tests
go test ./...
```

## Code Style

- Go: Follow standard Go conventions, run `gofmt`
- TypeScript: Use Prettier, ESLint
- Commits: Use conventional commits

## Pull Requests

1. Create a feature branch
2. Make your changes
3. Add tests
4. Update documentation
5. Submit PR

## Areas for Contribution

- New exercises
- Language support
- Editor integrations
- Documentation
- Bug fixes
