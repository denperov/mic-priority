// mic-priority keeps the system default input set to the first
// available allowed device from mic-priority.conf.
package main

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation
#include <CoreAudio/CoreAudio.h>
#include <CoreFoundation/CoreFoundation.h>

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
	return AudioObjectGetPropertyDataSize(id, &a, 0, NULL, &size) == noErr && size > 0;
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
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type config struct {
	allowed   []string        // [x], in priority order
	forbidden map[string]bool // [ ]
}

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

// Called on a CoreAudio thread: only signal here, main does the work.
//
//export goOnChange
func goOnChange() {
	select {
	case changed <- struct{}{}:
	default:
	}
}

// inputs returns name -> id for every device that has an input.
func inputs() map[string]C.AudioObjectID {
	var ids [256]C.AudioObjectID
	n := C.listDevices(&ids[0], C.int(len(ids)))
	res := map[string]C.AudioObjectID{}
	var buf [256]C.char
	for _, id := range ids[:n] {
		if C.hasInput(id) != 0 && C.deviceName(id, &buf[0], C.int(len(buf))) != 0 {
			res[C.GoString(&buf[0])] = id
		}
	}
	return res
}

// ponytail: strict priority, a manual pick of another mic is reverted too; for manual control stop the agent.
func enforce(c config) {
	devs := inputs()
	cur := C.getDefaultInput()
	curName, avail := "", map[string]bool{}
	for name, id := range devs {
		avail[name] = true
		if id == cur {
			curName = name
		}
	}
	name := target(c, avail, curName)
	if name == "" || devs[name] == cur {
		return
	}
	if st := C.setDefaultInput(devs[name]); st != 0 {
		log.Printf("failed to select %q: OSStatus %d", name, st)
	} else {
		log.Printf("input: %s -> %s", curName, name)
	}
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

func main() {
	exe, _ := os.Executable()
	path := flag.String("config", filepath.Join(filepath.Dir(exe), "mic-priority.conf"), "config file")
	list := flag.Bool("list", false, "print input devices in config format and exit")
	flag.Parse()

	if *list {
		for _, name := range slices.Sorted(maps.Keys(inputs())) {
			fmt.Println("[x]", name)
		}
		return
	}

	c, mtime, err := load(*path)
	if err != nil {
		log.Fatalf("config: %v (template: ./mic-priority -list > mic-priority.conf)", err)
	}
	C.watch()
	enforce(c)

	// ponytail: poll mtime every 2 s instead of kqueue; simpler and survives atomic saves by editors.
	tick := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-changed:
		case <-tick.C:
			st, err := os.Stat(*path)
			if err != nil || st.ModTime().Equal(mtime) {
				continue
			}
			nc, nm, err := load(*path)
			if err != nil {
				log.Printf("config: %v, keeping the previous one", err)
				continue
			}
			c, mtime = nc, nm
		}
		enforce(c)
	}
}
