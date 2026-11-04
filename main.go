// mic-priority keeps the macOS default input set to the first available
// allowed device from a priority list, and can install itself as a LaunchAgent.
package main

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation
#include <CoreAudio/CoreAudio.h>
#include <CoreFoundation/CoreFoundation.h>
#include <unistd.h>

extern void goOnChange(void);

static OSStatus listener(AudioObjectID obj, UInt32 n, const AudioObjectPropertyAddress *a, void *ctx) {
	goOnChange();
	return noErr;
}

static AudioObjectPropertyAddress addr(AudioObjectPropertySelector sel, AudioObjectPropertyScope scope) {
	AudioObjectPropertyAddress a = {sel, scope, kAudioObjectPropertyElementMain};
	return a;
}

static void watch(void) {
	// Deliver notifications on the HAL's own thread so no CFRunLoop is needed.
	CFRunLoopRef rl = NULL;
	AudioObjectPropertyAddress a = addr(kAudioHardwarePropertyRunLoop, kAudioObjectPropertyScopeGlobal);
	AudioObjectSetPropertyData(kAudioObjectSystemObject, &a, 0, NULL, sizeof(rl), &rl);

	a = addr(kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal);
	AudioObjectAddPropertyListener(kAudioObjectSystemObject, &a, listener, NULL);
	a = addr(kAudioHardwarePropertyDefaultInputDevice, kAudioObjectPropertyScopeGlobal);
	AudioObjectAddPropertyListener(kAudioObjectSystemObject, &a, listener, NULL);
}

static int listDevices(AudioObjectID *out, int max) {
	AudioObjectPropertyAddress a = addr(kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal);
	UInt32 size = max * sizeof(AudioObjectID);
	if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &a, 0, NULL, &size, out) != noErr) return 0;
	return size / sizeof(AudioObjectID);
}

static int hasInput(AudioObjectID id) {
	AudioObjectPropertyAddress a = addr(kAudioDevicePropertyStreams, kAudioObjectPropertyScopeInput);
	UInt32 size = 0;
	if (AudioObjectGetPropertyDataSize(id, &a, 0, NULL, &size) != noErr || size == 0) return 0;
	// Skip inputs macOS never offers as the default (e.g. Zoom's loopback device).
	UInt32 ok = 1;
	a.mSelector = kAudioDevicePropertyDeviceCanBeDefaultDevice;
	size = sizeof(ok);
	AudioObjectGetPropertyData(id, &a, 0, NULL, &size, &ok);
	return ok != 0;
}

static int deviceName(AudioObjectID id, char *buf, int len) {
	AudioObjectPropertyAddress a = addr(kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal);
	CFStringRef s = NULL;
	UInt32 size = sizeof(s);
	if (AudioObjectGetPropertyData(id, &a, 0, NULL, &size, &s) != noErr || !s) return 0;
	Boolean ok = CFStringGetCString(s, buf, len, kCFStringEncodingUTF8);
	CFRelease(s);
	return ok;
}

// 0 = external (USB, Thunderbolt, ...), 1 = built-in, 2 = avoid by default.
static int deviceKind(AudioObjectID id) {
	AudioObjectPropertyAddress a = addr(kAudioDevicePropertyTransportType, kAudioObjectPropertyScopeGlobal);
	UInt32 t = kAudioDeviceTransportTypeUnknown, size = sizeof(t);
	AudioObjectGetPropertyData(id, &a, 0, NULL, &size, &t);
	switch (t) {
	case kAudioDeviceTransportTypeBuiltIn:
		return 1;
	case kAudioDeviceTransportTypeUnknown:
	case kAudioDeviceTransportTypeBluetooth:
	case kAudioDeviceTransportTypeBluetoothLE:
	case kAudioDeviceTransportTypeVirtual:
	case kAudioDeviceTransportTypeAggregate:
	case kAudioDeviceTransportTypeAirPlay:
	case kAudioDeviceTransportTypeContinuityCaptureWired:
	case kAudioDeviceTransportTypeContinuityCaptureWireless:
		return 2;
	default:
		return 0;
	}
}

static AudioObjectID getDefaultInput(void) {
	AudioObjectPropertyAddress a = addr(kAudioHardwarePropertyDefaultInputDevice, kAudioObjectPropertyScopeGlobal);
	AudioObjectID id = kAudioObjectUnknown;
	UInt32 size = sizeof(id);
	AudioObjectGetPropertyData(kAudioObjectSystemObject, &a, 0, NULL, &size, &id);
	return id;
}

static OSStatus setDefaultInput(AudioObjectID id) {
	AudioObjectPropertyAddress a = addr(kAudioHardwarePropertyDefaultInputDevice, kAudioObjectPropertyScopeGlobal);
	return AudioObjectSetPropertyData(kAudioObjectSystemObject, &a, 0, NULL, sizeof(id), &id);
}
*/
import "C"

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"
)

const (
	kindExternal = iota // USB, Thunderbolt, PCI, ...: usually the best mic
	kindBuiltIn
	kindAvoid // Bluetooth headsets, virtual/aggregate devices, iPhone Continuity, unknown
)

type device struct {
	id   C.AudioObjectID
	name string
	kind int
}

type config struct {
	allowed   []string        // [x], in priority order
	forbidden map[string]bool // [ ]
}

const configHeader = `# mic-priority: top to bottom, the first available [x] device becomes the default input.
# [ ] = forbidden: never the default input while anything else is available.
# Unlisted devices are not selected while at least one [x] device is available.
# Device names: mic-priority list. Changes apply automatically within a couple of seconds.

`

// parse reads lines like "[x] Name" / "[ ] Name"; a line without brackets is allowed.
func parse(text string) config {
	c := config{forbidden: map[string]bool{}}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		on := true
		if i := strings.IndexByte(line, ']'); line[0] == '[' && i > 0 {
			on = strings.TrimSpace(line[1:i]) != ""
			line = strings.TrimSpace(line[i+1:])
		}
		switch {
		case line == "":
		case on:
			c.allowed = append(c.allowed, line)
		default:
			c.forbidden[line] = true
		}
	}
	return c
}

// deviceLines renders devices in config format: external, then built-in, then
// avoided ones unchecked; alphabetical within each group.
func deviceLines(devs []device) string {
	devs = slices.Clone(devs)
	slices.SortFunc(devs, func(a, b device) int {
		return cmp.Or(cmp.Compare(a.kind, b.kind), strings.Compare(a.name, b.name))
	})
	var b strings.Builder
	for _, d := range devs {
		mark := "x"
		if d.kind == kindAvoid {
			mark = " "
		}
		fmt.Fprintf(&b, "[%s] %s\n", mark, d.name)
	}
	return b.String()
}

// target returns the device to make the default input; "" means leave it alone.
func target(c config, avail map[string]bool, cur string) string {
	for _, name := range c.allowed {
		if avail[name] {
			return name
		}
	}
	if !c.forbidden[cur] {
		return ""
	}
	for _, name := range slices.Sorted(maps.Keys(avail)) {
		if !c.forbidden[name] {
			return name
		}
	}
	return ""
}

var changed = make(chan struct{}, 1)

// Called on a CoreAudio thread: only signal here, the run loop does the work.
//
//export goOnChange
func goOnChange() {
	select {
	case changed <- struct{}{}:
	default:
	}
}

// inputs returns every device that has an input.
func inputs() []device {
	var ids [256]C.AudioObjectID
	n := C.listDevices(&ids[0], C.int(len(ids)))
	var res []device
	var buf [256]C.char
	for _, id := range ids[:n] {
		if C.hasInput(id) != 0 && C.deviceName(id, &buf[0], C.int(len(buf))) != 0 {
			res = append(res, device{id, strings.TrimSpace(C.GoString(&buf[0])), int(C.deviceKind(id))}) // parse() trims too
		}
	}
	return res
}

// currentInput returns the name of the default input device, or "".
func currentInput(devs []device) string {
	cur := C.getDefaultInput()
	for _, d := range devs {
		if d.id == cur {
			return d.name
		}
	}
	return ""
}

// ponytail: strict priority, a manual pick of another mic is reverted too; for manual control stop the agent.
func enforce(c config) {
	devs := inputs()
	ids, avail := map[string]C.AudioObjectID{}, map[string]bool{}
	for _, d := range devs {
		ids[d.name], avail[d.name] = d.id, true
	}
	curName := currentInput(devs)
	name := target(c, avail, curName)
	if name == "" || name == curName {
		return
	}
	if st := C.setDefaultInput(ids[name]); st != 0 {
		log.Printf("failed to select %q: OSStatus %d", name, st)
	} else {
		log.Printf("input: %s -> %s", curName, name)
	}
}

// ensureConfig writes a default config from the connected devices if none exists.
func ensureConfig(path string) error {
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return writeFileAtomic(path, []byte(configHeader+deviceLines(inputs())), 0o644)
}

func load(path string) (config, time.Time, error) {
	st, err := os.Stat(path)
	if err != nil {
		return config{}, time.Time{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return config{}, time.Time{}, err
	}
	c := parse(string(b))
	log.Printf("config: allowed %q, forbidden %q", c.allowed, slices.Sorted(maps.Keys(c.forbidden)))
	return c, st.ModTime(), nil
}

// logToFile sends logs (and Go runtime crashes) to ~/Library/Logs when stderr is
// /dev/null, which is how launchd starts the agent.
func logToFile() {
	a, err1 := os.Stderr.Stat()
	b, err2 := os.Stat(os.DevNull)
	if err1 != nil || err2 != nil || !os.SameFile(a, b) {
		return
	}
	if f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644); err == nil {
		syscall.Dup2(int(f.Fd()), 2)
	}
}

// run is the agent itself: enforce on every device change and config edit.
func run(path string) {
	logToFile()
	if err := ensureConfig(path); err != nil {
		log.Fatalf("config: %v", err)
	}
	c, mtime, err := load(path)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	C.watch()
	enforce(c)

	// ponytail: poll mtime every 2 s instead of kqueue; simpler and survives atomic saves by editors.
	tick := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-changed:
		case <-tick.C:
			st, err := os.Stat(path)
			if err != nil || st.ModTime().Equal(mtime) {
				continue
			}
			nc, nm, err := load(path)
			if err != nil {
				log.Printf("config: %v, keeping the previous one", err)
				continue
			}
			c, mtime = nc, nm
		}
		enforce(c)
	}
}

const usage = `mic-priority keeps the macOS default microphone set by a priority list.

Usage:
  mic-priority             show status; offer to install if not installed
  mic-priority install [--all-users]
                           install as a login agent for this user, or for every
                           user on this Mac (asks for an admin password); also updates
  mic-priority uninstall [--all-users]
                           stop and remove the agent (configs are kept)
  mic-priority list        print input devices in config format
  mic-priority run [-config FILE]
                           run the agent in the foreground
`

func main() {
	cmd, args := "", []string(nil)
	if len(os.Args) > 1 {
		cmd, args = os.Args[1], os.Args[2:]
	}
	allUsers := false
	if cmd == "install" || cmd == "uninstall" {
		fl := flag.NewFlagSet(cmd, flag.ExitOnError)
		fl.BoolVar(&allUsers, "all-users", false, "for every user on this Mac (asks for an admin password)")
		fl.Parse(args)
		if fl.NArg() > 0 { // "install x --all-users" would silently drop the flag
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	}
	root := os.Geteuid() == 0
	if root && !allUsers {
		check(errors.New("run as your user, without sudo (sudo is only for install/uninstall --all-users)"))
	}
	if !root && home == "" {
		check(errors.New("cannot determine the home directory"))
	}

	switch {
	case cmd == "":
		interactive()
	case cmd == "install" && !allUsers:
		check(install())
		status()
	case cmd == "install" && root:
		check(installAllUsers())
	case cmd == "install":
		check(elevate("mic-priority wants to install itself for all users of this Mac.", "install", "--all-users"))
		check(ensureConfig(confPath)) // the agent writes it too, but maybe not before status() reads it
		status()
	case cmd == "uninstall" && !allUsers:
		check(uninstall())
	case cmd == "uninstall" && !exists(sysPlist) && !exists(sysBin):
		check(errors.New("not installed for all users; for this user run: mic-priority uninstall"))
	case cmd == "uninstall" && root:
		check(uninstallAllUsers())
	case cmd == "uninstall":
		check(elevate("mic-priority wants to uninstall itself for all users of this Mac.", "uninstall", "--all-users"))
	case cmd == "list":
		fmt.Print(deviceLines(inputs()))
	case cmd == "run":
		fl := flag.NewFlagSet("run", flag.ExitOnError)
		path := fl.String("config", confPath, "config file")
		fl.Parse(args)
		run(*path)
	case cmd == "help" || cmd == "-h" || cmd == "-help" || cmd == "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
