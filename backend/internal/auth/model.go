// Package auth serves the demo login. It is a mock: it checks one configured user and
// returns an opaque token that no other endpoint verifies.
package auth

// Credentials is the POST /auth/login body.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// User is the logged-in user.
type User struct {
	Username string `json:"username"`
}

// Session is the POST /auth/login response.
type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
