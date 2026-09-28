package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

// tokenBytes is the size of the random session token (hex-encoded to twice as many chars).
const tokenBytes = 32

// Service checks credentials against the single demo user.
type Service struct {
	username, password string
}

// NewService returns a service for the demo user.
func NewService(username, password string) *Service {
	return &Service{username: username, password: password}
}

// Login returns a session for the demo user or ErrUnauthorized.
func (s *Service) Login(c Credentials) (Session, error) {
	// Both comparisons always run, in constant time, so timing reveals neither field.
	userOK := subtle.ConstantTimeCompare([]byte(c.Username), []byte(s.username))
	passOK := subtle.ConstantTimeCompare([]byte(c.Password), []byte(s.password))
	if userOK&passOK != 1 {
		return Session{}, fmt.Errorf("invalid username or password: %w", apperr.ErrUnauthorized)
	}
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return Session{Token: hex.EncodeToString(b), User: User{Username: s.username}}, nil
}
