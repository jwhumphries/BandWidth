# List available commands
default:
    @just --list

# Start the local dev loop (Go API :8080 + Vite :3000)
dev:
    ./scripts/develop.sh

# Run all checks in parallel inside one Dagger session
check:
    dagger call check --source .

# Go linting (golangci-lint)
lint-go:
    dagger call lint-go --source .

# ESLint
lint-js:
    dagger call lint-js --source .

# TypeScript type checking
typecheck:
    dagger call typecheck --source .

# Go tests
test:
    dagger call test-go --source .

# Frontend tests (Vitest)
test-frontend:
    dagger call test-frontend --source .

# Prettier check
format-check:
    dagger call format-check --source .

# Format Go code (goimports)
fmt:
    dagger call fmt --source . export --path .

# Format frontend code (Prettier)
format:
    dagger call format --source . export --path frontend

# Build the frontend dist
build-frontend:
    dagger call build-frontend --source . export --path frontend/dist

# Build the production container image as a tarball
build version=`git rev-parse --short HEAD`:
    mkdir -p tmp
    dagger call release --source . --version {{version}} export --path tmp/bandwidth-image.tar

# Remove build artifacts
clean:
    rm -rf tmp frontend/dist frontend/node_modules
