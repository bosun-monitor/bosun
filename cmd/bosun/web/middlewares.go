package web

import (
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/captncraig/easyauth"
	"github.com/captncraig/easyauth/providers/ldap"
	"github.com/captncraig/easyauth/providers/token"
	"github.com/captncraig/easyauth/providers/token/redisStore"
	"github.com/gorilla/mux"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/collect"
	"bosun.org/opentsdb"
)

// This file contains custom middlewares for bosun. Must match alice.Constructor signature (func(http.Handler) http.Handler)

var miniProfilerMiddleware = func(next http.Handler) http.Handler {
	return miniprofiler.NewContextHandler(next.ServeHTTP)
}

var endpointStatsMiddleware = func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//metric for http vs https
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		collect.Add("http_protocol", opentsdb.TagSet{"proto": proto}, 1)

		//if we use gorilla named routes, we can add stats and timings per route
		routeName := ""
		if route := mux.CurrentRoute(r); route != nil {
			routeName = route.GetName()
		}
		if routeName == "" {
			routeName = "unknown"
		}
		t := collect.StartTimer("http_routes", opentsdb.TagSet{"route": routeName})
		next.ServeHTTP(w, r)
		t()
	})
}

type noopAuth struct{}

func (n noopAuth) GetUser(r *http.Request) (*easyauth.User, error) {
	name := "anonymous"
	if cookie, err := r.Cookie("action-user"); err == nil {
		name = cookie.Value
	}
	//everybody is an admin!
	return &easyauth.User{
		Access:   roleAdmin,
		Username: name,
		Method:   "noop",
	}, nil
}

func buildAuth(cfg *conf.AuthConf) (easyauth.AuthManager, *token.TokenProvider, error) {
	if cfg == nil {
		auth, err := easyauth.New()
		if err != nil {
			return nil, nil, err
		}
		auth.AddProvider("nop", noopAuth{})
		return auth, nil, nil
	}
	const defaultCookieSecret = "CookiesAreInsecure"
	if cfg.CookieSecret == "" {
		cfg.CookieSecret = defaultCookieSecret
	}
	auth, err := easyauth.New(easyauth.CookieSecret(cfg.CookieSecret))
	if err != nil {
		return nil, nil, err
	}
	if cfg.AuthDisabled {
		auth.AddProvider("nop", noopAuth{})
	} else {
		authEnabled = true
	}
	if cfg.LDAP.LdapAddr != "" {
		l, err := buildLDAPConfig(cfg.LDAP)
		if err != nil {
			return nil, nil, err
		}
		auth.AddProvider("ldap", l)
	}
	var authTokens *token.TokenProvider
	if cfg.TokenSecret != "" {
		tokensEnabled = true
		authTokens = token.NewToken(cfg.TokenSecret, redisStore.New(schedule.DataAccess))
		auth.AddProvider("tok", authTokens)
	}
	return auth, authTokens, nil
}

func buildLDAPConfig(ld conf.LDAPConf) (*ldap.LdapProvider, error) {
	l := &ldap.LdapProvider{
		Domain:         ld.Domain,
		UserBaseDn:     ld.UserBaseDn,
		LdapAddr:       ld.LdapAddr,
		AllowInsecure:  ld.AllowInsecure,
		RootSearchPath: ld.RootSearchPath,
		Users:          map[string]easyauth.Role{},
	}
	var role easyauth.Role
	var err error
	if role, err = parseRole(ld.DefaultPermission); err != nil {
		return nil, err
	}
	l.DefaultPermission = role
	for _, g := range ld.Groups {
		if role, err = parseRole(g.Role); err != nil {
			return nil, err
		}
		l.Groups = append(l.Groups, &ldap.LdapGroup{Path: g.Path, Role: role})
	}
	for name, perm := range ld.Users {
		if role, err = parseRole(perm); err != nil {
			return nil, err
		}
		l.Users[name] = role
	}
	return l, nil
}
