package forge

import "testing"

func TestIsHexSHARejectsNonHex(t *testing.T) {
	good := []string{"a1b2c3d", "0123456789abcdef0123456789abcdef01234567"}
	for _, s := range good {
		if !isHexSHA(s) {
			t.Errorf("%q should be accepted", s)
		}
	}
	// A sha lands in an argv slot git also reads options from.
	bad := []string{"", "abc", "HEAD", "--output=/tmp/pwn", "a1b2c3d;rm -rf /", "A1B2C3D",
		"0123456789abcdef0123456789abcdef012345678"}
	for _, s := range bad {
		if isHexSHA(s) {
			t.Errorf("%q should be rejected", s)
		}
	}
}
