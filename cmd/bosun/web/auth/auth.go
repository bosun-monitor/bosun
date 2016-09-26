package auth

import (
	"net/http"
	"strings"
)

// PermissionLevel is a level of permission required for an operation
type PermissionLevel int

const (
	//None means no special premissions granted. Only static content should be served
	None PermissionLevel = 1 << iota
	//Reader can access data from bosun/backends, but not alter it
	Reader
	//Admin has full control over all bosun actions
	Admin
)

func (p PermissionLevel) String() string {
	switch p {
	case None:
		return "None"
	case Reader:
		return "Reader"
	case Admin:
		return "Admin"
	}
	return "Unknown"
}

func Permission(s string) PermissionLevel {
	switch strings.ToLower(s) {
	case "none":
		return None
	case "read":
		return Reader
	case "admin":
		return Admin
	}
	return None
}

// User represents a "user" with their name and Permissions
type User struct {
	Name        string
	Permissions PermissionLevel
	AuthMethod  string
}

//Provider is an interface any auth provider should implement
type Provider interface {
	// Get the associated user from cookie/query data
	GetUser(r *http.Request) (*User, error)

	// Handler for the login page. Will handle /login (and all subroutes if more callbacks needed).
	// On an auth failure, user will be redirected to /login. All additional POSTS or callbacks are up to implementation to handle.
	LoginHandler() http.Handler
}
