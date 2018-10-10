---
layout: default
title: Configuration
order: 3
---

<div class="row">
<div class="col-sm-3" >
  <div class="sidebar" data-spy="affix" data-offset-top="0" data-offset-bottom="0" markdown="1">

 * Some TOC
 {:toc}

  </div>
</div>

<div class="doc-body col-sm-9" markdown="1">

<p class="title h1">{{page.title}}</p>

## Changes Since 0.5.0
Since 0.5.0, the config has been split into two different files.

### System
System config is documented on the rest of this page, and has been moved
into `bosun.toml` unless overriden on the command line. Changes to these
settings require a Bosun restart in order to take effect. The TOML
format is documented [here](https://github.com/toml-lang/toml).

The [example config
file](https://github.com/bosun-monitor/bosun/blob/master/cmd/bosun/bosun.example.toml)
is a good place to start when writing a configuration.

<div class="admonition">
<p class="admonition-title">Warning</p>
<p>The file does <em>not</em> follow the tradition of commenting out defaults,
which are instead mostly documented below.</p>
</div>

### Definitions
This file is documented [here](/definitions). It includes settings that
do not require a Bosun restart to take effect e.g. alerts, templates,
notifications.

## Keys

### Hostname
The hostname that Bosun uses to construct its links. The common use case
is in any [template functions](/definitions#template-functions) that
construct links.

Example:
`Hostname = "bosun.example.com"`

### Scheme
The URL that Bosun uses to construct its links. The common use case
is in any [template functions](/definitions#template-functions) that
construct links. the default is "http"

Example:
`Scheme = "https"`

### HTTPListen
HTTP IP and Port to listen on. The default if not specified is to
listen on `:8070`. However, if `HTTPSListen` is defined and `HTTPListen`
is not defined there will be no default, and Bosun will only listen on
HTTPS.

Example:
`HTTPListen = ":8080"`

### HTTPSListen
HTTPS (SSL) IP and Port to listen on. This requires that `TLSCertFile` and `TLSKeyFile` are defined.

### TLSCertFile
Path to the PEM encoded certificate Bosun should use for
`HTTPSListen`. May contain intermediate certificates followed by the
leaf certificate to form a chain.

Example:
`TLSCertFile = "/etc/ssl/certs/bosun.example.com.crt"`

### TLSKeyFile
Path to the PEM encoded private key that corresponds to the `TLSCertFile`.

Example:
`TLSKeyFile = "/etc/ssl/certs/bosun.example.com.key"`

### CheckFrequency
Minimum interval that alert checks will run at on a schedule. The format
of the value is the same as [Go's duration
format](https://golang.org/pkg/time/#Duration.String). By default, alert
checks are run at every `CheckFrequency` multiplied by the
`DefaultRunEvery` value. This defaults to "5m".

Example:
`CheckFrequency = "1m"`

### DefaultRunEvery
By default, alert checks are run at every
[`CheckFrequency`](/system_configuration#checkevery) multiplied by the
`DefaultRunEvery` value. This can be overridden in an alert definition
with the [`runEvery` keyword](/definitions#runevery). This defaults to
1.

e.g. If `CheckFrequency = 1m` and a `DefaultRunEvery = 5`, alerts by
default will run every 5 minutes. But you could have some run as
frequent as every "1m", and others that run less often (any multiple of
"1m").

Example:
`DefaultRunEvery = 5`

### AlertCheckDistribution
Selects algorithm to distribute alert checks to decrease system load spikes.  There is no distribution by default. This means, if there are several checks with same period, they all will happen at the same points in time. This method is used if the option is not specified or equals to empty string.

The single alternative option is `simple`. If specified, the alert checks with the same period will be uniformly distributed on second marks.

Example: `AlertCheckDistribution = "simple"`

### RuleFilePath
Path to the file containing definitions of alerts, macros, lookups,
templates, notifications, and global variables which are [documented
here](/definitions). If saving is enabled, this file can be written to
by Bosun via the API or [Save UI](/usage#definition-rule-saving).
Mandatory.

Example: `RuleFilePath = "dev.sample.conf"`

### MaxRenderedTemplateAge
If set, this will allow bosun to delete rendered templates from its' data store.
It will remove all rendered templates for alerts that have been closed for longer than this time (in days).

Example: `MaxRenderedTemplateAge = 30 # retain old templates for only 30 days`

### TimeAndDate
Used to configure time zones that will be linked to in Bosun's
dashboard. It is an array of timeanddate.com zones (the page that gets
linked to from Bosun's UI.) It has no impact on what time zone Bosun
operates in. Bosun is expected to use UTC and does support other
timezones.

Example:
`TimeAndDate = [ 202, 75, 179, 136 ]`

### ShortURLKey
Bosun's UI can generate short links using Google's goo.gl URL Shortener service. If you are hitting their API limits, you can get an API key and specify here, and that key will get used.

Example:
`ShortURLKey = "aKey"`

### MinGroupSize
Bosun's dashboard will use grouping logic on the dashboard if there are many similar alerts (in name, or tag/key pairs). `MinGroupSize` sets the minimum number of alerts needed to create a group in the UI and defaults to `5`.

If you have a lot of grouping, it often means you should refactor the alert to have a less granular scope. Therefore, it is recommended that this is used as a "protection mechanism" from flooding the dashboard with too many alerts.

Example: `MinGroupSize = 5`

### Unknown Threshold
Bosun will group all unknowns in a single check cycle (alerts on the same [`CheckFrequency`](/system_configuration#checkfrequency) and [`RunEvery`](/system_configuration#defaultrunevery)) into a single email. This sets how many unknowns would be sent in a single check cycle before a group is created. The default value is 5.

This is done because unknowns are generally caused by the data "disappearing". So if your TSDB Provider is broken or behind, it can generate a lot of unknowns. This alleviates flooding in the specific case of unknowns.

Example: `UnknownThreshold = 5`

### Ping
If set to `true`, Bosun will ping every value of the host tag that it has indexed and record that value to your TSDB. It currently only support OpenTSDB style data input, which is means you must use either OpenTSDB or Influx with the OpenTSDB endpoint on Influx configured.

Example:
`Ping = true`

### PingDuration
How long Bosun should wait before stopping to ping host tags it has
seen. e.g. If the value is the default of `"24h"`, if Bosun has
not indexed any data points for that `host` value, then it will stop
attempting to ping that host until it again sees data points that have that
tag.

Example:
`PingDuration = "24h"`

### SearchSince
Controls how long autocomplete and items in UI will show up since being indexed by Bosun. The format of the value is the same as [Go's duration format](https://golang.org/pkg/time/#Duration.String) and the default is 3 days. The goal is to make it so you don't have old items showing up in the UI. However, if you are using OpenTSDB and graphing page, you can still query metrics that don't autocomplete if you remember what they were (or look them up using OpenTSDB's native UI autocomplete).

Example: `SearchSince = "72h"`

### EnableSave
Enables saving via the web UI. It is disabled by default. When enabled,
users will be able to save the rule configuration file via the UI and
Bosun will then write to that file on the user's behalf.

Example: `EnableSave = true`

### ReloadEnabled
Sets if reloading of the rule configuration should be enabled. If
`EnableSave` is `true`, then reloading gets enabled regardless of this
setting. Reloads can be triggered via [`/api/reload` in the REST
API](/api#apireload) as well.

Example:
`EnableSave = true`

### CommandHookPath
When enabling saving, and a user issues a save, you have the option to run a executable or script by specifying this parameter. This allows you to do things like backup the file on writes or commit the file to a git repo.

This command is passed a filename, username, message, and vargs (vargs is currently not used). If the command exits a non-zero exit code, then the changes will be reverted (the file before the changes is copied back and Bosun doesn't restart). When the configuration is saved via the user interface, any messages to standard error will be shown to the user when there is a non-zero exit code.

Example:
`CommandHookPath = "/Users/kbrandt/src/hook/hook"`

### GetInternetProxy
Current code documentation says:
```
// GetInternetProxy sets a proxy for outgoing network requests from Bosun. Currently it
// only impacts requests made for shortlinks to https://goo.gl/
```
But not sure I trust that.

## Configuration Sections
All your key value pairs must be defined before any sections are defined. Sections are used for things that have multiple values to configure them. In particular the various time series database providers.

### RuleVars
Allows definition of variables that will be turned into [global
variables](/definitions#global-variables) available to definitions under
`$sys.`. This is designed for when you have some secrets you don't want
exposed in the definitions file. The values of these variables *can*
still be accessed from Bosun's rule editor and expression UI. This
merely hides them so you don't accidentally include them in
screenshots or when copying and pasting your config.

Example:

```
[RuleVars]
    CHAT = "https://chat.example.com/1?key=mySecretKey"
    foo = "baz"
```

The above could then be used in the rule configuration like:

```
notification chat {
    post = ${sys.CHAT}
    runOnActions = false
}
```

They can also be accessed in templates with the [`V()` function](/definitions#vstring-string)

### DBConf
Defines what internal storage Bosun should use. There are currently two
choices, a built-in redis-like server called ledis or redis. Redis is
recommended for production setups.

The default is to use ledis. If Both Redis and ledis are defined, Redis will take preference and the ledis configuration will be ignored. Ledis is the default, so if `RedisHost` is not specified ledis will be used even if you have no `DBConf` configuration defined.

#### RedisHost
The Redis hostname and port.

#### RedisDb
Optional integer database to store bosun data.  Defaults to 0.

#### RedisPassword
Optional password to use when connecting to Redis.

#### LedisDir
Directory in which ledis will store data. Default: `LedisDir = "ledis_data"`

#### LedisBind
Ledis host and port. Default: `LedisBindAddr = "127.0.0.1:9565"`.

#### Examples

Redis Configuration:

```
[DBConf]
	RedisHost = "localhost:6389"
```

Ledis Configuration:

```
[DBConf]
	RedisHost = "localhost:6389"
	LedisDir = "ledis_data"
	LedisBindAddr = "127.0.0.1:9565"
```

### SMTPConf
SMTP server configuration for email sending.

#### EmailFrom
Address from which emails will be sent.

#### Host
Outgoing SMTP server hostname or IP address.

#### Username
(TODO: See how this and Password is used with email auth, don't have a current example.)

#### Password

#### Example

```
[SMTPConf]
	EmailFrom = "bosun@example.com"
	Host = "mail.example.com"
```

### AzureMonitorConf
AzureConf enables [Azure Monitor specific functions](/expressions#azure-monitor-query-functions) in the expression language. Multiple clients may be defined allowing you to query different subscriptions and tenants from a single Bosun instance.

#### AzureMonitorConf.default
Default Azure client to use when the Prefix key is absent or is there and set to "default". When ysing multiple clients the string `default` can change to whatever you want to use in expressions to access this particular client.

#### SubscriptionId
The Azure Subscription GUID for the client. See [Getting your Azure Subscription GUID (new portal)](https://blogs.msdn.microsoft.com/mschray/2016/03/18/getting-your-azure-subscription-guid-new-portal/) for instructions on finding this with [Azure's portal](https://portal.azure.com).

#### TenantId
The Azure Tenant GUID for the client. To get the tenant you can access it via "Azure Active Directory > Properties > Directory ID in the Azure portal" as per Azure's [How to get an Azure Active Directory tenant documentation](https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-howto-tenant).

#### ClientId
The Azure Client GUID for the client. This will be the GUID listed as the "Application ID" when you get when create the app registration under Azure Active Directory.

#### ClientSecret
The Azure generated secret for tor the client. This will be under Settings :: Keys for the application as referenced in ClientID above.

#### Concurrency
For expressions that need to make multiple http requests like `azmulti()`, this sets the amount of concurrent http requests that will be made at a time. In other words, the number of request workers.

This is an optional parameter, If not set or set to `0` then it will be the default value of `10` workers. A value of `1` means no concurrency since there will only be one worker.

#### DebugRequest
If set to `true` then HTTP requests to the Azure API's will be logged to stdout. This is an optional parameter with a default of `false`.

#### DebugResponse
If set to `true` then HTTP responses from Azure API's will be logged to stdout. This is an optional parameter with a default of `false`.

#### Example

```
[AzureMonitorConf]
# Default instance will be queries when the prefix key is absent or is there and set to "default". If only defining one client for azure you would use this.
[AzureMonitorConf.default]
    SubscriptionId = "52c5bef7-d925-4d0e-9bcd-969dbdbb1068"
    TenantId = "60730c79-4f4c-4782-9eca-3325638b8f9c"
    ClientId = "2a434dc4-48df-43b8-ad0e-020798bcb36c"
    ClientSecret = "AzureMakes+Aweso//meSecrets="
    Concurrency = 5

[AzureMonitorConf.foo]
    SubscriptionId = "4b5922c6-0c5a-462f-876f-07072e842ade"
    TenantId = "2e8abf15-1328-458b-8762-192139857055"
    ClientId = "3a6d47ca-53eb-4a18-aa77-d3e2d764ba31"
    ClientSecret = "AzureCreate+Grea/tSecrets="
```

### OpenTSDBConf
Enables an OpenTSDB provider, and also enables [OpenTSDB specific
functions](/expressions#opentsdb-query-functions) in the expression
language. This also enables the Graph tab in Bosun's UI as that is
OpenTSDB specific. However, you can still graph other time series DBs in
Bosun's UI by using the Expression tab.

#### Host
OpenTSDB hostname and port to connect to.

#### Version
Either 2.1 or 2.2. Default: 2.1. If set to 2.2, certain features that
correspond to new OpenTDSB features are enabled e.g. [filter
support](http://opentsdb.net/docs/build/html/user_guide/query/filters.html).

#### ResponseLimit
Causes requests to error if the OpenTSDB response is larger than
this setting in bytes. This is useful to catch alerts that are pulling
in more data then they need to (i.e. maybe downsampling can be used.)

This does not cancel the query with OpenTSDB, but Bosun will stop processing the response.

#### Example

```
[OpenTSDBConf]
	Host = "ny-tsdb01:4242"
	Version = 2.2
	ResponseLimit = 25000000
```

### ElasticConf
Enables querying multiple Elastic clusters. The [elastic expression
functions](/expressions#elastic-query-functions) become available when
this is defined. The functions are designed more to be used for querying
log formatted data and stats from those logs.

The functions that would allow you to use Elastic effectively as a
time-series based backend do not currently exist.

#### ElasticConf.default
Default cluster to query when [PrefixKey](/expressions#prefixkey) is not
passed to the [elastic expression
functions](/expressions#elastic-query-functions).

#### Hosts
List of hosts that are members of the cluster. Elastic uses these hosts
to discover all the other hosts in the cluster, so you don't have to
specify every host in the cluster. [This article
on Sniffing](https://github.com/olivere/elastic/wiki/Sniffing) describes
how this discovery functions.

#### SimpleClient
Boolean determining when setting true periodic health checks and
sniffing will be disabled. This is useful when you want to query from a
single or fixed number of Elastic nodes.

#### ClientOptions
Advanced [Elastic](http://olivere.github.io/elastic/) client options
e.g. basic authentication, http schema etc.

#### Example

```
[ElasticConf]
    [ElasticConf.default]
        Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]

    [ElasticConf.foo]
        Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]
        SimpleClient = true

    [ElasticConf.bar]
        Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]

        [ElasticConf.bar.ClientOptions]
           Enabled = true
           BasicAuthUsername = "admin"
           BasicAuthPassword = "testing"
           Scheme = "https" => default http
           SnifferEnabled = false => default true
           SnifferTimeoutStartup = 10 => default 5s
           HealthcheckEnabled = false
```

### GraphiteConf
Enables querying Graphite server and exposes its query functions to the
expression language.

#### Host
Graphite connection host and port, e.g. `Host = "localhost:80"`.

#### GraphiteConf.Headers
Headers as key / value pairs (one per line) that will be sent with each
Graphite request.

#### Example

```
[GraphiteConf]
	Host = "localhost:80"
	[GraphiteConf.Headers]
		X-Meow = "Mix"
```

### AnnotateConf
Embeds the annotation service. This enables the ability to submit and
edit annotations via the UI or API. It also enables the annotation
related expression functions. Currently the only supported database for
annotate is Elastic. It can be the same cluster as the one defined in
`ElasticConf` or a different one.

<div class="admonition warning">
<p class="admonition-title">Warning</p>
<p>The format of annotation configuration may change before the final 0.6.0 release.</a>.</p>
</div>

#### Hosts
As for ElasticConf.

#### SimpleClient
As for ElasticConf.

#### ClientOptions
As for ElasticConf.

#### Index
The Elastic index to store annotations in. If not set the default is "annotate".

#### Example
```
[AnnotateConf]
    Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]
    Index = myAnnotate
```

```
[AnnotateConf]
    Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]
    SimpleClient = true
    Index = myAnnotate
```

```
[AnnotateConf]
    Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]
    Index = myAnnotate
    [AnnotateConf.ClientOptions]
           Enabled = true
           BasicAuthUsername = "admin"
           BasicAuthPassword = "testing"
           Scheme = "https" => default http
           SnifferEnabled = false => default true
           SnifferTimeoutStartup = 10 => default 5s
           HealthcheckEnabled = false
```

### InfluxConf
Enables the Influx TSDBProvider and makes its query functions available via the API.

#### URL
Full URL that Influx should use to connect to. e.g. `URL = "https://myInfluxServer:1234"`

#### Timeout
Timeout for Influx queries, formatted as per the [Go
duration format](https://golang.org/pkg/time/#Duration.String). e.g. `Timeout = "5m"`

#### UnsafeSSL
Setting to `true` allows you to connect to a Influx server even if the
https certificate is not validated correctly.

#### Username
Influx connection username.

#### Password
Influx connection password.

#### UserAgent
User agent that Bosun should identify itself as when querying Influx.

#### Example:

```
[InfluxConf]
	URL = "https://myInfluxServer:1234"
	Timeout = "5m"
	UnsafeSSL = true
```

### AuthConf
Bosun authentication settings. If not specified, your instance will have
no authentication, and will be open to anybody. When using Auth, TLS
should be used so credentials are not sent in clear text.

#### CookieSecret
A secret string used to encrypt cookies Bosun sets in client browsers to prevent token forgery. We recommend a longish random string here.

#### TokenSecret
Supplying this will activate "token authentication", where you can generate api tokens that external apps and services can use to interact with Bosun. Should be a long random string.

#### AuthDisabled
Disable authentication. Set to true to allow any user to access and change Bosun. The main use case for using this vs omitting `AuthConf` altogether is setting to true,
and also enabling token auth to generate tokens before fully activating Authentication.

#### AuthConf.LDAP
LDAP authentication configuration.

##### AuthConf.LDAP.Domain
LDAP Domain name.

##### AuthConf.LDAP.LdapAddr
Host and port of LDAP server

##### AuthConf.LDAP.AllowInsecure
Set to true to skip certificate validation if you are running self-signed certs, for example.

##### AuthConf.LDAP.DefaultPermission
Default permissions that will be applied to any user who can authenticate to LDAP.

##### AuthConf.LDAP.RootSearchPath
Base search path for searching group and user memberships. Not needed if not specifying Group level permissions. Usually just `DC=myOrg,DC=com` is sufficient.

##### AuthConf.LDAP.Groups
Allows you to set permission levels per LDAP group. See example for usage.

##### AuthConf.LDAP.Users
Allows you to grant permissions to individual users. See example for usage.

#### Permissions
Various parts of the config allow you to specify permissions. These
fields accept a comma seperated list of roles or permissions. Available
roles and permissions are defined [in the Bosun
source](https://github.com/bosun-monitor/bosun/blob/master/cmd/bosun/web/roles.go#L33).
Any of the description values can be used as a permission in the config.
See example for some examples. If a user matches multiple Group or User permissions, they will have the aggregate of all permissions granted to those groups and users, as well as the defaults.

#### Example:
```
[AuthConf]
  CookieSecret = "MAPpHDIjciqzTg708Ef0AXLeid0o9ghrwKReyj57RPUCk80QffmLvVVHqc4w+A=="
  TokenSecret =  "hEgeP7DJn0e2RmhOEQLaNItzNN0fm8fWyyX1F3PB3qUbD9859xFxdL1JPLknuQ=="
  [AuthConf.LDAP]
    Domain = "mycompany"
    LdapAddr = "ldap.mycompany.com:3269"
    DefaultPermission = "Reader"
    RootSearchPath = "DC=ds,DC=mycompany,DC=com"
    [[AuthConf.LDAP.Groups]]
      Path = "CN=Sysadmins,OU=Security Groups,DC=mycompany,DC=com"
      Role = "Admin"
    [[AuthConf.LDAP.Groups]]
      Path = "CN=Developers,OU=Security Groups,DC=mycompany,DC=com"
      Role = "Writer"
    [AuthConf.LDAP.Users]
      jSmith = "Actions,Create Annotations,Silence"
```

</div>
</div>
