package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
)

type TokenProvider struct {
	secret string
	data   TokenDataAccess
}

type TokenDataAccess interface {
	LookupToken(hash string) (*User, error)
	StoreToken(*Token) error
	RevokeToken(hash string) error
	ListTokens() ([]*Token, error)
}

type Token struct {
	Hash  string // Hash of token. Don't store token itself
	User  string
	Perms PermissionLevel
}

func NewToken(secret string, data TokenDataAccess) *TokenProvider {
	return &TokenProvider{
		secret: secret,
		data:   data,
	}
}

//AccessTokens: Hash
// Key is SHA(token).
// Value is json representation of User struct

func (t *TokenProvider) GetUser(r *http.Request) (*User, error) {
	if tok := r.FormValue("token"); tok != "" {
		return t.data.LookupToken(t.hashToken(tok))
	}
	return nil, nil
}

func (t *TokenProvider) LoginHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Token auth does not support login route")
	})
}

func (t *TokenProvider) generateToken() string {
	buf := make([]byte, 15)
	rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}

func (t *TokenProvider) hashToken(tok string) string {
	data := append([]byte(tok), []byte(t.secret)...)
	sum := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (t *TokenProvider) CreateToken(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	tok := t.generateToken()
	hash := t.hashToken(tok)
	token := &Token{
		Hash:  hash,
		User:  r.FormValue("user"),
		Perms: Permission(r.FormValue("level")),
	}
	if err := t.data.StoreToken(token); err != nil {
		return nil, err
	}
	return tok, nil
}

func (t *TokenProvider) ListTokens(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return t.data.ListTokens()
}

func (t *TokenProvider) Revoke(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return nil, t.data.RevokeToken(r.FormValue("hash"))
}
