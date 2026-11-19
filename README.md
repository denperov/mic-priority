# mic-priority

mic-priority automatically sets your Mac's input to the best available microphone from your priority list and moves it off any microphone you've blocked. You rank your microphones once; after that it reacts every time a mic is connected or disconnected.

The usual reason to want this: a Bluetooth headset with a poor microphone becomes the input every time it connects. mic-priority switches the input straight back to your USB or built-in mic. It only changes the input, so sound keeps playing through the headset.

It has no window and no menu bar icon. It runs in the background, starts at every login and restarts automatically if it stops.

**Requirements:** macOS 12 Monterey or later (tested on macOS 15), on Apple silicon or Intel. You don't need a GitHub account or developer tools, and installing it for yourself doesn't need an administrator password.

## Install

### 1. Run the install command

Open Terminal: press Cmd+Space, type `Terminal` and press Return. It is also in Finder, under Applications → Utilities.

Copy the block below: hover over it and click the copy icon at its top right. Click in the Terminal window, press Cmd+V, then press Return.

```sh
curl -fLo "$TMPDIR/mic-priority" https://github.com/denperov/mic-priority/releases/latest/download/mic-priority &&
chmod +x "$TMPDIR/mic-priority" &&
"$TMPDIR/mic-priority" install
rm -f "$TMPDIR/mic-priority"
```

This downloads the latest release to a temporary folder, installs it for your macOS user account, starts it and then deletes the download. To install it for every user of this Mac, use the [all-users command](#install-for-all-users) instead.

### 2. Check the output

First you'll see a small download progress table. After it, you should see something like this:

```text
mic-priority is installed for this user and running (pid 4242).
  binary: ~/.local/bin/mic-priority
  config: ~/.config/mic-priority.conf (edits apply within ~2 s)
  log:    ~/Library/Logs/mic-priority.log
  input:  Wireless Mic Rx

Priority:
  [x] Wireless Mic Rx
  [x] MacBook Pro Microphone
  [ ] JBL Tour One M3

Note: ~/.local/bin is not in PATH; call the binary by its full path.
```

- `running` means it is working. If it says `not running`, see [Troubleshooting](#troubleshooting).
- `input:` is the current default input.
- `Priority:` is your list. On the first install it is built from the mics connected at that moment: USB and other external mics first, then the built-in mic, all checked (`[x]`). Bluetooth, AirPlay, iPhone, virtual and aggregate mics are added unchecked (`[ ]`), so they are not used while a checked mic is connected. You can [change the list](#change-the-priority-list) at any time.
- The `Note:` line means Terminal doesn't recognize the short name `mic-priority`. Type the full path `~/.local/bin/mic-priority` instead. The commands in this README already do this.

macOS may also show a notification that a background item was added. That item is mic-priority.

### 3. Try it

Connect your Bluetooth headset. If macOS switches the input to the headset's mic, mic-priority switches it straight back. A new USB mic is different: it goes to the top of the list and becomes the input.

You can check the input in System Settings → Sound → Input, or ask mic-priority:

```sh
~/.local/bin/mic-priority
```

Every switch is written to the log, for example `input: JBL Tour One M3 -> MacBook Pro Microphone`, and so is every change to your list. This command opens the log:

```sh
open ~/Library/Logs/mic-priority.log
```

## Install for all users

To run mic-priority for every user of this Mac, paste this instead of the normal install command. It is the same command with `--all-users` added:

```sh
curl -fLo "$TMPDIR/mic-priority" https://github.com/denperov/mic-priority/releases/latest/download/mic-priority &&
chmod +x "$TMPDIR/mic-priority" &&
"$TMPDIR/mic-priority" install --all-users
rm -f "$TMPDIR/mic-priority"
```

macOS shows its usual administrator password dialog. After that you see the same status as above, with these differences:

- The program is in `/usr/local/bin/mic-priority`, which is in PATH, so the short command `mic-priority` works. Use it wherever this README says `~/.local/bin/mic-priority`.
- It starts now for you, and for other users the next time they log in. Each user gets their own list in their own `~/.config/mic-priority.conf`, created the first time it runs for them.
- If you had a per-user install, it is replaced. Your list is kept.

## Download with a browser

You can also download the program with a browser, without a GitHub account. Open the [latest release](https://github.com/denperov/mic-priority/releases/latest) and click `mic-priority` under **Assets**. Browsers mark downloaded files as quarantined, and macOS won't run a quarantined program that isn't signed with an Apple Developer ID. This makes the file runnable and removes the quarantine mark:

```sh
chmod +x ~/Downloads/mic-priority && xattr -c ~/Downloads/mic-priority
```

Then install it for your user:

```sh
~/Downloads/mic-priority install
```

Or for all users instead:

```sh
~/Downloads/mic-priority install --all-users
```

If macOS asks whether Terminal may access your Downloads folder, allow it. When the install is done, you can delete `mic-priority` from Downloads: the installed copy is separate.

## Change the priority list

Open the list in TextEdit:

```sh
open -e ~/.config/mic-priority.conf
```

Below a few comment lines that start with `#`, the file holds your list, one mic per line:

```text
[x] Wireless Mic Rx
[x] MacBook Pro Microphone
[ ] JBL Tour One M3
```

With this list, Wireless Mic Rx is the input whenever it is plugged in. When it is unplugged, the input falls back to MacBook Pro Microphone. The JBL headset's mic is not used as long as one of the other two is connected.

- Order is priority. The first connected `[x]` mic becomes the default input. Move lines up or down to change the order.
- `[ ]` blocks a mic. A blocked mic never becomes the default input while an `[x]` mic is connected.
- Names must match the device names exactly, so don't edit them. To print the mics connected right now in this format, run `~/.local/bin/mic-priority list`.
- A line without brackets counts as `[x]`. Lines starting with `#` are comments.
- Save with Cmd+S. The change takes effect within about 2 seconds, and you don't need to restart anything.

New mics are added to the list automatically when you connect them. A new USB or other wired mic goes to the top, checked, and becomes the input right away. This includes the mic or headset jack of a webcam, dock, display or audio interface; uncheck it if you don't want it used. Any other new mic goes to the bottom. Bluetooth, AirPlay, iPhone, virtual and aggregate mics are added unchecked.

To stop a mic from being used, uncheck it instead of deleting its line. A deleted mic comes back as soon as it is connected, or right away if it is connected now.

> [!IMPORTANT]
> **Priority is strict.** If you pick another mic in System Settings, or another app changes the system input, mic-priority switches it back. To use a different mic, move it up the list or uncheck the mics above it. To stop the switching completely, [uninstall](#uninstall) it. mic-priority only changes the macOS default input: an app that is set to a specific mic in its own settings keeps using that mic.

## Update

mic-priority doesn't update itself. To update, paste the [install command](#1-run-the-install-command) again, or the [all-users command](#install-for-all-users) if you installed for all users. It replaces the program with the latest release and restarts it. Your list is kept.

## Uninstall

If you installed it for your user:

```sh
~/.local/bin/mic-priority uninstall
```

If you installed it for all users (asks for an administrator password):

```sh
mic-priority uninstall --all-users
```

Other users who are logged in keep their running copy until they log out.

Uninstalling stops mic-priority and removes the program. Your list in `~/.config/mic-priority.conf` and the log are kept in case you reinstall later; with an all-users install, every user's list is kept. To delete your list and log as well:

```sh
rm -f ~/.config/mic-priority.conf ~/Library/Logs/mic-priority.log
```

## Troubleshooting

- **`zsh: command not found: mic-priority`**: with a per-user install, type `~/.local/bin/mic-priority`.
- **Status says `not running`**: paste the [install command](#1-run-the-install-command) again, or the [all-users command](#install-for-all-users) if you installed for all users. If someone else installed it for all users while you were logged in, just log out and back in. If it still says `not running; see ~/Library/Logs/mic-priority.log`, the reason is at the end of the log.
- **`error: already installed for all users`**: to update, use the [all-users command](#install-for-all-users). To switch to a per-user install, run `mic-priority uninstall --all-users` first.
- **`error: run as your user, without sudo`**: run the command without `sudo`. Only `--all-users` needs administrator rights, and it asks for the password itself.
- **A mic is never selected**: it must be checked (`[x]`), and its name must exactly match a line of `~/.local/bin/mic-priority list`. A connected `[x]` mic higher in the list always wins.

## Reference

### Commands

With a per-user install, type `~/.local/bin/mic-priority` wherever this table says `mic-priority`.

| Command | What it does |
|---|---|
| `mic-priority` | Show the status. If it isn't installed, explain what install will do and ask before installing. |
| `mic-priority install` | Install or update for this user, and (re)start it. |
| `mic-priority install --all-users` | The same for every user of this Mac (asks for an administrator password). |
| `mic-priority uninstall [--all-users]` | Stop and remove it. Lists and logs are kept. |
| `mic-priority list` | Print the connected mics in list format. |
| `mic-priority run [-config FILE]` | Run in the foreground and log to the terminal instead of the log file. Ctrl+C stops it. |

### Files

| | Installed for this user | Installed for all users |
|---|---|---|
| Program | `~/.local/bin/mic-priority` | `/usr/local/bin/mic-priority` |
| Login agent `local.mic-priority` | `~/Library/LaunchAgents/local.mic-priority.plist` | `/Library/LaunchAgents/local.mic-priority.plist` |
| Priority list | `~/.config/mic-priority.conf` | each user's `~/.config/mic-priority.conf` |
| Log | `~/Library/Logs/mic-priority.log` | each user's `~/Library/Logs/mic-priority.log` |

## Building from source

You need Go (the version in `go.mod`) and the Xcode Command Line Tools (`xcode-select --install`). The CoreAudio calls use cgo.

```sh
make dist
```

This builds `dist/mic-priority`, the universal arm64 + x86_64 binary. `make build` builds `./mic-priority` for this Mac only; `make install` and `make uninstall` build it, then install or uninstall it for this user; `make log` follows the log.

On every push, GitHub Actions runs `go vet`, `go test` and `make dist`, and keeps the binary as a workflow artifact. Downloading that artifact requires a GitHub login. A `v*` tag also publishes the binary as a GitHub release, which is the file the install command downloads.
