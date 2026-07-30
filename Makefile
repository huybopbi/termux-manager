APP     = manager
MODULE  = github.com/huybopbi/termux-manager
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build android-arm64 android-arm android-amd64 clean fmt

all: android-arm64 android-arm android-amd64

build:
	go build $(LDFLAGS) -o $(APP) .

android-arm64:
	GOOS=android GOARCH=arm64 CGO_ENABLED=0 \
	go build $(LDFLAGS) -o dist/$(APP)-android-arm64 .
	@echo "✓ dist/$(APP)-android-arm64"

android-arm:
	GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=0 \
	go build $(LDFLAGS) -o dist/$(APP)-android-arm .
	@echo "✓ dist/$(APP)-android-arm"

android-amd64:
	GOOS=android GOARCH=amd64 CGO_ENABLED=0 \
	go build $(LDFLAGS) -o dist/$(APP)-android-amd64 .
	@echo "✓ dist/$(APP)-android-amd64"

linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build $(LDFLAGS) -o dist/$(APP)-linux-amd64 .

clean:
	rm -rf dist/ $(APP)

fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

run:
	go run . -root /tmp

dist:
	mkdir -p dist
