# Simple project helpers for formatting and checks.

.PHONY: fmt fmtcheck imports importscheck vet check build run

fmt:
	go fmt ./...

# Lists files that are not gofmt'ed. Fails if any are found.
fmtcheck:
	# Use working tree files to avoid deleted tracked paths; exclude refs/
	@dirty=$$(find . -type f -name '*.go' ! -path './refs/*' -exec gofmt -l {} + 2>/dev/null); \
	if [ -n "$$dirty" ]; then \
		echo "The following files need formatting:"; \
		echo "$$dirty" | sed 's/^/  - /'; \
		exit 1; \
	fi

vet:
	mkdir -p .gocache
	GOCACHE=$(CURDIR)/.gocache go vet ./...

imports:
	@command -v goimports >/dev/null 2>&1 || { \
		echo "goimports not installed. Install: go install golang.org/x/tools/cmd/goimports@latest"; \
		exit 1; \
	}
	goimports -w -local github.com/komsit37/yf-go .

importscheck:
	@if command -v goimports >/dev/null 2>&1; then \
		out=$$(find . -type f -name '*.go' ! -path './refs/*' -exec goimports -l {} + 2>/dev/null); \
		if [ -n "$$out" ]; then \
			printf '%s\n' "The following files need goimports (imports formatting):"; \
			printf '%s\n' "$$out" | sed 's/^/  - /'; \
			exit 1; \
		fi; \
	else \
		echo "goimports not installed; skipping imports check"; \
	fi

check: fmtcheck importscheck vet

build:
	mkdir -p .gocache
	GOCACHE=$(CURDIR)/.gocache go build -o yf ./cmd/yf

run:
	mkdir -p .gocache
	GOCACHE=$(CURDIR)/.gocache go run ./cmd/yf --help
