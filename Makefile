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
