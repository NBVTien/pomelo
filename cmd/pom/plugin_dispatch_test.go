package main

import "testing"

func TestFirstSubcommand(t *testing.T) {
	cases := []struct {
		args []string
		want string
		idx  int
	}{
		{nil, "", -1},
		{[]string{"--help"}, "", -1},
		{[]string{"web"}, "web", 0},
		{[]string{"-v", "hello", "world"}, "hello", 1},
		{[]string{"hello", "--flag", "x"}, "hello", 0},
	}
	for _, c := range cases {
		got, idx := firstSubcommand(c.args)
		if got != c.want || idx != c.idx {
			t.Errorf("firstSubcommand(%v) = (%q,%d), want (%q,%d)", c.args, got, idx, c.want, c.idx)
		}
	}
}
