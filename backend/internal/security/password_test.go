package security

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "valid", password: "Admin12345678!", wantErr: false},
		{name: "short", password: "A1!short", wantErr: true},
		{name: "missing upper", password: "admin12345678!", wantErr: true},
		{name: "missing digit", password: "AdminPassword!!", wantErr: true},
		{name: "missing symbol", password: "Admin123456789", wantErr: true},
	}

	for _, tc := range cases {
		err := ValidatePassword(tc.password)
		if tc.wantErr && err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}
}
