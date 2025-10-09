# Simple project helpers for formatting and checks.

.PHONY: fmt fmtcheck imports importscheck vet check build run

fmt:
	go fmt ./...

# Lists files that are not gofmt'ed. Fails if any are found.
fmtcheck:
	@dirty=$$(gofmt -l $$(git ls-files '*.go')); \
	if [ -n "$$dirty" ]; then \
		echo "The following files need formatting:"; \
		echo "$$dirty" | sed 's/^/  - /'; \
		exit 1; \
	fi

vet:
	go vet ./...

imports:
	@command -v goimports >/dev/null 2>&1 || { \
		echo "goimports not installed. Install: go install golang.org/x/tools/cmd/goimports@latest"; \
		exit 1; \
	}
	goimports -w -local yf .

importscheck:
	@if command -v goimports >/dev/null 2>&1; then \
		out=$$(goimports -l $$(git ls-files '*.go')); \
		if [ -n "$$out" ]; then \
			echo "The following files need goimports (imports formatting):"; \
			echo "$$out" | sed 's/^/  - /'; \
			exit 1; \
		fi; \
	else \
		echo "goimports not installed; skipping imports check"; \
	fi

check: fmtcheck importscheck vet

build:
	go build -o yf .

run:
	go run . --help
