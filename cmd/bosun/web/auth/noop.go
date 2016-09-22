package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

//NoAuth is an auth provider implementation that provides no access control. It will read username from json data in request
type NoAuth struct{}

var _ Provider = NoAuth{}

func (n NoAuth) GetUser(r *http.Request) (*User, error) {
	type hasUser struct {
		User string
	}
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(buf))
	user := &hasUser{}
	if err = json.Unmarshal(buf, user); err != nil {
		return nil, err
	}
	return &User{
		Name:        user.User,
		AuthMethod:  "json",
		Permissions: Admin,
	}, nil
}

func (n NoAuth) LoginHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "No auth configured. Invalid route.")
	})
}
