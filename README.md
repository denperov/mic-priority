# mic-priority

Keeps the macOS default input set to the first available allowed device from `mic-priority.conf`:

```
[x] Wireless Mic Rx          <- line order = priority
[x] MacBook Pro Microphone
[ ] JBL Tour One M3          <- [ ] = forbidden
```

The config lives next to the binary; edits are picked up automatically within ~2 s.
`./mic-priority -list` prints the current input devices in the same format.

Autostart (LaunchAgent `local.mic-priority`):

```sh
make install     # build and start, survives reboots
make uninstall   # stop and remove
make log         # ~/Library/Logs/mic-priority.log
```

Priority is strict: picking another mic manually in System Settings gets reverted too. For manual control, run `make uninstall`.
