package services

import "testing"

func TestBranchHost(t *testing.T) {
	cases := map[string]string{
		"proj-855":         "proj-855",
		"PROJ-855":         "proj-855",
		"feat/add-login":   "feat-add-login",
		"bss_send_confirm": "bss-send-confirm",
		"-weird-/":         "weird",
	}
	for in, want := range cases {
		if got := BranchHost(in); got != want {
			t.Errorf("BranchHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBranchHostDNSLabelCap(t *testing.T) {
	long := "proj-1069-rent-manager-let-partners-map-their-own-custom-fields-udfs-to-acme-fields"
	got := BranchHost(long)
	if len(got) > dnsLabelMax {
		t.Fatalf("label %q has len %d, exceeds DNS label max %d", got, len(got), dnsLabelMax)
	}
	if got[0] == '-' || got[len(got)-1] == '-' {
		t.Errorf("label %q must not start/end with '-'", got)
	}

	other := long + "-extra-tail-that-differs-only-past-the-cut"
	if BranchHost(long) == BranchHost(other) {
		t.Errorf("distinct long branches collided: %q", got)
	}

	if BranchHost(long) != got {
		t.Errorf("BranchHost not deterministic for %q", long)
	}
}
