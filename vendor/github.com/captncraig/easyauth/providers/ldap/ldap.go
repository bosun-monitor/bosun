package ldap

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/captncraig/easyauth"
	ldap "gopkg.in/ldap.v1"
)

type LdapProvider struct {
	//name of domain
	Domain string
	//user base dn (for LDAP Auth)
	UserBaseDn string
	//server to query
	LdapAddr string
	//if untrusted certs should be allowed
	AllowInsecure bool
	//Permissions granted to any user who successfully authenticates
	DefaultPermission easyauth.Role
	//List of groups to grant additional permissions
	Groups []*LdapGroup
	//Individual user permissions
	Users map[string]easyauth.Role
	//Root search path to check group memberships. Ex "DC=myorg,DC=com"
	RootSearchPath string

	//Name to use for cookie
	CookieName string

	// Function to call on successful login. Can change user data or roles if desired.
	// return non-nil error to deny the login
	OnLogin func(u *easyauth.User) error

	OnLoginFail func(string)
}

//ensure at compile time we implement the interfaces we intend to
var _ easyauth.Logoutable = (*LdapProvider)(nil)
var _ easyauth.FormProvider = (*LdapProvider)(nil)

type LdapGroup struct {
	Path string
	Role easyauth.Role
}

func (l *LdapProvider) cookieName() string {
	if l.CookieName != "" {
		return l.CookieName
	}
	return "ldap-auth"
}

func (l *LdapProvider) GetUser(r *http.Request) (*easyauth.User, error) {
	u := &easyauth.User{}
	err := easyauth.GetCookieManager(r).ReadCookie(r, l.cookieName(), 0, u)
	if err != nil {
		if err == http.ErrNoCookie {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (l *LdapProvider) Logout(w http.ResponseWriter, r *http.Request) {
	easyauth.GetCookieManager(r).ClearCookie(w, l.cookieName())
}

func (l *LdapProvider) GetRequiredFields() []string {
	return []string{"Username", "Password"}
}

func (l *LdapProvider) HandlePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		panic("Error with request")
	}
	un := r.FormValue("Username")
	if un == "" {
		panic("Username may not be empty")
	}
	pw := r.FormValue("Password")
	if pw == "" {
		panic("Password may not be empty")
	}
	role := l.Authorize(un, pw)
	//success! drop cookie and redirect to content
	user := &easyauth.User{
		Username: un,
		Method:   "ldap",
		Access:   role,
	}
	if l.OnLogin != nil {
		err := l.OnLogin(user)
		if err != nil {
			panic("Error from login callback")
		}
	}
	easyauth.GetCookieManager(r).SetCookie(w, l.cookieName(), 0, user)
	easyauth.GetRedirector(r)()
}

func (l *LdapProvider) Authorize(un, pw string) easyauth.Role {

	var fullUn string
	var auth_ldap bool

	if l.UserBaseDn != "" {
		// prepare LDAP user bind dn
		fullUn = "uid=" + un + "," + l.UserBaseDn
		auth_ldap = true
	} else {
		// prepare AD user domain
		fullUn = l.Domain + "\\" + un
	}

	conn, err := ldap.DialTLS("tcp", l.LdapAddr, &tls.Config{
		InsecureSkipVerify: l.AllowInsecure,
	})
	if err != nil {
		log.Println(err)
		panic("Error connecting to ldap server")
	}
	defer conn.Close()
	err = conn.Bind(fullUn, pw)
	if err != nil {
		log.Println(err)
		if l.OnLoginFail != nil {
			l.OnLoginFail(un)
		}
		panic("Invalid Credentials")
	}
	var role = l.DefaultPermission
	for _, group := range l.Groups {
		if group.Path == "*" {
			role |= group.Role
		}
		isMember, err := checkGroupMembership(un, conn, l.RootSearchPath, map[string]bool{}, []string{group.Path}, auth_ldap)
		if err != nil {
			log.Println("Error checking group membership", err)
			panic("Error checking group memberships")
		}
		if isMember {
			role |= group.Role
		}
	}
	if l.Users != nil {
		role |= l.Users[un] //not present is noop (or with 0)
	}
	return role
}

func checkGroupMembership(un string, conn *ldap.Conn, rootSearch string, alreadySearched map[string]bool, groups []string, auth_ldap bool) (isMember bool, err error) {
	// Implementation of recursive group membership search. There is a magic AD key that kinda does this, but this may be more reliable.
	grps := make([]string, len(groups))
	for i, g := range groups {
		alreadySearched[g] = true
		grps[i] = fmt.Sprintf("(memberof=%s)(member=%s)", g, g)
	}

	// map user key for LDAP it will be cn
	var user_id string
	if auth_ldap {
		user_id = "cn"
	} else {
		user_id = "sAMAccountName"
	}

	//find all users or groups that are direct members of ANY of the given groups
	searchRequest := ldap.NewSearchRequest(
		rootSearch,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(|(objectClass=user)(objectClass=group)(objectClass=groupOfNames))(|%s))", strings.Join(grps, "")),
		[]string{"dn", "objectClass", user_id},
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

		//for LDAP group member check only the name
		if auth_ldap {
			if acctName == un {
				return true, nil
			}
		}

	}
	if len(nextSearches) > 0 {
		return checkGroupMembership(un, conn, rootSearch, alreadySearched, nextSearches, auth_ldap)
	}
	return false, nil
}
