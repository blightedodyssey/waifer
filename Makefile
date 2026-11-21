.PHONY: build dev test clean

WEBKIT_TAGS :=

build:
	wails build $(WEBKIT_TAGS)

dev:
	wails dev $(WEBKIT_TAGS)

test:
	go test ./internal/... ./cli/...

clean:
	rm -rf build/bin
