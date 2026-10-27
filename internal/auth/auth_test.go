package auth

import (
	"errors"
	"testing"
)

func TestParsePermissions(t *testing.T) {
	got, err := ParsePermissions("read|write|zip")
	if err != nil {
		t.Fatalf("ParsePermissions: %v", err)
	}
	if len(got) != 3 || got[0] != PermRead || got[2] != PermZip {
		t.Fatalf("got %v", got)
	}
}

func TestParsePermissionsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "||"} {
		got, err := ParsePermissions(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
		}
		if len(got) != 0 {
			t.Errorf("%q: got %v, want none", in, got)
		}
	}
}

func TestParsePermissionsInvalid(t *testing.T) {
	_, err := ParsePermissions("read|teleport")
	if !errors.Is(err, ErrInvalidPerm) {
		t.Fatalf("got %v, want ErrInvalidPerm", err)
	}
}

func TestEncodePermissionsRoundTrip(t *testing.T) {
	in := []Permission{PermRead, PermWrite, PermChmod}
	parsed, err := ParsePermissions(EncodePermissions(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != len(in) {
		t.Fatalf("got %v", parsed)
	}
	for i := range in {
		if parsed[i] != in[i] {
			t.Fatalf("got %v want %v", parsed, in)
		}
	}
}

func TestHasPermissions(t *testing.T) {
	u := &User{Permissions: []Permission{PermRead, PermWrite}}
	if !u.Can() {
		t.Error("empty requirement should pass")
	}
	if !u.Can(PermRead, PermWrite) {
		t.Error("expected both permissions")
	}
	if u.Can(PermRead, PermZip) {
		t.Error("unexpected zip permission")
	}
}

func TestRoles(t *testing.T) {
	if (&User{Role: RoleAdmin}).IsAdmin() == false {
		t.Error("admin should be admin")
	}
	if (&User{Role: RoleAdmin}).IsGuest() {
		t.Error("admin should not be guest")
	}
	for _, r := range []Role{RoleGuest, RoleUser, RoleAdmin} {
		if !ValidRole(r) {
			t.Errorf("%q should be valid", r)
		}
	}
	if ValidRole("root") {
		t.Error("root should be invalid")
	}
}
