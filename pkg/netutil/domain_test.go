package netutil

import "testing"

func TestIsValidDomain(t *testing.T) {
	valid := []string{
		"example.com",
		"app.local",
		"a.b.c.d.e",
		"my-app.home.local",
		"x",
		"a1.b2.c3",
		"pihole.home.lan",
	}
	for _, d := range valid {
		if !IsValidDomain(d) {
			t.Errorf("expected %q to be valid", d)
		}
	}

	invalid := []string{
		"",
		"-start.local",
		"end-.local",
		".leading-dot.local",
		"trailing-dot.local.",
		"spa ces.local",
		"new\nline.local",
		"tab\there.local",
		string(make([]byte, 254)), // too long
		"label-" + string(make([]byte, 64)) + ".local", // label too long
	}
	for _, d := range invalid {
		if IsValidDomain(d) {
			t.Errorf("expected %q to be invalid", d)
		}
	}
}
