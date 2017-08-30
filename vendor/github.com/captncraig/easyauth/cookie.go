package easyauth

import (
	"net/http"

	"github.com/gorilla/securecookie"
)

type CookieManager struct {
	sc       *securecookie.SecureCookie
	duration int
}

func (cm *CookieManager) ReadCookie(r *http.Request, name string, maxAge int, dst interface{}) error {
	val, err := cm.ReadCookiePlain(r, name)
	if err != nil {
		return err
	}
	if err = cm.sc.MaxAge(maxAge).Decode(name, val, dst); err != nil {
		return err
	}
	return nil
}

func (cm *CookieManager) SetCookie(w http.ResponseWriter, name string, maxAge int, dat interface{}) error {
	if maxAge == 0 {
		maxAge = cm.duration
	}
	val, err := cm.sc.MaxAge(maxAge).Encode(name, dat)
	if err != nil {
		return err
	}
	cm.SetCookiePlain(w, name, maxAge, val)
	return nil
}

func (cm *CookieManager) SetCookiePlain(w http.ResponseWriter, name string, maxAge int, value string) {
	if maxAge == 0 {
		maxAge = cm.duration
	}
	cookie := &http.Cookie{
		MaxAge:   maxAge,
		HttpOnly: true,
		Name:     name,
		Path:     "/",
		//Secure:   true,
		Value: value,
	}
	http.SetCookie(w, cookie)
}

func (cm *CookieManager) ReadCookiePlain(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err //cookie no exist
	}
	return cookie.Value, nil
}

func (cm *CookieManager) ClearCookie(w http.ResponseWriter, name string) {
	cm.SetCookiePlain(w, name, -1, "")
}
