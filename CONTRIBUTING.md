# Contributing to HashGrid

First off, thank you for considering contributing to HashGrid! It's people like you that make it a great tool.

## Development Environment Setup

### Prerequisites
- **Go** (v1.25+)
- **Node.js** (v20+) or **Bun** (latest)
- **Docker** & **Docker Compose**
- **Buf CLI** (for Protobuf generation)

### Initial Setup
1. Fork and clone the repository.
2. Install infrastructure:
   ```bash
   make up
   ```
3. Generate Protobuf code:
   ```bash
   make gen
   ```
4. Install Node.js dependencies:
   ```bash
   make install-node
   ```

## Development Workflow

### 1. Branching
Create a branch for your changes:
```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### 2. Protocol Changes
If you modify `proto/spatial/v1/spatial.proto`, you **must** run:
```bash
make gen
```
This ensures both Go and TypeScript code remains in sync.

### 3. Coding Standards
We enforce strict linting and formatting rules. Before committing, ensure everything is clean:
```bash
make lint
```
- **Go**: We use `golangci-lint` (v2.x) and `gofmt`.
- **TypeScript**: We use `tsc` for type checking.

### 4. Testing
Always add tests for new features or bug fixes. Run existing tests to ensure no regressions:
```bash
make test
```
If you change core spatial logic, please run benchmarks to check for performance regressions:
```bash
make bench-go
```

## Pull Request Process

1. Ensure CI passes on your branch (GitHub Actions).
2. Update the `README.md` if you changed any user-facing features or configuration.
3. Your PR should ideally include:
   - A clear description of the change.
   - Any new tests added.
   - Benchmark results if performance was affected.

## Project Structure

- `/go-worker`: Go Coprocessor (Spatial Engine).
- `/node-plane`: Node.js Master (Simulation & Logic).
- `/proto`: Protocol Buffer definitions.
- `Makefile`: Central entry point for all development tasks.

## Code of Conduct
By contributing to this project, you agree to maintain a professional and respectful environment for all contributors.

Thank you for your contribution!
