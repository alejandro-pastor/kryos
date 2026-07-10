VERSION := 0.1.0
COMMIT := $(shell git rev-parse --short HEAD)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o kryos ./cmd/kryos

install: build
	sudo install -m 0755 kryos /usr/local/bin/kryos
	sudo ./kryos --install

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -f kryos

check-scripts:
	@diff -q internal/scripts/kryos-stress-test.sh scripts/kryos-stress-test.sh >/dev/null 2>&1 || \
	  { echo "ERROR: scripts out of sync! Run: cp scripts/kryos-stress-test.sh internal/scripts/"; exit 1; }
	@echo "✅ scripts are in sync"

vet:
	go vet ./...

.PHONY: build install test tidy clean check-scripts vet
