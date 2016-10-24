package easyauth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"

	"github.com/gorilla/securecookie"
)

type Option func(*authManager) error

func CookieSecret(s string) Option {
	return func(a *authManager) error {
		var dat []byte
		var err error
		//if valid b64, use that. best practice is a longish random base 64 string
		if dat, err = base64.StdEncoding.DecodeString(s); err != nil {
			dat = []byte(s)
		}
		if len(dat) < 8 {
			return fmt.Errorf("Cookie secret is too small. Recommend 64 bytes in base 64 encoded string.")
		}
		var hashKey, blockKey []byte
		if len(dat) == 64 {
			hashKey, blockKey = dat[:32], dat[32:]
		} else {
			split := len(dat) / 2
			h, e := sha256.Sum256(dat[split:]), sha256.Sum256(dat[:split])
			hashKey, blockKey = h[:], e[:]
		}
		a.cookie.sc = securecookie.New(hashKey, blockKey)
		return nil
	}
}

func CookieDuration(seconds int) Option {
	return func(a *authManager) error {
		a.cookie.duration = seconds
		return nil
	}
}

func LoginTemplate(t string) Option {
	return func(a *authManager) error {
		tmpl, err := template.New("login").Parse(t)
		if err != nil {
			return err
		}
		a.loginTemplate = tmpl
		return nil
	}
}
