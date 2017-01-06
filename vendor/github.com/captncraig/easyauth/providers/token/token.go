package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/captncraig/easyauth"
)

type TokenProvider struct {
	secret string
	data   TokenDataAccess
}

type TokenDataAccess interface {
	LookupToken(hash string) (*easyauth.User, error)
	StoreToken(*Token) error
	RevokeToken(hash string) error
	ListTokens() ([]*Token, error)
}

type Token struct {
	Hash        string // Hash of token. Don't store token itself
	User        string
	Role        easyauth.Role
	Description string
	LastUsed    time.Time
}

func NewToken(secret string, data TokenDataAccess) *TokenProvider {
	return &TokenProvider{
		secret: secret,
		data:   data,
	}
}

func (t *TokenProvider) GetUser(r *http.Request) (*easyauth.User, error) {
	var tok string
	if cook, err := r.Cookie("AccessToken"); err == nil {
		tok = cook.Value
	} else if tok = r.FormValue("token"); tok == "" {
		if tok = r.Header.Get("X-Access-Token"); tok == "" {
			return nil, nil
		}
	}
	return t.data.LookupToken(t.hashToken(tok))
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

func (t *TokenProvider) NewToken(user, description string, role easyauth.Role) (string, error) {
	tok := t.generateToken()
	hash := t.hashToken(tok)
	token := &Token{
		Hash:        hash,
		User:        user,
		Role:        role,
		Description: description,
	}
	if err := t.data.StoreToken(token); err != nil {
		return "", err
	}
	return tok, nil
}

func (t *TokenProvider) createToken(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	tok := &Token{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(tok); err != nil {
		return nil, err
	}
	return t.NewToken(tok.User, tok.Description, tok.Role)
}

func (t *TokenProvider) listTokens(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return t.data.ListTokens()
}

func (t *TokenProvider) revoke(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return nil, t.data.RevokeToken(r.FormValue("hash"))
}

func (t *TokenProvider) AdminHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var target func(http.ResponseWriter, *http.Request) (interface{}, error)
		switch r.Method {
		case http.MethodGet:
			target = t.listTokens
		case http.MethodPost:
			target = t.createToken
		case http.MethodDelete:
			target = t.revoke
		}
		response, err := target(w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if response == nil {
			return
		}
		enc := json.NewEncoder(w)
		enc.Encode(response)
	})
}
