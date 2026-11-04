# Use bash on all platforms
# Windows users: Requires Git Bash (comes with Git for Windows: https://git-scm.com/download/win)
# Add C:\Program Files\Git\bin to your PATH, or run make from Git Bash terminal
ifeq ($(OS),Windows_NT)
    SHELL := C:/Program Files/Git/bin/bash.exe
    .SHELLFLAGS := -ec
else
    SHELL := /bin/bash
endif

# Path to the dev-tools module, relative to backend/ (see backend/build/go.mod).
BACKEND_BUILD := build

.SILENT:

.PHONY: setup update build build-docker build-docker-slim build-backend build-frontend run generate-config
.PHONY: lint-frontend lint-backend lint test test-backend test-frontend check-all setup-gofitz-cgo

setup:
	echo "creating ./backend/test_config.yaml for local testing..."
	if [ ! -f backend/test_config.yaml ]; then \
		cp backend/config.yaml backend/test_config.yaml; \
	fi
	$(MAKE) setup-gofitz-cgo
	echo "installing backend dev tooling..."
	cd backend/build && go get tool
	cd backend/internal/web && mkdir -p embed && touch embed/.gitignore
	echo "installing npm requirements for frontend..."
	cd frontend && npm i

setup-gofitz-cgo:
	echo "linking go-fitz MuPDF headers for preview CGO..."
	cd backend && go run ./scripts/setup-gofitz-cgo

update:
	cd backend && go get -u ./... && go mod tidy
	cd frontend && npm update

build: build-frontend build-backend

build-docker:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a" -t cannoncommander -f _docker/Dockerfile .

build-docker-slim:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a" -t cannoncommander -f _docker/Dockerfile.slim .

build-backend:
	@echo "Building backend..."
	cd backend && go build -o cannoncommander --ldflags="-w -s -X 'github.com/junkerderprovinz/cannoncommander/backend/internal/version.CommitSHA=testingCommit' -X 'github.com/junkerderprovinz/cannoncommander/backend/internal/version.Version=testing'"
	@echo "Backend built"

# The server embeds the build. For a live interface, run `npm run dev` in
# frontend/ next to this; Vite sends /api on to the server.
run: build-frontend setup-gofitz-cgo
	cd backend && CGO_ENABLED=1 go run --tags=mupdf \
	--ldflags="-w -s -X 'github.com/junkerderprovinz/cannoncommander/backend/internal/version.CommitSHA=testingCommit' -X 'github.com/junkerderprovinz/cannoncommander/backend/internal/version.Version=testing'" . -c test_config.yaml

generate-config:
	cd backend && CC_GENERATE_CONFIG=true go run .

build-frontend:
	@echo "Building frontend..."
	cd frontend && npm run build
	@echo "Frontend built"

lint-frontend:
	cd frontend && npm run typecheck

lint-backend:
	cd backend && GOLANGCI_LINT="$$(cd $(BACKEND_BUILD) && go tool -n golangci-lint)" && "$$GOLANGCI_LINT" run --path-prefix=backend

lint: lint-backend lint-frontend

test: test-backend test-frontend

check-all: lint test

test-backend:
	cd backend && go test -race -timeout=30s ./...

test-frontend:
	cd frontend && npm run test
