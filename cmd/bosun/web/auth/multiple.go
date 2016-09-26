package auth

import (
	"net/http"
)

//MultipleProviders is a simple wrapper to allow multiple auth providers.
//providers will be called in order for each request until one of them returns a valid user or an error
//For the login handler, it will use only the first provider.
type MultipleProviders []Provider

func (m MultipleProviders) GetUser(r *http.Request) (*User, error) {
	for _, p := range m {
		u, err := p.GetUser(r)
		if err != nil {
			return nil, err
		}
		if u != nil {
			return u, nil
		}
	}
	return nil, nil
}

func (m MultipleProviders) LoginHandler() http.Handler {
	return m[0].LoginHandler()
}
