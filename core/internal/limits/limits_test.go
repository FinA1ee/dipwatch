package limits

import "testing"

func TestInviteList(t *testing.T) {
	l := Defaults
	l.Invited = []string{" Friend@Example.com ", "friend@example.com", ""}
	l.normalize()
	if len(l.Invited) != 1 || l.Invited[0] != "friend@example.com" {
		t.Fatalf("normalized: %q", l.Invited)
	}
	if err := l.Validate(); err != nil {
		t.Fatal(err)
	}
	for email, want := range map[string]bool{"FRIEND@example.com": true, "other@example.com": false, "": false} {
		if got := l.Allows(email); got != want {
			t.Errorf("Allows(%q) = %v, want %v", email, got, want)
		}
	}
	l.InviteOnly = false
	if !l.Allows("") {
		t.Error("open site refused a visitor")
	}
	l.Invited = []string{"not an email"}
	if l.Validate() == nil {
		t.Error("accepted a non-address")
	}
}
