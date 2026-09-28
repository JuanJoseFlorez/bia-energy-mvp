package auth

import (
	"errors"
	"regexp"
	"testing"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

var hexToken = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestLogin(t *testing.T) {
	tests := []struct {
		name  string
		creds Credentials
		ok    bool
	}{
		{"valid", Credentials{Username: "demo", Password: "s3cret"}, true},
		{"wrong password", Credentials{Username: "demo", Password: "nope"}, false},
		{"wrong user", Credentials{Username: "admin", Password: "s3cret"}, false},
		{"password prefix", Credentials{Username: "demo", Password: "s3cre"}, false},
		{"empty", Credentials{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewService("demo", "s3cret").Login(tt.creds)

			if !tt.ok {
				if !errors.Is(err, apperr.ErrUnauthorized) {
					t.Errorf("error = %v, want ErrUnauthorized", err)
				}
				return
			}
			if err != nil || s.User.Username != "demo" || !hexToken.MatchString(s.Token) {
				t.Errorf("Login() = %+v, %v", s, err)
			}
		})
	}
}

func TestLoginTokensDiffer(t *testing.T) {
	svc := NewService("demo", "demo")
	a, _ := svc.Login(Credentials{Username: "demo", Password: "demo"})
	b, _ := svc.Login(Credentials{Username: "demo", Password: "demo"})

	if a.Token == b.Token {
		t.Error("two logins returned the same token")
	}
}
