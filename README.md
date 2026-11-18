# mic-priority

Keeps the macOS default input set to the first available allowed device from a priority list,
so a bad Bluetooth headset mic never takes over when the headset connects.

## Quick install

No GitHub account or developer tools needed. Open Terminal (Applications → Utilities),
paste these lines and press Return:

```sh
curl -fLo "$TMPDIR/mic-priority" https://github.com/denperov/mic-priority/releases/latest/download/mic-priority &&
chmod +x "$TMPDIR/mic-priority" &&
"$TMPDIR/mic-priority" install
rm -f "$TMPDIR/mic-priority"
```

It downloads the latest release, installs it for your user and prints your mic priority list.
Paste the same lines again to update. Terminal doesn't know the short name `mic-priority`
used elsewhere in this README; type `~/.local/bin/mic-priority` instead.

Edit the list in TextEdit; changes apply within ~2 s after saving (see [Config](#config)):

```sh
open -e ~/.config/mic-priority.conf
```

Check the status:

```sh
~/.local/bin/mic-priority
```

Uninstall (the config is kept):

```sh
~/.local/bin/mic-priority uninstall
```

## Download

Every push is built on GitHub Actions into a universal (arm64 + x86_64) binary: the `mic-priority`
artifact of a run on the Actions tab, or a release asset for `v*` tags. With the GitHub CLI:

```sh
gh release download -R denperov/mic-priority -p mic-priority && chmod +x mic-priority
```

Downloaded with a browser? Clear the quarantine flag first, or Gatekeeper blocks the unsigned binary:

```sh
chmod +x mic-priority && xattr -d com.apple.quarantine mic-priority
```

## Install

A single self-installing binary. Run it without arguments and answer the prompt:

```sh
./mic-priority
```

It copies itself to `~/.local/bin/mic-priority`, creates `~/.config/mic-priority.conf`
and starts as a LaunchAgent (`local.mic-priority`) now and at every login.
Running it again shows the status; a newer binary run with `install` replaces the installed one.

### For all users

```sh
./mic-priority install --all-users
```

macOS shows its standard administrator password dialog (or use `sudo`). The binary goes to
`/usr/local/bin/mic-priority` and the agent to `/Library/LaunchAgents`, so it runs in every
user's login session; it starts right away for you, and for other users at their next login.
Each user still gets their own config, created on the agent's first run.
This replaces your per-user install, if any. Remove with `mic-priority uninstall --all-users`.

## Config

`~/.config/mic-priority.conf`, picked up automatically within ~2 s:

```
[x] Wireless Mic Rx          <- line order = priority
[x] MacBook Pro Microphone
[ ] JBL Tour One M3          <- [ ] = forbidden
```

The first run fills it from the connected devices: external (USB, ...) first, then built-in;
Bluetooth, virtual, aggregate and iPhone Continuity devices start unchecked.
Devices connected later are added the same way: a new external mic goes to the top and becomes
the default input, anything else goes to the bottom (Bluetooth etc. unchecked).
To ignore a device, uncheck it: a deleted line comes back the next time the device is connected.
`mic-priority list` prints the current devices in the same format.

Priority is strict: picking another mic manually in System Settings gets reverted too.
For manual control, run `mic-priority uninstall` (`--all-users` if installed for all users).

## Commands

```
mic-priority             status; offers to install if not installed
mic-priority install     install or update for this user, (re)start the agent
mic-priority install --all-users      same for every user (admin password)
mic-priority uninstall [--all-users]  stop and remove the agent (configs are kept)
mic-priority list        input devices in config format
mic-priority run         run the agent in the foreground
```

Log: `~/Library/Logs/mic-priority.log`.

## Build

```sh
make build   # ./mic-priority for this Mac
make dist    # dist/mic-priority, universal arm64 + x86_64
```
