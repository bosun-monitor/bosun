---
layout: default
title: Configuration (0.5.0 and earlier)
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

<div class="admonition">
<p class="admonition-title">Attention</p>
<p>This documentation is for versions prior to 0.6.0. For 0.6.0 there are two different documentation sections that replace this section: <a href="/system_configuration">system configuration</a> and <a href="/definitions">definitions</a>.</p>
</div>

{% raw %}

Syntax is sectional, with each section having a type and a name, followed by `{` and ending with `}`. Key/value pairs follow of the form `key = value`. Key names are non-whitespace characters before the `=`. The value goes until end of line and is a string. Multi-line strings are supported using backticks to delimit start and end of string. Comments go from a `#` to end of line (unless the `#` appears in a backtick string). Whitespace is trimmed at ends of values and keys. Files are UTF-8 encoded.

## Variables

Variables perform simple text replacement - they are not intelligent. They are any key whose name begins with `$`, and may also be surrounded by braces (`{`, `}`) to disambiguate between shorter keys (ex: `${var}`) Before an expression is evaluated, all variables are evaluated in the text. Variables can be defined at any scope, and will shadow other variables with the same name of higher scope.

### Environment Variables

Environment variables may be used similarly to variables, but with `env.` preceding the name. For example: `tsdbHost = ${env.TSDBHOST}` (with or without braces). It is an error to specify a non-existent or empty environment variable.

## Sections

### globals

Globals are all key=value pairs not in a section. These are generally placed at the top of the file.
Every variable is optional, though you should enable at least 1 backend.

#### backends

* tsdbHost: OpenTSDB host. Must be GZIP-aware (use the [next branch](https://github.com/opentsdb/opentsdb/tree/next)). Can specify both host and port: `tsdb-host:4242`. Defaults to port 4242 if no port specified. If you use opentsdb without relaying the data through Bosun currently the following won't work (and this isn't something we officially support):
  * Tag value glob matching, for example `avg:metric.name{tag=something-*}`. However single asterisks like `tag=*` will still work.
  * The items page.
  * The graph page's tag list.
* tsdbVersion: Defaults to 2.1 if not present. Should always be specified as Number.Number. Various OpenTSDB features are added with newer versions.
* relayListen: Listen on the given address (i.e., set to :4242) and will pass through all /api/X calls to your OpenTSDB server. This is an optinal parameter when using OpenTSDB so it is not required for any Bosun functionality
* graphiteHost: an ip, hostname, ip:port, hostname:port or a URL, defaults to standard http/https ports, defaults to "/render" path.  Any non-zero path (even "/" overrides path)
* graphiteHeader: a http header to be sent to graphite on each request in 'key:value' format. optional. can be specified multiple times.
* logstashElasticHosts: Elasticsearch hosts populated by logstash. Must be a CSV list of URLs and only works with elastic pre-v2. The hosts you list are used to discover all hosts in the cluster.
* elasticHosts: Elasticsearch hosts. This is not limited to logstash's schema. It must be a CSV list of URLs and only works with elastic v2 and later. The hosts you list are used to discover all hosts in the cluster.
* annotateElasticHosts: Enables annotations by setting this. Is a CSV list of URLs like elasticHosts. More on annotations in the [usage documentation](http://bosun.org/usage#annotations). By default the index is named "annotate" and will be created if it doesn't exist. You can change which index to use/create with the annotateIndex setting.
* influxHost: InfluxDB host address ip:port pair.
* influxUsername: InfluxDB username. If empty will attempt to connect without authentication.
* influxPassword: InfluxDB password. If empty will attempt to connect without authentication.
* influxTLS: Whether to use TLS when connecting to InfluxDB. Default is false.
* influxTimeout: Timeout duration for connections to InfluxDB.

#### data storage

With bosun v0.5.0, bosun uses redis as a storage mechanism for it's internal state. You can either run a redis instance to hold this data, or bosun can use an embedded server if you would rather run standalone (using [ledisDb](http://ledisdb.com/)). Redis is recommend for production use. [This gist](https://gist.github.com/kylebrandt/3fdc97171b96ba46fd9e1d14abd03027) shows an example redis config, tested redis version, and an example cron job for backing up the redis data.

Config items:  

* redisHost: redis server to use. Ex: `localhost:6379`. Redis 3.0 or greater is required.
* redisDb: redis database to use. Default is `0`.
* redisPassword: redis password.
* ledisDir: directory for ledisDb to store it's data. Will default to `ledis_data` in working dir if no redis host is provided.
* ledisBindAddr: Address and port for ledis to bind to, defaults to `127.0.0.1:9565`.

#### settings

* checkFrequency: time between alert checks, defaults to `5m`
* defaultRunEvery: default multiplier of check frequency to run alerts. Defaults to `1`.
* emailFrom: from address for notification emails, required for email notifications
* httpListen: HTTP listen address, defaults to `:8070`
* hostname: when generating links in templates, use this value as the hostname instead of using the system's hostname
* minGroupSize: minimum group size for alerts to be grouped together on dashboard. Default `5`.
* ping: if present, will ping all values tagged with host
* responseLimit: number of bytes to limit OpenTSDB responses, defaults to 1MB (`1048576`)
* searchSince: duration of time to filter by during certain searches, defaults to `3d`; currently used by the hosts list on the items page
* smtpHost: SMTP server, required for email notifications
* squelch: see [alert squelch](#squelch)
* stateFile: bosun state file, defaults to `bosun.state`
* unknownTemplate: name of the template for unknown alerts
* shortURLKey: goo.gl API key, needed if you hit usage limits when using the short link button
* timeAndDate: The configuration parameter for the worldclock links is timeAndDate, i.e. `timeAndDate = 202,75,179,136` adds adds Portland, Denver, New York, and London to the datetime links generated in alerts. See [timeanddate.com documentation](http://www.timeanddate.com/worldclock/converter-about.html)

#### SMTP Authentication

These optional fields, if either is specified, will authenticate with the SMTP server

* smtpUsername: SMTP username
* smtpPassword: SMTP password

### macro

Macros are sections that can define anything (including variables). It is not an error to reference an unknown variable in a macro. Other sections can reference the macro with `macro = name`. The macro's data will be expanded with the current variable definitions and inserted at that point in the section. Multiple macros may be thus referenced at any time. Macros may reference other macros. For example:

~~~
$default_time = "2m"

macro m1 {
	$w = 80
	warnNotification = default
}

macro m2 {
	macro = m1
	$c = 90
}

alert os.high_cpu {
	$q = avg(q("avg:rate:os.cpu{host=ny-nexpose01}", $default_time, ""))
	macro = m2
	warn = $q > $w
	crit = $q >= $c
}
~~~

Will yield a warn expression for the os.high_cpu alert:

~~~
avg(q("avg:rate:os.cpu{host=ny-nexpose01}", "2m", "")) > 80
~~~

and set `warnNotification = default` for that alert.

### template

Templates are the message body for emails that are sent when an alert is triggered. Syntax is the golang [text/template](http://golang.org/pkg/text/template/) package. Variable expansion is not performed on templates because `$` is used in the template language, but a `V()` function is provided instead. Email bodies are HTML, subjects are plaintext. Macro support is currently disabled for the same reason due to implementation details.

* body: message body (HTML)
* subject: message subject (plaintext)

#### Variables available to alert templates:

* Ack: URL for alert acknowledgement
* Expr: string of evaluated expression
* Group: dictionary of tags for this alert (i.e., host=ny-redis01, db=42)
* History: array of Events. An Event has a `Status` field (an integer) with a textual string representation; and a `Time` field. Most recent last. The status fields have identification methods: `IsNormal()`, `IsWarning()`, `IsCritical()`, `IsUnknown()`, `IsError()`.
* Incident: URL for incident page
* IsEmail: true if template is being rendered for an email. Needed because email clients often modify HTML.
* Last: last Event of History array
* Subject: string of template subject
* Touched: time this alert was last updated
* Alert: dictionary of rule data (but the first letter of each is uppercase)
  * Crit
  * IncidentId
  * Name
  * Vars: alert variables, prefixed without the `$`. For example: `{{.Alert.Vars.q}}` to print `$q`.
  * Warn

#### Functions available to alert templates:

* Eval(string): executes the given expression and returns the first result with identical tags, or `nil` tags if none exists, otherwise `nil`.
* EvalAll(string): executes the given expression and returns all results. The `DescByValue` function may be called on the result of this to sort descending by value: `{{(.EvalAll .Alert.Vars.expr).DescByValue}}`.
* GetMeta(metric, name, tags): Returns metadata data for the given combination of metric, metadata name, and tag. `metric` and `name` are strings. `tags` may be a tag string (`"tagk=tagv,tag2=val2"`) or a tag set (`.Group`). If If `name` is the empty string, a slice of metadata matching the metric and tag is returned. Otherwise, only the metadata value is returned for the given name, or `nil` for no match.
* Graph(expression, y_label): returns an SVG graph of the expression with tags identical to the alert instance. `expression` is a string or an expression and `y_label` is a string. `y_label` is an optional argument.
* GraphLink(expression): returns a link to the graph tab for the expression page for the given expression. The time is set to the time of the alert. `expression` is a string.
* GraphAll(expression, y_label): returns an SVG graph of the expression. `expression` is a string or an expression and `y_label` is a string. `y_label` is an optional argument.
* LeftJoin(expr, expr[, expr...]): results of the first expression (which may be a string or an expression) are left joined to results from all following expressions.
* Lookup("table", "key"): Looks up the value for the key based on the tagset of the alert in the specified lookup table
* LookupAll("table", "key", "tag=val,tag2=val2"): Looks up the value for the key based on the tagset specified in the given lookup table
* HTTPGet("url"): Performs an http get and returns the raw text of the url
* HTTPGetJSON("url"): Performs an http get for the url and returns a [jsonq.JsonQuery object](https://godoc.org/github.com/jmoiron/jsonq)
* LSQuery("indexRoot", "filterString", "startDuration", "endDuration", nResults). Returns an array of a length up to nResults of Marshaled Json documents (Go: marshaled to interface{}). This is like the lscount and lsstat functions. There is no `keyString` because the group (aka tags) if the alert is used.
* LSQueryAll("indexRoot", "keyString" filterString", "startDuration", "endDuration", nResults). Like LSQuery but you have to specify the `keyString` since it is not scoped to the alert.
* ESQuery(index ESIndexer, filter ESQuery, startDuration string, endDuration string, nResults Scalar). Returns an array of a length up to nResults of Marshaled Json documents (Go: marshaled to interface{}). This is like the escount and esstat functions. The group (aka tags) of the alert is used to further filter the results.
* ESQueryAll((index ESIndexer, filter ESQuery, startDuration string, endDuration string, nResults Scalar). Like ESQuery but the results are not filtered based on the tagset (aka group) of the alert. As an example:

```
template test {
	subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}
	body = `
	    {{ $filter := (.Eval .Alert.Vars.filter)}}
	    {{ $index := (.Eval .Alert.Vars.index)}}
	    {{range $i, $x := .ESQuery $index $filter "5m" "" 10 }}
	        <p>{{$x.machinename}}</p>
	    {{end}}
	`
}

alert test {
	template = test
	$index = esls("logstash")
	$filter = esand(esregexp("source", ".*"), esregexp("machinename", "ls-dc.*"))
    crit = avg(escount($index, "source,machinename", $filter, "2m", "10m", ""))
}
```

Global template functions:

* V: performs variable expansion on the argument and returns it. Needed since normal variable expansion is not done due to the `$` character being used by the Go template syntax.
* bytes: converts the string input into a human-readable number of bytes with extension KB, MB, GB, etc.
* pct: formats the float argument as a percentage. For example: `{{5.1 | pct}}` -> `5.10%`.
* replace: [strings.Replace](http://golang.org/pkg/strings/#Replace)
* short: Trims the string to everything before the first period. Useful for turning a FQDN into a shortname. For example: `{{short "foo.baz.com"}}` -> `foo`.
* parseDuration: [time.ParseDuration](http://golang.org/pkg/time/#ParseDuration). Useful when working with an alert's .Last.Time.Add method to generate urls to other systems.
* html: takes a string and renders it as html. Useful for when you have alert variables that contain html. For example in the alert you may have `$notes = <a href="...">Foo</a>` and the in the template you can render it as html with `{{ html .Alert.Vars.notes }}` 

All body templates are associated, and so may be executed from another. Use the name of the other template section for inclusion. Subject templates are similarly associated.

An example:

~~~
template name {
	body = Name: {{.Alert.Name}}
}
template ex {
	body = `Alert definition:
	{{template "name" .}}
	Crit: {{.Alert.Crit}}

	Tags:{{range $k, $v := .Group}}
	{{$k}}: {{$v}}{{end}}
	`
	subject = {{.Alert.Name}}: {{.Alert.Vars.q | .E}} on {{.Group.host}}
}
~~~

#### unknown template

The unknown template (set by the global option `unknownTemplate`) acts differently than alert templates. It receives groups of alerts since unknowns tend to happen in groups (i.e., a host stops reporting and all alerts for that host trigger unknown at the same time).

Variables and function available to the unknown template:

* Group: list of names of alerts
* Name: group name
* Time: [time](http://golang.org/pkg/time/#Time) this group triggered unknown

Example:

~~~
template ut {
	subject = {{.Name}}: {{.Group | len}} unknown alerts
	body = `
	<p>Time: {{.Time}}
	<p>Name: {{.Name}}
	<p>Alerts:
	{{range .Group}}
		<br>{{.}}
	{{end}}`
}

unknownTemplate = ut
~~~

### alert

An alert is an evaluated expression which can trigger actions like emailing or logging. The expression must yield a scalar. The alert triggers if not equal to zero. Alerts act on each tag set returned by the query. It is an error for alerts to specify start or end times. Those will be determined by the various functions and the alerting system.

* crit: expression of a critical alert (which will send an email)
* critNotification: comma-separated list of notifications to trigger on critical. This line may appear multiple times and duplicate notifications, which will be merged so only one of each notification is triggered. Lookup tables may be used when `lookup("table", "key")` is an entire `critNotification` value. See example below.
* depends: expression that this alert depends on. If the expression is non-zero, this alert is unevaluated. Unevaluated alerts do not change state or become unknown.
* ignoreUnknown: if present, will prevent alert from becoming unknown
* unknownIsNormal: will convert unkown events into normal events. For example, if you are alerting for the existence of error log messages, when there are none, that means things are normal. Using `ignoreUnknown` with this setting would be uneccesary.
* runEvery: multiple of global `checkFrequency` at which to run this alert. If unspecified, the global `defaultRunEvery` will be used.
* squelch: <a name="squelch"></a> comma-separated list of `tagk=tagv` pairs. `tagv` is a regex. If the current tag group matches all values, the alert is squelched, and will not trigger as crit or warn. For example, `squelch = host=ny-web.*,tier=prod` will match any group that has at least that host and tier. Note that the group may have other tags assigned to it, but since all elements of the squelch list were met, it is considered a match. Multiple squelch lines may appear; a tag group matches if any of the squelch lines match.
* template: name of template
* unjoinedOk: if present, will ignore unjoined expression errors
* unknown: time at which to mark an alert unknown if it cannot be evaluated; defaults to global checkFrequency
* warn: expression of a warning alert (viewable on the web interface)
* warnNotification: identical to critNotification, but for warnings
* log: setting `log = true` will make the alert behave as a "log alert". It will never show up on the dashboard, but will execute notifications every check interval where the status is abnormal.
* maxLogFrequency: will throttle log notifications to the specified duration. `maxLogFrequency = 5m` will ensure that notifications only fire once every 5 minutes for any given alert key. Only valid on log alerts.

Example of notification lookups:

~~~
notification all {
	#...
}

notification n {
	#...
}

notification d {
	#...
}

lookup l {
	entry host=a {
		v = n
	entry host=b* {
		v = d
	}
}

alert a {
	crit = 1
	critNotification = all # All alerts have the all notification.
	# Other alerts are passed through the l lookup table and may add n or d.
	# If the host tag does not match a or b*, no other notification is added.
	critNotification = lookup("l", "v")
	# Do not evaluate this alert if its host is down.
	depends = alert("host.down", "crit")
}
~~~

### notification

A notification is a chained action to perform. The chaining continues until the chain ends or the alert is acknowledged. At least one action must be specified. `next` and `timeout` are optional. Notifications are independent of each other and executed concurrently (if there are many notifications for an alert, one will not block another).

* body: overrides the default POST body. The alert subject is passed as the templates `.` variable. The `V` function is available as in other templates. Additionally, a `json` function will output JSON-encoded data.
* next: name of next notification to execute after timeout. Can be itself.
* timeout: duration to wait until next is executed. If not specified, will happen immediately.
* contentType: If your body for a POST notification requires a different Content-Type header than the default of `application/x-www-form-urlencoded`, you may set the contentType variable. 
* runOnActions: Exclude this notification from action notifications. Notifications will be sent on ack/close/forget actions using a built-in template to all root level notifications for an alert, *unless* the notification specifies `runOnActions = false`. 

#### actions

* email: list of email address of contacts. Comma separated. Supports formats `Person Name <addr@domain.com>` and `addr@domain.com`.  Alert template subject and body used for the email.
* get: HTTP get to given URL
* post: HTTP post to given URL. Alert subject sent as request body. Content type is set as `application/x-www-form-urlencoded` by default, but may be overriden by setting the `contentType` variable for the notification.
* print: prints template subject to stdout. print value is ignored, so just use: `print = true`

Example:

~~~
# HTTP Post to a chatroom, email in 10m if not ack'd
notification chat {
	next = email
	timeout = 10m
	post = http://chat.meta.stackoverflow.com/room/318?key=KEY&message=whatever
}

# email sysadmins and Nick each day until ack'd
notification email {
	email = sysadmins@stackoverflow.com, nick@stackoverflow.com
	next = email
	timeout = 1d
}

# post to a slack.com chatroom via Incoming Webhooks integration
notification slack{
	post = https://hooks.slack.com/services/abcdef
	body = {"text": {{.|json}}}
}

#post json
notification json{
	post = https://someurl.com/submit
	body = {"text": {{.|json}}, apiKey="2847abc23"}
	contentType = application/json
}
~~~

### lookup

Lookups are used when different values are needed based on the group. For example, an alert for high CPU use may have a general setting, but need to be higher for known high-CPU machines. Lookups have subsections for lookup entries. Each entry subsection is named with an OpenTSDB tag group, and supports globbing. Entry subsections have arbitrary key/value pairs.

The `lookup` function can be used in expressions to query lookup data. It takes two arguments: the name of the lookup table and the key to be extracted. When the function is executed, all possible combinations of tags are fetched from the search service, matched to the correct rule, and returned. The first successful match is used. Unmatched groups are ignored.

For example, to filter based on host:

~~~
lookup cpu {
	entry host=web-* {
		high = 0.5
	}
	entry host=sql-* {
		high = 0.8
	}
	entry host=* {
		high = 0.3
	}
}

alert cpu {
	crit = avg(q("avg:rate:os.cpu{host=*}", "5m", "")) > lookup("cpu", "high")
}
~~~

Multiple groups are supported and separated by commas. For example:

~~~
lookup cpu {
	entry host=web-*,dc=eu {
		high = 0.5
	}
	entry host=sql-*,dc=us {
		high = 0.8
	}
	entry host=*,dc=us {
		high = 0.3
	}
	entry host=*,dc=* {
		high = 0.4
	}
}

alert cpu {
	crit = avg(q("avg:rate:os.cpu{host=*,dc=*}", "5m", "")) > lookup("cpu", "high")
}
~~~

# Example File

~~~
tsdbHost = tsdb01.stackoverflow.com:4242
smtpHost = mail.stackoverflow.com:25

template cpu {
	body = `Alert definition:
	Name: {{.Alert.Name}}
	Crit: {{.Alert.Crit}}
	
	Tags:{{range $k, $v := .Group}}
	{{$k}}: {{$v}}{{end}}
	`
	subject = cpu idle at {{.Alert.Vars.q | .E}} on {{.Group.host}}
}

notification default {
	email = someone@domain.com
	next = default
	timeout = 1h
}

alert cpu {
	template = cpu
	$q = avg(q("sum:rate:linux.cpu{host=*,type=idle}", "1m"))
	crit = $q < 40
	notification = default
}
~~~

{% endraw %}

</div>
</div>
