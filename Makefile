# Oldest macOS the binary runs on (Go 1.25 needs 12); without it, cgo stamps the
# build machine's macOS, e.g. 26.0 on the CI runner. A flag, not
# MACOSX_DEPLOYMENT_TARGET: Go's build cache keys on CGO_*FLAGS but not on that
# variable, so cached runtime/cgo objects for a newer macOS would get linked in.
export CGO_CFLAGS := -O2 -g -mmacosx-version-min=12.0
export CGO_LDFLAGS := -O2 -g -mmacosx-version-min=12.0

build:
	go build -o mic-priority .

# Universal (arm64 + x86_64) binary for downloading onto any Mac.
dist:
	mkdir -p dist
	CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -o dist/mic-priority-arm64 .
	CGO_ENABLED=1 GOARCH=amd64 go build -trimpath -o dist/mic-priority-amd64 .
	lipo -create -output dist/mic-priority dist/mic-priority-arm64 dist/mic-priority-amd64
	rm dist/mic-priority-arm64 dist/mic-priority-amd64

install: build
	./mic-priority install

uninstall: build
	./mic-priority uninstall

log:
	tail -f $(HOME)/Library/Logs/mic-priority.log

.PHONY: build dist install uninstall log
