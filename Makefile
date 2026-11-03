PLIST := $(HOME)/Library/LaunchAgents/local.mic-priority.plist
LOG   := $(HOME)/Library/Logs/mic-priority.log
GUI   := gui/$(shell id -u)

build:
	go build -o mic-priority .

install: build
	sed -e 's|@BIN@|$(CURDIR)/mic-priority|' -e 's|@LOG@|$(LOG)|' local.mic-priority.plist > $(PLIST)
	-launchctl bootout $(GUI) $(PLIST) 2>/dev/null
	launchctl bootstrap $(GUI) $(PLIST)

uninstall:
	-launchctl bootout $(GUI) $(PLIST)
	rm -f $(PLIST)

log:
	tail -f $(LOG)

.PHONY: build install uninstall log
