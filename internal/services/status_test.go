package services

import "testing"

func TestWorkspacePhase(t *testing.T) {
	cases := []struct {
		ready, total int
		want         string
	}{
		{0, 0, "Empty"},
		{0, 3, "Stopped"},
		{2, 3, "Partial"},
		{3, 3, "Running"},
	}
	for _, c := range cases {
		if got := workspacePhase(c.ready, c.total); got != c.want {
			t.Errorf("workspacePhase(%d,%d) = %q, want %q", c.ready, c.total, got, c.want)
		}
	}
}
