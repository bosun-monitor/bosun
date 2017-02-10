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
	LookupToken(hash string) (*Token, error)
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
	RoleHash    string // Hash of token and Role for security purposes
}

func NewToken(secret string, data TokenDataAccess) *TokenProvider {
	return &TokenProvider{
		secret: secret,
		data:   data,
	}
}

func (t *TokenProvider) GetUser(r *http.Request) (*easyauth.User, error) {
	var tokn string
	if cook, err := r.Cookie("AccessToken"); err == nil {
		tokn = cook.Value
	} else if tokn = r.URL.Query().Get("token"); tokn == "" {
		if tokn = r.Header.Get("X-Access-Token"); tokn == "" {
			return nil, nil
		}
	}
	tok, err := t.data.LookupToken(t.hashToken(tokn))
	if err != nil {
		return nil, err
	}
	//verify hash with role to ensure not tampered with
	hashWithRole := t.hashToken(tokn + fmt.Sprint(tok.Role))
	if hashWithRole != tok.RoleHash {
		return nil, fmt.Errorf("Token hash mismatch")
	}
	return &easyauth.User{
		Access:   tok.Role,
		Method:   "token",
		Username: tok.User,
	}, nil
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

//Creates two hashes. One with the roles included (for security against modification), and one without (for fast lookup)
func (t *TokenProvider) hashToken(tok string) string {
	data := append([]byte(tok), []byte(t.secret)...)
	sum := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (t *TokenProvider) NewToken(user, description string, role easyauth.Role) (string, error) {
	tok := t.generateToken()
	hash := t.hashToken(tok)
	hashWithRole := t.hashToken(tok + fmt.Sprint(role))
	token := &Token{
		Hash:        hash,
		User:        user,
		Role:        role,
		Description: description,
		RoleHash:    hashWithRole,
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
