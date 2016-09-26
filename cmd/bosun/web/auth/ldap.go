package auth

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"bosun.org/slog"
	"github.com/gorilla/securecookie"
	ldap "gopkg.in/ldap.v1"
)

type ldapProvider struct {
	LdapAddr       string
	Groups         []*LdapGroup
	Domain         string
	RootSearchPath string
	s              *securecookie.SecureCookie
}

type LdapGroup struct {
	Level PermissionLevel
	Path  string
}

type byPermission []*LdapGroup

func (a byPermission) Len() int           { return len(a) }
func (a byPermission) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPermission) Less(i, j int) bool { return a[i].Level < a[j].Level }

const ldapCookieName = "ldap-auth"

func NewLdap(addr string, domain string, groups []*LdapGroup, rootSearch, secret string) Provider {
	//derive hash and block keys from secret string
	hkey := sha256.Sum256([]byte(secret))
	bkey := sha256.Sum256([]byte(hkey[:]))
	s := securecookie.New(hkey[:], bkey[:])
	s.SetSerializer(securecookie.JSONEncoder{})
	sort.Sort(sort.Reverse(byPermission(groups)))
	return &ldapProvider{
		LdapAddr:       addr,
		Groups:         groups,
		Domain:         domain,
		RootSearchPath: rootSearch,
		s:              s,
	}
}

func (l *ldapProvider) GetUser(r *http.Request) (*User, error) {
	if cookie, err := r.Cookie(ldapCookieName); err == nil {
		u := &User{}
		if err = l.s.Decode(ldapCookieName, cookie.Value, u); err == nil {
			return u, nil
		}
	}
	return nil, nil
}

func (l *ldapProvider) LoginHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg = ""
		if r.Method == "POST" { // postback
			if err := r.ParseForm(); err == nil {
				un, pw := r.FormValue("username"), r.FormValue("password")
				perms, err := l.Authorize(un, pw)
				if err != nil {
					msg = err.Error()
				} else {
					l.DropCookie(un, perms, w)
					targetURL := r.FormValue("u")
					if targetURL == "" {
						targetURL = "/"
					}
					http.Redirect(w, r, targetURL, http.StatusFound)
				}
			} else {
				msg = fmt.Sprintf("Error with login: %s", err)
			}
		}
		// login form html
		w.Header().Set("Content-Type", "text/html")
		ldapLoginForm.Execute(w, map[string]interface{}{"Msg": msg})

	})
}

func (l *ldapProvider) DropCookie(un string, perms PermissionLevel, w http.ResponseWriter) {
	u := &User{
		Name:        un,
		Permissions: perms,
		AuthMethod:  "LDAP",
	}
	dat, err := l.s.Encode(ldapCookieName, u)
	if err != nil {
		slog.Error(err)
		return
	}
	cookie := &http.Cookie{
		Name:    ldapCookieName,
		Value:   dat,
		Path:    "/",
		Expires: time.Now().Add(14 * time.Hour * 24), // 14 days sounds good
		//Secure:   true,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

func (l *ldapProvider) Authorize(un, pw string) (PermissionLevel, error) {
	fullUn := l.Domain + "\\" + un
	conn, err := ldap.DialTLS("tcp", l.LdapAddr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error(err)
		return None, fmt.Errorf("Error connecting to ldap server")
	}
	defer conn.Close()
	fmt.Println(fullUn, "fullUN")
	err = conn.Bind(fullUn, pw)
	if err != nil {
		slog.Error(err)
		return None, fmt.Errorf("Invalid Credentials")
	}
	if len(l.Groups) == 0 {
		//no groups specified. Assume anyone who can bind is an admin
		return Admin, nil
	}
	for _, group := range l.Groups {
		if group.Path == "*" {
			return group.Level, nil
		}
		isMember, err := checkGroupMembership(un, conn, l.RootSearchPath, map[string]bool{}, []string{group.Path})
		if err != nil {
			slog.Error("Error checking group membership", err)
			continue
		}
		if isMember {
			return group.Level, nil
		}
	}
	// not in any appropriate groups
	return None, fmt.Errorf("Invalid Credentials")
}

func checkGroupMembership(un string, conn *ldap.Conn, rootSearch string, alreadySearched map[string]bool, groups []string) (isMember bool, err error) {
	// Implementation of recursive group membership search. There is a magic AD key that kinda does this, but this may be more reliable.
	grps := make([]string, len(groups))
	for i, g := range groups {
		alreadySearched[g] = true
		grps[i] = fmt.Sprintf("(memberof=%s)", g)
	}
	//find all users or groups that are direct members of ANY of the given groups
	searchRequest := ldap.NewSearchRequest(
		rootSearch,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(|(objectClass=user)(objectClass=group))(|%s))", strings.Join(grps, "")),
		[]string{"dn", "objectClass", "sAMAccountName"},
		nil,
	)
	sr, err := conn.Search(searchRequest)
	if err != nil {
		return false, err
	}
	nextSearches := []string{}
	for _, ent := range sr.Entries {
		objType := ent.Attributes[0].Values[1]
		acctName := ent.Attributes[1].Values[0]
		//a group we have not seen. Add it to the next search batch.
		if objType == "group" && !alreadySearched[ent.DN] {
			nextSearches = append(nextSearches, ent.DN)
		}
		//a user with the correct name. We found it!
		if objType == "person" && acctName == un {
			return true, nil
		}
	}
	if len(nextSearches) > 0 {
		return checkGroupMembership(un, conn, rootSearch, alreadySearched, nextSearches)
	}
	return false, nil
}

// login form and css stolen and adapted from getbootstrap.com/examples/signin/
var ldapLoginForm = template.Must(template.New("").Parse(`
<html>
<head>
<link href="/static/css/bootstrap.min.css" rel="stylesheet">
<style>
body {
  padding-top: 40px;
  padding-bottom: 40px;
  background-color: #eee;
}

.form-signin {
  max-width: 330px;
  padding: 15px;
  margin: 0 auto;
}
.form-signin .form-signin-heading,
.form-signin .checkbox {
  margin-bottom: 10px;
}
.form-signin .checkbox {
  font-weight: normal;
}
.form-signin .form-control {
  position: relative;
  height: auto;
  -webkit-box-sizing: border-box;
     -moz-box-sizing: border-box;
          box-sizing: border-box;
  padding: 10px;
  font-size: 16px;
}
.form-signin .form-control:focus {
  z-index: 2;
}
.form-signin input[type="text"] {
  margin-bottom: -1px;
  border-bottom-right-radius: 0;
  border-bottom-left-radius: 0;
}
.form-signin input[type="password"] {
  margin-bottom: 10px;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
}
</style>
</head>
<body>
<div class="container">
      {{if .Msg}}<div class="alert alert-danger" role="alert">{{.Msg}}</div>{{end}}
      <form class="form-signin" method="post">
        <h2 class="form-signin-heading">Please sign in</h2>
        <label for="inputUn" class="sr-only">Username</label>
        <input type="text" id="inputUn" name="username" class="form-control" placeholder="Username" required autofocus>
        <label for="inputPassword" class="sr-only">Password</label>
        <input type="password" id="inputPassword" name="password" class="form-control" placeholder="Password" required>
        <button class="btn btn-lg btn-primary btn-block" type="submit">Sign in</button>
      </form>
    </div> 
</body>
</html>
`))
