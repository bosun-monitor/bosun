package easyauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

//Role is a number representing a permission level.
//Specific roles will be defined by the host application.
//It is intended to be a bitmask.
//Each user will have a certain permission level associated, as can any endpoint or content.
//If the user permissions and the content requirement have any common bits (user & content != 0), then access will be granted.
type Role uint32

type User struct {
	Username string
	Access   Role
	Method   string
	Data     interface{}
}

//AuthProvider is any source of user authentication. These core methods must be implemented by all providers.
type AuthProvider interface {
	//Retreive a user from the current http request if present.
	GetUser(r *http.Request) (*User, error)
}

//FormProvider is a provider that accepts login info from an html form.
type FormProvider interface {
	AuthProvider
	GetRequiredFields() []string
	HandlePost(http.ResponseWriter, *http.Request)
}

//HTTPProvider is a provider that provides an http handler form managing its own login endpoints, for example oauth.
//Will receive all calls to /{providerName}/*
type HTTPProvider interface {
	AuthProvider
	http.Handler
}

type Logoutable interface {
	//Logout allows the provider to delete any relevant cookies or session data in order to log the user out.
	//The provider should not otherwise write to the response, or redirect
	Logout(w http.ResponseWriter, r *http.Request)
}

type AuthManager interface {
	AddProvider(string, AuthProvider)
	LoginHandler() http.Handler
	Wrapper(required Role) func(http.Handler) http.Handler
	Wrap(next http.Handler, required Role) http.Handler
	WrapFunc(next http.HandlerFunc, required Role) http.Handler
}

type namedProvider struct {
	Name     string
	Provider AuthProvider
}

type namedFormProvider struct {
	Name     string
	Provider FormProvider
}

type namedHTTPProvider struct {
	Name     string
	Provider HTTPProvider
}

type authManager struct {
	Providers     []namedProvider
	FormProviders []namedFormProvider
	HTTPProviders []namedHTTPProvider
	names         map[string]bool
	cookie        *CookieManager
	loginTemplate *template.Template
}

func New(opts ...Option) (AuthManager, error) {
	var mgr = &authManager{
		names: map[string]bool{},
		cookie: &CookieManager{
			duration: int(time.Hour * 24 * 30),
		},
	}
	for _, opt := range opts {
		if err := opt(mgr); err != nil {
			return nil, err
		}
	}
	if mgr.loginTemplate == nil {
		mgr.loginTemplate = template.Must(template.New("login").Parse(loginTemplate))
	}
	return mgr, nil
}

func (m *authManager) AddProvider(name string, p AuthProvider) {
	if _, ok := m.names[name]; ok {
		panic(fmt.Errorf("Auth provider %s registered multiple times", name))
	}
	m.names[name] = true
	m.Providers = append(m.Providers, namedProvider{name, p})
	if form, ok := p.(FormProvider); ok {
		m.FormProviders = append(m.FormProviders, namedFormProvider{name, form})
	}
	if httpp, ok := p.(HTTPProvider); ok {
		m.HTTPProviders = append(m.HTTPProviders, namedHTTPProvider{name, httpp})
	}

}

func (m *authManager) LoginHandler() http.Handler {
	mx := mux.NewRouter()
	mx.Path("/").Methods("GET").HandlerFunc(m.loginPage)
	mx.Path("/out").Methods("GET").HandlerFunc(m.logout)
	mx.Path("/deny").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><h1>Access denied</h1>You do not have access to the requested content."))
	})
	for _, form := range m.FormProviders {
		mx.Path(fmt.Sprintf("/%s", form.Name)).Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.postForm(w, r, form.Provider)
		})
	}
	for _, httpp := range m.HTTPProviders {
		mx.PathPrefix(fmt.Sprintf("/%s/", httpp.Name)).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.delegateHTTP(w, r, httpp.Provider)
		})
	}
	return mx
}

func (m *authManager) buildContext(w http.ResponseWriter, r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), cookieContextKey, m.cookie)
	ctx = context.WithValue(ctx, redirectContextKey, func() {
		m.redirect(w, r)
	})
	return r.WithContext(ctx)
}

//Wrapper returns a middleware constructor for the given auth level. This function can be used with middleware chains
//or by itself to create new handlers in the future
func (m *authManager) Wrapper(required Role) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return m.Wrap(h, required)
	}
}

func (m *authManager) Wrap(next http.Handler, required Role) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orig := r
		r = m.buildContext(w, r)
		var u *User
		var err error
		for _, p := range m.Providers {
			u, err = p.Provider.GetUser(r)
			if err != nil {
				log.Println(err)
				continue
			}
			if u != nil {
				r = orig //don't send intenral context to real handler
				r = r.WithContext(context.WithValue(r.Context(), userContextKey, u))
				break
			}
		}
		if required == 0 {
			//no permission needed
			next.ServeHTTP(w, r)
		} else if u == nil {
			//logged out. redir to login
			if strings.Contains(r.Header.Get("Accept"), "html") {
				m.cookie.SetCookiePlain(w, redirCookirName, 10*60, r.URL.String())
				http.Redirect(w, r, "/login/", http.StatusFound) //todo: configure this
			} else {
				http.Error(w, "Access Denied", http.StatusForbidden)
			}
		} else if u.Access&required == 0 {
			//denied for permissions
			if strings.Contains(r.Header.Get("Accept"), "html") {
				m.cookie.SetCookiePlain(w, redirCookirName, 10*60, r.URL.String())
				http.Redirect(w, r, "/login/deny", http.StatusFound)
			} else {
				http.Error(w, "Access Denied", http.StatusForbidden)
			}
		} else {
			//has permission
			next.ServeHTTP(w, r)
		}
	})
}

func (m *authManager) WrapFunc(next http.HandlerFunc, required Role) http.Handler {
	return m.Wrap(next, required)
}

const (
	errMsgCookieName = "errMsg"
	redirCookirName  = "redirTo"
)

func (m *authManager) loginPage(w http.ResponseWriter, r *http.Request) {
	msg, err := m.cookie.ReadCookiePlain(r, errMsgCookieName)
	if err == nil {
		m.cookie.ClearCookie(w, errMsgCookieName)
	}
	var ctx = map[string]interface{}{
		"Auth":    m,
		"Message": msg,
	}
	if err := m.loginTemplate.Execute(w, ctx); err != nil {
		log.Printf("Error executing login template: %s", err)
	}
}

func (m *authManager) logout(w http.ResponseWriter, r *http.Request) {
	r = m.buildContext(w, r)
	for _, p := range m.Providers {
		if lo, ok := p.Provider.(Logoutable); ok {
			lo.Logout(w, r)
		}
	}
	http.Redirect(w, r, "/", 302)
}

type contextKeyType int

const (
	cookieContextKey contextKeyType = iota
	redirectContextKey
	userContextKey
)

func (m *authManager) delegateHTTP(w http.ResponseWriter, r *http.Request, h HTTPProvider) {
	r = m.buildContext(w, r)
	h.ServeHTTP(w, r)
}

func (m *authManager) redirect(w http.ResponseWriter, r *http.Request) {
	path := "/"
	stored, err := m.cookie.ReadCookiePlain(r, redirCookirName)
	if err == nil {
		path = stored
		m.cookie.ClearCookie(w, redirCookirName)
	}
	http.Redirect(w, r, path, http.StatusFound)
}

func (m *authManager) postForm(w http.ResponseWriter, r *http.Request, p FormProvider) {
	defer func() {
		if rc := recover(); rc != nil {
			//set short-lived cookie with message and redirect to login page
			m.cookie.SetCookiePlain(w, errMsgCookieName, 60, fmt.Sprint(rc))
			http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		}
	}()
	r = m.buildContext(w, r)
	p.HandlePost(w, r)
}

func GetCookieManager(r *http.Request) *CookieManager {
	return r.Context().Value(cookieContextKey).(*CookieManager)
}
func GetRedirector(r *http.Request) func() {
	return r.Context().Value(redirectContextKey).(func())
}

func GetUser(r *http.Request) *User {
	u := r.Context().Value(userContextKey)
	if u == nil {
		return nil
	}
	return u.(*User)
}

//RandomString returns a random string of bytes length long, base64 encoded.
func RandomString(length int) string {
	var dat = make([]byte, length)
	rand.Read(dat)
	return base64.StdEncoding.EncodeToString(dat)
}
