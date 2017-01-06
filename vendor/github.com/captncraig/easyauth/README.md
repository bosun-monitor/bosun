# easyauth
This package aims to make authentication in go apps as simple as possible. It currently supports the following authentication methods:

- LDAP
- Api Tokens
- Oauth (soon)
- Custom user database (soon)

It provides an end-to-end solution for integrating any or all of these providers into your app, including:

- Access control middleware (compatible with almost any web framework)
- Secure session cookie management
- Fine grained, endpoint level access control
- All needed http handlers, callbacks, login pages, etc. generated for you
- Full customizability

## Usage

### Role based access-control
easyauth uses a role-based permission system to control access to resources. Your application can define whatever roles and access levels that are appropriate. A typical setup may look like this:

```
const(
    //capabilities are bit flags. Content usually requires specific capabilities/
    CanRead easyauth.Role = 1 << iota
    CanWrite
    CanModerate
    CanViewSystemStats
    CanChangeSettings

    //roles combine flags. Users have an associated role value
    RoleReader = CanRead
    RoleWriter = RoleReader | CanWrite
    RoleMod = RoleWriter | CanModerate
    RoleAdmin = RoleMod | CanViewSystemStats | CanChangeSettings 
)
```

### Getting Started

`import "github.com/captncraig/easyauth"`

1. Create a new `AuthManager`.

  ```
  auth := easyAuth.New(...)
  ```
  This call accepts any number of option modifiers. See [options](#options) for full options documentation.

2. Add Providers:

  Here I will add an ldap provider:

  ```
l := &ldap.LdapProvider{
    LdapAddr:          "ad.myorg.com:3269",
    DefaultPermission: RoleReader,
    Domain:            "MYORG",
}
auth.AddProvider("ldap", l)
  ```

  Anyone who enters valid credentials will be granted the "Reader" role as defined by my app's role structure.

  You can add as many different providers as you wish. See [providers](#providers) for detailed info on how to use and configure indivisual providers.

3. Register auth handler:

  ```
http.Handle("/auth/", http.StripPrefix("/auth", auth.LoginHandler()))
  ```

  This handler will handle all requests to `/auth/*` and handle them as appropriate. This include login pages,
callbacks, form posts, logout requests, deny pages and so forth.

4. Apply middeware to your app's http handlers:
  ```
http.Handle("/api/stats", auth.Wrap(myStatHandler,CanViewSystemStats))
  ```
  Each handler can specify their own requirements for user capabilities to access that content.

### options

- `CookieSecret("superSecretString")`: set the secret used to hash and encrypt all cookies made by the system. Can be any string, but I recommend using 64 bytes of random data, base64 encoded.
You can generate a suitable secret with this command: `go test github.com/captncraig/easyauth -run TestGenerateKey -v`
- `CookieDuration(int seconds)`: Set the default duration for all session cookies. Defaults to 30 days.
- `LoginTemplate(tmpl string)`: Override the [built-in](https://github.com/captncraig/easyauth/blob/master/template.go) template for the login page. The context is a bit tricky to support multiple types of providers, but the example should serve as a decent model. If your app has a known set of providers/options, then a custom login page may be much easier.
