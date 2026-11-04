package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTarget(t *testing.T) {
	c := parse(`
# comment
[x] DJI
[X]   Built-in
[ ] JBL
[] Zoom
Bare
[x]
`)
	if len(c.allowed) != 3 || c.allowed[0] != "DJI" || c.allowed[1] != "Built-in" || c.allowed[2] != "Bare" {
		t.Fatalf("allowed = %q", c.allowed)
	}
	if !c.forbidden["JBL"] || !c.forbidden["Zoom"] || len(c.forbidden) != 2 {
		t.Fatalf("forbidden = %v", c.forbidden)
	}

	set := func(names ...string) map[string]bool {
		m := map[string]bool{}
		for _, n := range names {
			m[n] = true
		}
		return m
	}
	for _, tc := range []struct {
		avail []string
		cur   string
		want  string
	}{
		{[]string{"DJI", "Built-in", "JBL"}, "JBL", "DJI"}, // highest priority
		{[]string{"Built-in", "JBL"}, "JBL", "Built-in"},   // next in the list
		{[]string{"JBL", "Other"}, "Other", ""},            // no allowed device, current not forbidden: leave it
		{[]string{"JBL", "Zoom", "Other"}, "JBL", "Other"}, // current forbidden: any non-forbidden device
		{[]string{"JBL", "Zoom"}, "JBL", ""},               // only forbidden devices left
	} {
		if got := target(c, set(tc.avail...), tc.cur); got != tc.want {
			t.Errorf("target(%v, %q) = %q, want %q", tc.avail, tc.cur, got, tc.want)
		}
	}
}

func TestDeviceLines(t *testing.T) {
	devs := []device{
		{name: "JBL Tour One M3", kind: kindAvoid},
		{name: "MacBook Pro Microphone", kind: kindBuiltIn},
		{name: "ZoomAudioDevice", kind: kindAvoid},
		{name: "Wireless Mic Rx", kind: kindExternal},
	}
	want := "[x] Wireless Mic Rx\n[x] MacBook Pro Microphone\n[ ] JBL Tour One M3\n[ ] ZoomAudioDevice\n"
	got := deviceLines(devs)
	if got != want {
		t.Fatalf("deviceLines:\n%s\nwant:\n%s", got, want)
	}
	if devs[0].name != "JBL Tour One M3" {
		t.Fatal("deviceLines must not reorder its input")
	}
	c := parse(configHeader + got)
	if len(c.allowed) != 2 || c.allowed[0] != "Wireless Mic Rx" || len(c.forbidden) != 2 {
		t.Fatalf("round trip: %+v", c)
	}
}

func TestPlist(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.plist")
	if err := os.WriteFile(f, []byte(plist(`/Users/a & b/<bin> "x"`)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("plutil", "-lint", f).CombinedOutput(); err != nil {
		t.Fatalf("plutil: %v\n%s", err, out)
	}
	out, err := exec.Command("plutil", "-extract", "ProgramArguments.0", "raw", f).Output()
	if err != nil || string(out) != `/Users/a & b/<bin> "x"`+"\n" {
		t.Fatalf("ProgramArguments.0 = %q, %v", out, err)
	}
}

// The elevate() command line must survive shell quoting inside an AppleScript string.
func TestQuote(t *testing.T) {
	tricky := "/Users/a b/it's \"q\" $(id) `x` \\n\nline2"
	script := "do shell script " + appleQuote("printf %s "+shellQuote(tricky)) + " without altering line endings"
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSuffix(string(out), "\n"); got != tricky {
		t.Fatalf("got %q, want %q", got, tricky)
	}
}
