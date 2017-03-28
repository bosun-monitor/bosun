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

{% raw %}

## Changes Since 0.5.0
This configuration has been split into two different files. One file is for [definitions](/definitions) of various Bosun sections such as alerts, templates, etc, This file, can be edited and saved via Bosun's UI when [the `EnableSave` setting](/system_configuration#enablesave) has been enabled. 

This was done because the definitions can now be reloaded without restarting the Bosun process. This also means that users can edit alerts directly in the UI.

System configuration has been moved into a new file. Settings in this file require that Bosun be restarted. The new file format is in [toml](https://github.com/toml-lang/toml). The page documents this new system configuration. 

There is also an [example file](https://github.com/bosun-monitor/bosun/blob/master/cmd/bosun/bosun.example.toml) that can be looked at. It should be noted that this file does not follow the tradition of commenting out all defaults. This is because the file is used for testing as well. For the time being, the value of the example being tested is has been valued over following that tradition for until we have the bandwidth to duplicate the two files in a way where this tradition can be maintained. 

## Configuration Keys

### Hostname
The `Hostname` sets the hostname that bosun will use to construct all its links. The common use case would be in any [template functions](/definitions#template-functions) that construct links.

Example:
`Hostname = "bosun.example.com"`

### HTTPListen
`HTTPListen` sets the HTTP IP and Port to Listen on. (TODO: Document what this will look like when we have SSL merged). The default if not specified is to listen on `:8070`

Example:
`HTTPListen = ":8080"`

### HTTPSListen
(TODO: Document)

### CheckFrequency
`CheckFrequency` specifies the minimum interval that alert checks will run at on a schedule. The format of the value is the same as [Go's duration format](https://golang.org/pkg/time/#Duration.String). By default, alert checks are run at every `CheckFrequency` multipied by the `DefaultRunEvery` value. This defaults to "5m".

Example:
`CheckFrequency = "1m"`

### DefaultRunEvery
By default, alert checks are run at every [`CheckFrequency`](/system_configuration#checkevery) multiplied by the `DefaultRunEvery` value. This can be overridden in an alert definition with the [`runEvery` keyword](http://localhost:4000/definitions#runevery). This defaults to 1.
 
So for example if you have a `CheckFrequency` of "1m" and a `DefaultRunEvery` of 5, alerts by default will run every 5 minutes. But you could have some run as frequent as every "1m", and others that run less often (any multiple of "1m").

Example:
`DefaultRunEvery = 5`

### RuleFilePath
This is the path to the file that contains all the definitions of alerts, macros, lookups, templates, notifications, and global variables which are [documented here](/definitions). If saving is enabled, this file can be written to by Bosun via the API or UI.

Example: `RuleFilePath = "dev.sample.conf"`

### TimeAndDate
`TimeAndDate` is used to configure time zones that will be linked to in Bosun's dashboard. It is an array of timeanddate.com zones (the page that gets linked to from Bosun's UI.) It has no impact on what time zone Bosun operates in. Bosun is expected to use UTC and does support other timezones.

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
If set to `true`, Bosun will ping every value of the host tag that it has indexed (TODO: link to an explanation of indexing) and record that value to your TSDB. It currently only support OpenTSDB style data input, which is means you must use either OpenTSDB or Influx with the OpenTSDB endpoint on Influx configured. 

Example: 
`Ping = true`

### PingDuration
`PingDuration` is how long bosun should wait stop pinging host tags it has seen. For example, if the value is the default of `"24h"`, if Bosun has not indexed any data points for that `host` value, then it will stop attempting to ping that host until it sees data points that have that tag again.

Example:
`PingDuration = "24h"`

### SearchSince
`SearchSince` controls how long autocomplete and items in UI will show up since being indexed by Bosun. The format of the value is the same as [Go's duration format](https://golang.org/pkg/time/#Duration.String) and the default is 3 days. The goal is to make it so you don't have old items showing up in the UI. However, if you are using OpenTSDB and graphing page, you can still query metrics that don't autocomplete if you remember what they were (or look them up using OpenTSDB's native UI autocomplete).

Example: `SearchSince = "72h"`

### EnableSave
`EnableSave` enables saving via the user interface. It is disabled by default. When it is enabled, users will be able to save the rule configuration file via the UI and Bosun will then write to that file on the user's behalf.

Example: `EnableSave = true`

### ReloadEnabled
`ReloadEnabled` sets if reloading of the rule configuration should be enabled. If `EnableSave` is `true`, then reloading gets enabled regardless of this setting. Reloads can be triggered via the API by (TODO: Document the reload web api).

Example:
`EnableSave = true`

### CommandHookPath
When enabling saving, and a user issues a save, you have the option to run a executable or script by specifying this parameter. This allows you to do things like backup the file on writes or commit the file to a git repo.

This command is passed a filename, username, message, and vargs (vargs is currently not used). If the command exits a non-zero exit code, then the changes will be reverted (the file before the changes is copied back and bosun doesn't restart).

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

### DBConf
`DBConf` defines what internal storage Bosun should use. There are currently two choices, a built-in redis like server called ledis or redis. Redis is recommended for production setups. 

The default is to use ledis. If Both Redis and ledis are defined, Redis will take preference and the ledis configuration will be ignored. Ledis is the default, so if `RedisHost` is not specified ledis will be used even if you have no `DBConf` configuration defined.

#### RedisHost
The value of `RedisHost` defines the hostname and port to connect to for redis. 

#### RedisPassword
The value of `RedisPassword` defines an optional password to use when connecting to redis

#### LedisDir
`LedisDir` defines the directory that ledis will store its data in if Ledis is being used instead of Redis. The default is `LedisDir = "ledis_data"`

#### LedisBind
`LedisBind` is the host and port to connect to for ledis. The default is `LedisBindAddr = "127.0.0.1:9565"`.

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
`SMTPConf` Configure email for you servers

#### EmailFrom
`EmailFrom` is the address that emails from Bosun will be sent from.

#### Host
`Host` is the address of the mailserver to connect to in order to send emails

#### Username
(TODO: See how this and Password is used with email auth, don't have a current example.)

#### Password

#### Example

```
[SMTPConf]
	EmailFrom = "bosun@example.com"
	Host = "mail.example.com"
```

### OpenTSDBConf
`OpenTSDBConf` enables an OpenTSDB provider, and also enables OpenTSDB specific functions in the expression language. This also enables the Graph tab in Bosun's UI as that is OpenTSDB specific. However, you can still graph other time series DBs in Bosun's UI by using the Expression tab.

#### Host 
`Host` specifies the hostname and port to connect to for OpenTSDB.

#### Version
`Version` can either be 2.1 or 2.2. 2.1 is the default. When it is set to 2.2 then certain features that correspond to new OpenTDSB features are enabled such as [filter support](http://opentsdb.net/docs/build/html/user_guide/query/filters.html).

#### ResponseLimit
`ResponseLimit` will make requests error if the response from OpenTSDB is larger than this setting in bytes. This is useful to catch alerts that are pulling in more data then they need to (i.e. maybe downsampling can be used.)

This does not cancel the query with OpenTSDB, but bosun will stop processing the response. 

#### Example

```
[OpenTSDBConf]
	Host = "ny-tsdb01:4242"
	Version = 2.2
	ResponseLimit = 25000000
```

### ElasticConf
`ElasticConf` enables you to query an elastic cluster. The [elastic expression functions](/expressions#elastic-query-functions) become available when this is defined. The functions are designed more to be used for querying log formatted data and stats from those logs. 

The functions that would allow you to use elastic effectively as a time-seris based backend do not currently exist.

<div class="admonition warning">
<p class="admonition-title">Warning</p>
<p>The format of elastic configuration may change before the final 0.6.0 release.</a>.</p>
</div>

#### Hosts
`Hosts` is a list of hosts that are members of the cluster. It will uses these hosts to discover all the other hosts in the cluster. So you don't have to specify every host in the cluster for Bosun to query it. [This article on Sniffing](https://github.com/olivere/elastic/wiki/Sniffing) describes how this discovery functions.

#### Example

```
[ElasticConf]
	Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]
```

### GraphiteConf
`GraphiteConf` enables you to query a graphite server and makes the graphite query functions available to the expression language.

#### Host
The host and port to connect to for querying graphite. For example, `Host = "localhost:80"`.

#### GraphiteConf.Headers
`[GraphiteConf.Headers]` lets you specify headers as key value pairs (one per line) that will be sent with each graphite request.

#### Example

```
[GraphiteConf]
	Host = "localhost:80"
	[GraphiteConf.Headers]
		X-Meow = "Mix"
```

### AnnotateConf
Embeds the annotation service. This enables the ability to submit and edit annotations via the UI or API. It also enables the annotation related expression functions. Currently the only supported database for annotate is elastic. It can be the same cluster as the one defined in `ElasticConf` or a different one.

<div class="admonition warning">
<p class="admonition-title">Warning</p>
<p>The format of annotation configuration may change before the final 0.6.0 release.</a>.</p>
</div>

#### Hosts
`Hosts` is a list of hosts that are members of the cluster. It will uses these hosts to discover all the other hosts in the cluster. So you don't have to specify every host in the cluster for Bosun to query it. [This article on Sniffing](https://github.com/olivere/elastic/wiki/Sniffing) describes how this discovery functions.

#### Index
The elastic index to store annotations in. If not set the default is "annotate".

#### Example
```
[AnnotateConf]
    Hosts = ["http://ny-lselastic01.example.com:9200", "http://ny-lselastic02.example.com:9200"]
     Index = myAnnotate
```

### InfluxConf
This enables the Influx TSDBProvider and makes the influx query functions available via the API.

#### URL
`URL` is the the full url that Influx should use to connect to. For example: `URL = "https://myInfluxServer:1234"`

#### Timeout
Set the timeout for Influx queries. The format of the value is the same as [Go's duration format](https://golang.org/pkg/time/#Duration.String). For example `Timeout = "5m"`

#### UnsafeSSL
Setting UnsafeSSL to `true` allows you to connect to a influx server even if the https certificate is not validated correctly.

#### Username
The username to use to connect to influx.

#### Password
The password to use in combination with the username to connect to Influx.

#### UserAgent
This specifies the user agent that bosun should identify itself as when querying Influx.

#### Example:

```
[InfluxConf]
	URL = "https://myInfluxServer:1234"
	Timeout = "5m"
	UnsafeSSL = true
```

### AuthConf
`AuthConf` configures your bosun authentication settings. If not specified, your bosun instance will have no authentication, and will be open to anybody.

#### CookieSecret
A secret string used to encrypt cookies bosun sets in client browsers to prevent token forgery. We recommend a longish random string here.

#### TokenSecret
Supplying this will activate "token authentication", where you can generate api tokens that external apps and services can use to interact with bosun. Should be a long random string.

#### AuthDisabled
Disable authentication. Set to true to allow any user to access and change bosun. The main use case for using this vs omitting `AuthConf` altogether is setting to true,
and also enabling token auth to generate tokens before fully activating Authentication.

#### AuthConf.LDAP
Allows you to configure LDAP authentication for bosun. Subkeys:

#### AuthConf.LDAP.Domain
LDAP Domain name. 

#### AuthConf.LDAP.LdapAddr
Host and port of LDAP server

#### AuthConf.LDAP.AllowInsecure
Set to true to skip certificate validation if you are running self-signed certs, for example.

#### AuthConf.LDAP.DefaultPermission
Default permissions that will be applied to any user who can authenticate to LDAP.

#### AuthConf.LDAP.RootSearchPath
Base search path for searching group and user memberships. Not needed if not specifying Group level permissions. Usually just `DC=myOrg,DC=com` is sufficient. 

#### AuthConf.LDAP.Groups
Allows you to set permission levels per LDAP group. See example for usage.

#### AuthConf.LDAP.Users
Allows you to grant permissions to individual users. See example for usage.

#### Permissions
A few places in the config allow you to specify permissions. These fields accept a comma seperated list of roles or permissions. Availible roles and permissions are defined
[in the bosun source](https://github.com/bosun-monitor/bosun/blob/master/cmd/bosun/web/roles.go#L33). Any of the description values can be used as a permission in the config.
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