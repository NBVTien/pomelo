package configbundle

import "testing"

func TestRoundTripEncrypted(t *testing.T) {
	yaml := "session: demo\n"
	secrets := map[string]string{"STRIPE": "sk_live_x", "TOK": "abc"}
	blob, err := BuildEncrypted(yaml, secrets, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(blob) {
		t.Fatal("not detected as encrypted")
	}
	gotY, gotS, err := Open(blob, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if gotY != yaml || gotS["STRIPE"] != "sk_live_x" || gotS["TOK"] != "abc" {
		t.Fatalf("mismatch: %q %v", gotY, gotS)
	}
	if _, _, err := Open(blob, "wrong"); err == nil {
		t.Fatal("expected wrong-password failure")
	}
}

func TestPlain(t *testing.T) {
	y, s, err := Open(BuildPlain("session: x\n"), "")
	if err != nil || y != "session: x\n" || s != nil {
		t.Fatalf("plain: %q %v %v", y, s, err)
	}
}
