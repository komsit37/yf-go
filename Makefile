# Simple project helpers for formatting and checks.

.PHONY: fmt fmtcheck vet check build run

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

check: fmtcheck vet

build:
	go build -o yf .

run:
	go run . --help

