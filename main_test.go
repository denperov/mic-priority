package main

import "testing"

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
