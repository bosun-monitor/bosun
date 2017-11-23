---
layout: default
title: Definitions (RuleConf)
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
Since 0.5.0, the config has been split into two different files.

### System
System config is documented [here](/system_configuration).

### Definitions
This file is documented in the rest of this page. It includes settings that
do not require a Bosun restart to take effect e.g. alerts, templates,
notifications, and its location is defined by [the system configuration's
RuleFilePath](/system_configuration#rulefilepath).

It can be edited from the [Rule Editor](/usage#definition-rule-saving) in the
web UI when the [`EnableSave` setting](/system_configuration#enablesave) is
enabled.

The file is divided into sections, each of which having a type and a
name, followed by `{` and ending with `}`. Each section is a definition
of e.g. an alert or a notification. Key/value pairs follow, written as
`key = value`. Multi-line strings are supported using backticks (\`) to
delimit start and end of string. Comments go from `#` to end of line.

## Alert Definitions
An alert is defined with the following syntax:

```
alert uniqueAlertName {
    $variable = value
    ...
    keyword = value
    ...
}
```

The minimum requirement for an alert is that it have a `warn` or `crit` expression. However, the most common case is to define at least: `warn`, `warnNotification`, `crit`, `critNotification`, and `template`.

### Alert Keywords

#### crit
{: .keyword}
The expression to evaluate to set a critical severity state for an incident that is instantiated from the alert definition. The expression's [return type](/expressions#data-types) must return a Scalar or NumberSet. 

No crit notifications will be sent if `critNotification` is not declared in the alert definition. However, it will still appear on the dashboard.

#### critNotification
{: .keyword}
A comma-separated list of notifications to trigger on critical a state (when the crit expression is non-zero). This line may appear multiple times and duplicate notifications, which will be merged so only one of each notification is triggered. [Lookup tables](/definitions#lookup-tables) may be used when `lookup("table", "key")` is the only `critNotification` value. This means you can't mix notifications names with lookups in the same `critNotification`. However, since an alert can have multiple `critNotification` entries you make one entry that has a lookup, and another that has notification names.

#### depends
{: .keyword}

`depends` is an expression that makes the alert dependent on another alert. If the depends expression evaluates to non-zero than the alert will be [unevaluated](/usage#additional=states). This is most frequently used in conjunction with the [`alert()` expression function](/expressions#alertname-string-key-string-numberset).

Note that the depends feature does not work when using Bosun's testing in the Rule Editor UI.

Given the example that follows there would be two incidents in a warn state: `dependOnMe{host=a}` and `iDependOnOthers{host=b}`. There is *no* incident for `iDependOnOthers{host=a}` because the dependency was true for host a. There *is* an incident for `iDependOnOthers{host=b}` since the dependency was false in the case of host b.

Example:

```
alert dependOnMe {
    template = depend
    $series = merge(series("host=a", 0, 1), series("host=b", 0, 0))
    # host a is 1, host b is 0
    warn = avg($series)
}

alert iDependOnOthers {
    template = depend
    depends = alert("dependOnMe", "warn")
    $series = merge(series("host=a", 0, 1), series("host=b", 0, 1))
    # host a and b are 1
    warn = avg($series)
}

template depend {
    subject = `{{.Alert.Name}}: {{.Group}}`
}
```

#### ignoreUnknown
{: .keyword}
Setting `ignoreUnknown = true` will prevent an alert from becoming unknown. This is often used where you expect the tagsets or data for an alert to be sparse and/or you want to ignore things that stop sending information.

#### log
{: .keyword}
Setting `log = true` will make the alert behave as a "log alert". It will never show up on the dashboard, but will execute notifications every check interval where the status is abnormal. `maxLogFrequency` can be used to throttle the notifications.

#### maxLogFrequency
{: .keyword}
Setting `maxLogFrequency = true` will throttle [log](/definitions#log) notifications to the specified duration. `maxLogFrequency = 5m` will ensure that notifications only fire once every 5 minutes for any given alert key. Only valid on alerts that have `log = true`.

#### runEvery
{: .keyword}
Multiple of global system configuration value [CheckFrequency](/system_configuration#checkfrequency) at which to run this alert. If unspecified, the system configuration value [DefaultRunEvery](/system_configuration#defaultrunevery) will be used for the alert frequency.

#### squelch
{: .keyword}
`squelch` is comma-separated list of `tagk=tagv` pairs. `tagv` is a regex. If the current tag group matches all values, the alert is squelched, and will not trigger as crit or warn. For example, `squelch = host=ny-web.*,tier=prod` will match any group that has at least that host and tier. Note that the group may have other tags assigned to it, but since all elements of the squelch list were met, it is considered a match. Multiple squelch lines may appear; a tag group matches if any of the squelch lines match.

This can also be defined at the global of level of the configuration. 

When using squelch, alerts will be removed even if they are not within the scope of the final tagset. The common case of this would be using the `t` ([transpose function]()) to reduce the number of final results. So when doing this, results will still be removed because they are removed at the expression level for the `warn` and `crit` expressions.

#### template
{: .keyword}
The name of the [template](/definitions#templates) that will be used to send alerts to the specified notifications for the alert.

#### unjoinedOk
{: .keyword}
If present, expressions within the alert will ignore unjoined expression errors. Unjoins happen when expressions with in an alert use a comparison operator (i.e. `>` or `&&`), and there are tagsets in one set but are not in the other set.

#### unknown
{: .keyword}
`unknown` is the duration (i.e. `unknown = 5m` ) at which to mark an incident as [unknown](/usage#severity-states) if it can not be evaluated. It defaults the system configuration global variable [CheckFrequency](/system_configuration#checkfrequency). Bosun remembers the tagsets it has seen for an alert and determines an alert to be unknown when a tagset is no longer present for the alert. 

#### unknownIsNormal
{: .keyword}
Setting `unknownIsNormal = true` will convert unknown events for an incident into a normal event.

This is often useful if you are alerting on log messages where the absence of log messages means that the state should go back to normal. Using `ignoreUnknown` with this setting would be unnecessary.

#### warn
{: .keyword}
The expression to evaluate to set a warn state for an incident that is instantiated from the alert definition. 

The expression must evaluate to a NumberSet or a Scalar (See [Data Types](/expressions#data-types)). 0 is false (do not trigger) and any non-zero value is true (will trigger). 

If the crit expression is true, the warn expression will not be evaluated as crit supersedes warn.

No warn notifications will be sent if `warnNotification` is not declared in the alert definition. It will still however appear on the dashboard.

#### warnNotification
{: .keyword}
Identical to `critNotification` above, but the condition evaluates to warning state.


## Variables 
Variables are in the form of `$foo = someText` where someText continues until the end of the line. These are not variables in the sense that they hold a value, rather they are simply text replacement done by the the parsers.

<div class="admonition">
<p class="admonition-title">Tip</p>
<p>Because this is text replacement, it is important to note that variables do <em>not</em> impact the order of operations. So it may be necessary at times to enclose the variable in parenthesis (either when setting the value, or referencing it).</p>
</div>

They can be referenced by `$foo` or by `${foo}`, the later being useful if you want to use the variable in a context where whitespace does not immediately follow the value.

### Global Variables

Global Variables exist outside of any section and should be defined before they are used.

Global variables can be overridden in sections defining a variable within the scope of the section that has the same name.

## Templates 
Templates are used to construct what alerts will look like when they are sent. They are pointed to in the definition of an alert by the [template keyword](/definitions#template). They are like "views" in web frameworks.

Templates in Bosun are built on top of go's templating. The subject is rendered using the golang [text/template](http://golang.org/pkg/text/template/) package (plaintext) and the body is rendered using the golang [html/template](https://golang.org/pkg/html/template/) package (html).

For learning the fundamentals of templates (how to do conditionals, loops. etc) read the [text/template](http://golang.org/pkg/text/template/) documentation since Bosun templates add on top of that.

Variable expansion is not performed on templates because `$` is used in the template language, but a [`V()` function](/definitions#vstring-string) is provided for global variables and alert variables are available in [.Alert.Vars](/definitions#alertvars).

Macros can not be used in templates, however, templates can [include other templates](/definitions#template-inclusions).

Note that templates are rendered when the expression is evaluated and it is non-normal. This is to eliminate changes in what is presented in the template because the data has changed in the tsdb since the alert was triggered.

### The Unknown Template
Since there is limited information for an alert that is unknown, and since unknowns can be grouped the unknown template is different. 

The unknown template (set by the global option `unknownTemplate`) acts differently than alert templates. It receives groups of alerts since unknowns tend to happen in groups (i.e., a host stops reporting and all alerts for that host trigger unknown at the same time).

Variables and function available to the unknown template:

* Group: list of names of alerts
* Name: group name
* Time: [time](http://golang.org/pkg/time/#Time) this group triggered unknown

Example:

```
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
```

In general it is better to stick with the system default by not defining an unknown template. The system default is:

Body: 

```
<p>Time: {{.Time}}
<p>Name: {{.Name}}
<p>Alerts:
{{range .Group}}
    <br>{{.}}
{{end}}
```

Subject:

```
{{.Name}}: {{.Group | len}} unknown alerts
```

The template for grouped unknowns can not be changed and is hard coded into Bosun and has the following body:

```
<p>Threshold of {{ .Threshold }} reached for unknown notifications. The following unknown
group emails were not sent.
<ul>
{{ range $group, $alertKeys := .Groups }}
    <li>
        {{ $group }}
        <ul>
            {{ range $ak := $alertKeys }}
            <li>{{ $ak }}</li>
            {{ end }}
        <ul>
    </li>
{{ end }}
</ul>
```

### Template Inclusions
Templates can include other templates that have been defined as in the example below. The templates are combined when rendered. This is useful to build reusable pieces in templates such as headers and footers. All templates with the same name are combined together across bosun templates, so you can reference them by the bosun template name you would like to include.

```
template include {
    body = `<p>This gets included!</p>`
    subject = `include example`
}

template includes {
    body = `{{ template "include" . }}`
    subject = `includes example`
}

alert include {
    template = includes
    warn = 1
}
```

### Template CSS
HTML templates will "inline" css. Since email doesn't support `<style>` blocks, an inliner ([douceur](https://github.com/aymerick/douceur)) takes a style block, and process the HTML to include those style rules as style attributes. 

Example:

```
template header {
    body = `
    <style>
        td, th {
            padding-right: 10px;
        }
    </style>
`
}

template table {
    body = `
    {{ template "header" . }}
    <table>
        <tr>
            <th>One</th>
            <!-- Will be rendered as:
            <th style="padding-right: 10px;">One</th> -->
            <th>Two</th>
        <tr>
            <td>This will have</td>
            <td>Styling applied when rendered</td>
        </tr>
    </table>
    `
    subject = `table example`
}

alert table {
    template = table
    warn = 1
}
```

### Template Keywords

#### body
{: .keyword}
The message body. This is always formated as HTML.

#### subject
{: .keyword}
The subject of the template. This is also the text that will be used in the dashboard for triggered incidents. The format of the subject is plaintext.

#### custom fields
{: .keyword}
Any other key/value pairs will add "custom" templates to the template. A notification may select these to send as its' content instead of just using subject/body. You can add any number of these as you like, with whatever name you choose.

### Template Variables
Template variables hold information specific to the instance of an alert. They are bound to the template's root context. That means that when you reference them in a block they need to be referenced differently just like context bound functions ([see template function types](/definitions#template-function-types))

#### Example of template variables
This example shows examples of the template variables documented below that are simple enough to be in a table:

```
alert vars {
    template = vars
    warn = avg(q("avg:rate:os.cpu{host=*bosun*}", "5m", ""))
}

template vars {
    body = `
    <!-- Examples of Variables -->
    <table>
        <tr>
            <th>Variable</th>
            <th>Example Value</th>
        </tr>
        <tr>
            <!-- Incident Id -->
            <td>Id</td>
            <td>{{ .Id }}</td>
        </tr>
        <tr>
            <!-- Start Time of Incident -->
            <td>Start</td>
            <td>{{ .Start }}</td>
        </tr>
        <tr>
            <!-- Alert Key -->
            <td>AlertKey</td>
            <td>{{.AlertKey}}</td>
        </tr>
        <tr>
            <!-- The Tags for the Alert instance -->
            <td>Tags</td>
            <td>{{.Tags}}</td>
        </tr>
        <tr>
            <!-- The rendered subject field of the template. -->
            <td>Subject</td>
            <td>{{.Subject}}</td>
        </tr>
        <tr>
            <!-- Boolean that is true if the alert has not been acknowledged -->
            <td>NeedAck</td>
            <td>{{.NeedAck}}</td>
        </tr>
        <tr>
            <!-- Boolean that is true if the alert is unevaluated (due to a dependency) -->
            <td>Unevaluated</td>
            <td>{{.Unevaluated}}</td>
        </tr>
        <tr>
            <!-- Status object representing current severity -->
            <td>CurrentStatus</td>
            <td>{{.CurrentStatus}}</td>
        </tr>
        <tr>
            <!-- Status object representing the highest severity -->
            <td>WorstStatus</td>
            <td>{{.WorstStatus}}</td>
        </tr>
        <tr>
            <!-- Status object representing the the most recent non-normal severity -->
            <td>LastAbnormalStatus</td>
            <td>{{.LastAbnormalStatus}}</td>
        </tr>
        <tr>
            <!-- Unix epoch (as int64) representing the time of LastAbnormalStatus -->
            <td>LastAbnormalTime</td>
            <td>{{.LastAbnormalTime}}</td>
        </tr>
        <tr>
            <!-- The name of the alert -->
            <td>Alert.Name</td>
            <td>{{.Alert.Name}}</td>
        </tr>
        <tr>
            <!-- Get Override Uknown Duration -->
            <td>Alert.Unknown</td>
            <td>{{.Alert.Unknown}}</td>
        </tr>
        <tr>
            <!-- Get Ignore Unknown setting for the alert (bool) -->
            <td>Alert.IgnoreUnknown</td>
            <td>{{.Alert.IgnoreUnknown}}</td>
        </tr>
        <tr>
            <!-- Get UnknownsNormal setting for the alert (bool) -->
            <td>Alert.UnknownsNormal</td>
            <td>{{.Alert.UnknownsNormal}}</td>
        </tr>
        <tr>
            <!-- Get UnjoinedOk setting for the alert (bool) -->
            <td>Alert.UnjoinedOK</td>
            <td>{{.Alert.UnjoinedOK}}</td>
        </tr>
        <tr>
            <!-- Get the Log setting for the alert (bool) -->
            <td>Alert.Log</td>
            <td>{{.Alert.Log}}</td>
        </tr>
        <tr>
            <!-- Get the MaxLogFrequency setting for the alert -->
            <td>Alert.MaxLogFrequency</td>
            <td>{{.Alert.MaxLogFrequency}}</td>
        </tr>
        <tr>
            <!-- Get the root template name -->
            <td>Alert.TemplateName</td>
            <td>{{.Alert.TemplateName}}</td>
        </tr>
        
    </table>
    `
    subject = `vars example`
}
```

#### .Actions
{: .var}
`.Actions` is a slice of of [action objects](/definitions#action) of actions taken on the incident. They are ordered by time from past to recent. This list will be empty when using Bosun's testing UI.

Example:

```
<table>
    <tr>
        <th>User</th>
        <th>Action Type</th>
        <th>Time</th>
        <th>Message</th>
    <tr>
    {{ range $action := .Actions }}
        <tr>
            <td>{{.User}}</th>
            <td>{{.Type}}</th>
            <td>{{.Time}}</th>
            <td>{{.Message}}</th>
        </tr>
    {{ end }}
</table>
```

#### .Alert.Crit
{: .var}
`.Alert.Crit` is a [bosun expression object](/definitions#expr) that maps to the crit expression in the alert. It is only meant to be used to display the expression, or run the expression by passing it to functions like `.Eval`.

Example:

```
template expr {
    body = `
        Expr: {{.Alert.Warn}}</br>
        <!-- note that an error from eval is not checked in this example, 
        see other examples for error checking -->
        Result: {{.Eval .Alert.Warn}}
    `
    subject = `expr example`
}

alert expr {
    template = expr
    warn = 1 + 1
    crit = 3
}
```

#### .Alert.Depends
{: .var}
Like `.Alert.Crit` but the [depends](/definitions#depends) expression.


#### .Alert.IgnoreUnknown
{: .var}
`.Alert.IgnoreUnknown` is a bool that will be true if [ignoreUnknown](/definitions#ignoreunknown) is set on the alert.

#### .Alert.Log
{: .var}
`.Alert.Log` is a bool that is true if this is a [log alert](/definitions#log).

#### .Alert.MaxLogFrequency
{: .var}
`.Alert.MaxLogFrequency` is a golang [time.Duration](https://golang.org/pkg/time/#Duration) that shows the [maxLogFrequency](/definitions#maxlogfrequency) settings for the alert.

#### .Alert.Name
{: .var}
`.Alert.Name` holds the the name of the alert. For example for an alert defined `alert myAlert { ... }` the value would be myAlert.

#### .Alert.RunEvery
{: .var}
`.Alert.RunEvery` is an integer that shows an alerts [runEvery](/definitions#runevery) setting.

#### .Alert.TemplateName
{: .var}
`.Alert.TemplateName` is the name of the template that the alert is configured to use.

#### .Alert.Text
{: .var}
`.Alert.Text` is the raw text of the alert definition as a string. It includes comments:

```
template text {
    body = `<pre>{{.Alert.Text}}</pre>`
    subject = `text example`
}

alert text {
    # A comment
    template = text
    warn = 1
}
```

#### .Alert.UnjoinedOk
{: .var}
`.Alert.UnjoinedOk` is a bool that is true of the [unjoinedOk](/definitions#unjoinedok) alert setting is set to true. This makes it so when doing operations with two sets, if there are items in one set that have no match they will be ignored instead of triggering an error.

#### .Alert.Unknown
{: .var}
`.Alert.Unknown` is a golang [time.Duration](https://golang.org/pkg/time/#Duration) that is the duration for unknowns if the alert uses the [unknown](/definitions#unknown) alert keyword to override the global duration. It will be zero if the alert is using the global setting.

#### .Alert.UnknownsNormal
{: .var}
`.Alert.UnknownsNormal` is a bool that is true [unknownIsNormal](/definitions#unknownisnormal) is set for the alert.

#### .Alert.Vars
{: .var}
`.Alert.Vars` is a map of string to string. Any variables declared in the alert definition get an entry in the map. The key is name of the variable without the dollar sign prefix, and the value is the text that the variable maps to (Variables in Bosun don't store results, and are just simple text replacement.) If the variable does not exist than an empty string will be returned. Global variables are only accessible via a mapping in the alert definition as show in the example below (or using the [V() template function](/definitions#vstring-string)).

Example:

```
$aGlobalVar = "Hiya"

template alert.vars {
    body = `
        {{.Alert.Vars.foo}}
        
        <!-- baz returns an empty string since it is not defined -->
        {{.Alert.Vars.baz}}
        
        <!-- Global vars don't work -->
        {{.Alert.Vars.aGlobalVar }}
        
        <!-- Workaround for Global vars -->
        {{.Alert.Vars.fakeGlobal }}
    `
    subject = `alert vars example`
}

alert alert.vars {
    template = alert.vars
    $foo = 1 + 1
    $fakeGlobal = $aGlobalVar
    warn = $foo
}
```

#### .Alert.Warn
{: .var}
Like the [`.Alert.Crit` template variable](/definitions#alertcrit) but the warning expression.

#### .AlertKey
{: .var}
`.AlertKey` is a string representation of the alert key. The alert key is in the format `alertname{tagset}`. For example `diskused{host=ny-bosun01,disk=/}`.

#### .Attachments
{: .var}
When the graph functions that generate images are used they are added to `.Attachments`. Although it is available, you should *not* need to access this variable from templates. It is a slice of pointers to Attachment objects. An attachment has three fields, Data (a byte slice), Filename (string), and ContentType string. 

#### .CurrentStatus
{: .var}
`.CurrentStatus` is a [status object](/definitions#status) representing the current severity state of the incident. This will be "none" when using Bosun's testing UI.

#### .Errors
{: .var}
A slice of strings that gets appended to when a [context-bound function](/definitions#context-bound) returns an error. 

#### .Events
{: .var}
The value of `.Events` is a slice of [Event](/definitions#event) objects.

Example:  

```
template test {
    body = `
    <table>
    
        <tr>
            <th>Time</th>
            <th>Status</th>
            <th>Warn Value</th>
            <th>Crit Value</th>
        </tr>
        
        {{ range $event := .Events}}
            <tr>
                <td>{{ $event.Time }}</td>
                <td>{{ $event.Status }}</td>
                <!-- Ensure Warn or Crit are not nil since they are pointers -->
                <td>{{ if $event.Warn }} {{$event.Warn.Value }} {{ else }} {{ "none" }} {{ end }}</td>
                <td>{{ if $event.Crit }} {{$event.Crit.Value }} {{ else }} {{ "none" }} {{ end }}</td>
            </tr>
        {{ end }}
    </table>
    `
    subject = `events example`
}
```

#### .Expr 
{: .var}
The value of `.Expr` is the warn or crit expression that was used to evaluate the alert in the format of a string. 

#### .Id
{: .var}
`.Id` is a unique number that identifies an incident in Bosun. It is an int64, see the documentation on the [lifetime of an incident](/usage#the-lifetime-of-an-incident) to understand when new incidents are created.

#### .IsEmail
{: .var}
The value of is `IsEmail` is true if the template is being rendered for an email. This allows you to use the same template for different types of notifications conditionally within the template.


#### .LastAbnormalStatus
{: .var}
`.LastAbnormalStatus` is a [status object](/definitions#status) representing the the most recent non-normal severity for the incident. This will be "none" when using Bosun's testing UI.

#### .LastAbnormalTime
{: .var}
`.LastAbnormalTime` is an int64 representing the time of `.LastAbnormalStatus`. This is not a time.Time object, but rather a unix epoch. This will be 0 when using Bosun's testing UI.


#### .NeedAck
{: .var}
`.NeedAck` is a boolean value that is true if the alert has not been acknowledged yet.

#### .Result
{: .var}
`.Result` is a pointer to a "result object". This is *not* the same as the [result object](/definitions#result-1) i.e. the object return by `.Eval`. Rather is has the following properties:

    * Value: The number returned by the expression (technically a float64)
    * Expr: The expression evaluated as a string

If the severity status is Warn that it will be the result of the Warn expression, or if it crit than it will be a pointer to the result of the crit expression.

Example:

```
template result {
    body = `
    {{ if notNil .Result }}
        {{ .Result.Expr }}
        {{ .Result.Value }}
    {{ end }}
    `
    subject = `result example`
}

alert result {
    template = result
    warn = 1
    crit = 2
}
```

#### .Start
{: .var}
`.Start` is the the time the incident started and a golang [time.Time](https://golang.org/pkg/time/#Time) object. This means you can work with the time object if you need to, but a simple `{{ .Start }}` will print the time in 

#### .Subject
{: .var}
`.Subject` is the rendered subject field of the template as a string. It is only available in the body, and does not show up via Bosun's testing UI.

#### .Tags
{: .var}
`.Tags` is a string representation of the tags for the alert. It is in the format of `tagkey=tagvalue,tag=tagvalue`. For example `host=ny-bosun01,disk=/`

#### .Unevaulated
{: .var}
`.Unevaluated` is a boolean value that is true if the alert did not trigger because of a [dependency](/definitions#depends). This field would only show true when viewed on the dashboard.

#### .WorstStatus
{: .var}
`.WorstStatus` is a [status object](/definitions#status) representing the highest severity reached in the lifetime of the incident. This will be "none" when using Bosun's testing UI.

### Template Function Types
Template functions come in two types. Functions that are global, and context-bound functions. Unfortunately, it is important to understand the difference because they are called differently in functions and have different behavior in regards to [error handling](/definitions#template-error-handling).

#### Global
Calling global functions is simple. The syntax is just the function name and arguments. I can be used in regular format or a chained pipe format.

```
template global_type_example {
    body = `
        <!-- Regular Format -->
        {{ bytes 12312313 }}
        <!-- Pipe Format -->
        {{ 12312313 | bytes }}
    `
    subject = `global example`
}
```

#### Context Bound
Context bound functions, like Global functions, can be called in regular or pipe format. What makes them different is the syntax used to call them. Context bound functions have a period before them, such as `{{ .Eval }}`. Essentially they are methods on that act on the instance of an alert/template combination when it is rendered and perform queries.

They are bound to the parent context. The "parent context" is essentially the top level namespace within a template. This is because they access data associated with the instance of the template when it sent. 

What this practically means, is that when you are inside a block within a template (for example, inside a range look) context-bound functions need to be called with `{{ $.Func }}` to reference the parent context.

```
template context_bound_type_example {
    body = `
        <!-- Context Bound at top level -->
        {{ .Eval .Alert.Vars.avg_cpu }}
        
        <!-- Context Bound in Block -->
        {{ range $x := .Events }}
            {{ $.Eval $.Alert.Vars.avg_cpu }}
        {{ end }}
    `
    subject = `context bound example`
}
```

### Template Error handling
Templates can throw errors at runtime (i.e. when a notification is sent). Although the configuration check makes sure that templates are valid, you can still do things like try to reference objects that are nil pointers.

When a template fails to render:

 * Email: A generic notification will be emailed to the people that would have received the alert.
 * Post notification (where the subject is used): The following text will be posted `error: template rendering error for alert <alertkey>` where the alert key is something like `os.cpu{host=a}`

In order to prevent the template from completely failing and resulting in the generic notification, errors can be handled inside the application. 

Errors are handled differently depending on the [type of the function](/definitions#template-function-types) (Context Bound vs Global). When context bound functions have errors the error string is appended to the [`.Errors` template variable](/definitions#errors). This is not the case for global functions. 

Global functions always returns strings except for parseDuration. When global functions error than `.Errors` is *not* appended to, but the string that would have been returned with an error is show in the template. parseDuration returns nil when it errors, and in this one exception you can't see what the error is.

If the function returns a string or an image (technically an interface) the error message will be displayed in the template. If an object is returned (i.e. a result sets, a slice, etc) nil is returned and the user can check for that in the template. In both cases `.Errors` will be appended to if it is a context bound functions.

See the examples in the functions that follow to see examples of Error handling. 

### Template Functions

#### Context-Bound Functions

##### .Ack() (string)
{: .func}

`.Ack` creates a link to Bosun's view for alert acknowledgement. This is generated using the [system configuration's Hostname](/system_configuration#hostname) value as the root of the link.


##### .UseElastic(host string)
{: .func}

`.UseElastic` set the ElasticHost context object which is used in ESQuery and ESQueryAll functions mentioned below.

Querying [foo](system_configuration#example-2) cluster:

```
template test {
        subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}
        body = `
            {{ $filter := (.Eval .Alert.Vars.filter)}}
            {{ $index := (.Eval .Alert.Vars.index)}}
            {{ .UseElastic "foo" }}
            {{range $i, $x := .ESQuery $index $filter "5m" "" 10}}
                <p>{{$x.machinename}}</p>
            {{end}}
        `
}
``` 

##### .ESQuery(indexRoot expr.ESIndexer, filter expr.ESQuery, sduration, eduration string, size int) ([]interface{})
{: .func}

`.ESQuery` returns a slice of elastic documents. The function behaves like the escount and esstat [elastic expression functions](/expressions#elastic-query-functions) but returns documents instead of statistics about those documents. The number of documents is limited to the provided size argument. Each item in the slice is the a document that has been marshaled to a golang interface{}. This means the contents are dynamic like elastic documents. If there is an error, than nil is returned and `.Errors` is appended to. 

The group (aka tags) of the alert is used to further filter the results. This is implemented by taking each key/value pair in the alert, and adding them as an [elastic term query](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html).

Example:

```
template esquery {
    body = `
        {{ $filter := (.Eval .Alert.Vars.filter)}}
        {{ $index := (.Eval .Alert.Vars.index)}}
        {{ $esResult := .ESQuery $index $filter "5m" "" 10 }}
        {{ if notNil $esResult }}
            {{ range $row := $esResult }}
                <p>{{ range $key, $value := $row }}
                    <!-- Show the Key, Value, and the Type of the value. Values could be objects
                    but dereferencing their properties if they don't exist could cause the template
                    to fail to render -->
                    {{ $key }}: {{ $value }} ({{ $value | printf "%T" }}),
                {{ end }}<p> 
            {{ end }}
        {{ else }}
            <p>{{ .LastError }}
        {{end}}
    `
    subject = `esquery example`
}

alert esquery {
    template = esquery
    $index = esls("logstash")
    $filter = esregexp("logsource", "ny-.*")
    crit = avg(escount($index, "logsource", $filter, "2m", "10m", ""))
}
```

<div class="admonition warning">
<p class="admonition-title">Warning</p>
<p>Currently ESQuery and ESQueryAll do not have a short timeout than the timeouts for elastic expressions. Therefore be aware that using these functions could slow down template processing since templates are procedural.</p>
</div>

##### .ESQueryAll(indexRoot expr.ESIndexer, filter expr.ESQuery, sduration, eduration string, size int) (interface{})
{: .func}

`.ESQueryAll` behaves just like `.ESQuery`, but the tag filtering to filter results to match the alert instance is *not* applied.

##### .Eval(string|Expression) (resultValue)
{: .func}

Executes the given expression and returns the first result that includes the tag key/value pairs of the alert instance. In other words, it evaluates the expression within the context of the alert. So if you have an alert that could trigger multiple incidents (i.e. `host=*`) then the expression will return data specific to the host for this alert.

If the expression results in an error `nil` will be returned and `.Errors` will be appended to. If the result set is empty, than `NaN` is returned. Otherwise the value of the first matching result in the set is returned. That result can be any type of value that Bosun can return since the type returned by Eval is dependent on what the return type of the expression it evaluates. Mostly commonly it will be float64 when used to evaluate an expression that is enclosed in a reduction like in the following example:

```
alert eval {
    template = eval
    $series = merge(series("host=a", 0, .2), series("host=b", 0, .5))
    $r = avg($series)
    crit = $r
}

template eval {
    body = `
    {{$v := .Eval .Alert.Vars.r }}
    <!-- If $v is not nil (which is what .Eval returns on errors) -->
    {{ if notNil $v }}
        {{ $v }}
    {{ else }}
        {{ .LastError }}
    {{ end }}
`
    subject = `eval example`
}
```

The above would display "0.2" for host "a". More simply, you template could just be `{{.Eval .Alert.Vars.r}}` and it would display 0.2 assuming there are no errors.

<div class="admonition">
<p class="admonition-title">Info</p>
<p>The "filtering" implementation currently behaves differently with OpenTSDB for Eval and Graph. The query text will actually be replaced so that it only queries that tags for the alert. This behavior may be removed in the future.</p>
</div>

##### .EvalAll(string|Expression|ResultSlice) (Result)
{: .func}

`.EvalAll` executes the given expression and returns a slice of [ResultSlice](/definitions#resultslice). The type of each results depends on the return type of the expression. Mostly commonly one uses a an expression that returns a numberSet ([see data types](/expressions#data-types)). If there is an error nil is returned and `.Errors` is appended to.

Example:

```
alert evalall {
    template = evalall
    # the type returned by the avg() func is a seriesSet
    $cpu = q("sum:rate{counter,,1}:os.cpu{host=ny-bosun*}", "5m", "")
    # the type returned by the avg() func is a numberSet
    $cpu_avg = avg($cpu)
    warn = 1
}

template evalall {
    body = `
    {{ $numberSetResult := .EvalAll .Alert.Vars.cpu_avg }}
    {{ if notNil $numberSetResult }}
    <table>
        <tr>
            <th>Host</th>
            <th>Avg CPU Value</th>
        <tr>
        {{ range $result := $numberSetResult }}
            <tr>
                <!-- Show the value of the host tag key -->
                <td>{{$result.Group.host}}</td>
                <!-- Since we know the value of cpu_avg is a numberSet which contains floats.
                we pipe the float to printf to make it so it only print to two decimal places -->
                <td>{{$result.Value | printf "%.2f"}}</td>
            </tr>
        {{end}}
    <table>
    {{ else }}
        {{ .LastError }}
    {{ end }}
    
    <!-- You could end up with other types, but their usage is not recommended, but to illustrate the point working with a seriesSet is show -->
    {{ $seriesSetResult := .EvalAll .Alert.Vars.cpu }}
    {{ if notNil $seriesSetResult }}
        {{ range $result := $seriesSetResult }}
            <h2>{{ $result.Group }}</h2>
            <table>
                <tr>
                    <th>Time</th>
                    <th>Value</th>
                <tr>
                <!-- these are *not* sorted -->
                {{ range $time, $value := $result.Value }}
                    <tr>
                        <td>{{ $time }}</td>
                        <td>{{ $value }}</td>
                    </tr>
                {{end}}
            <table>
        {{ end }}
    {{ else }}
        {{ .LastError }}
    {{ end }}
    
    `
    subject = `evalall example`
}
```


##### .GetMeta(metric, key string, tags string|TagSet) (object|string)
{: .func}

`.GetMeta` fetches information from Bosun's metadata store. This function returns two types of metadata: metric metadata and metadata attached to tags. If the metric argument is a non-empty string, them metric metadata is fetched, otherwise tag metadata is fetched. 

For both Metric and Tag metadata, in case where the key is a non-empty string then a string will be returned which will either be the value or an error. In cases where key is an empty string, a slice of objects is returned unless there is an error in which case nil is returned. The example shows these cases.

When a slice of objects are returned, the objects have the following properties:

 * `Metric`: A string representing the metric name
 * `Tags`: A map of tag keys to tag values (string[string]) for the metadata
 * `Name`: The key of the metadata (same as key argument to this function, if provided)
 * `Value`: A string
 * `Time`: Last time this Metadata was updated

For Metric metadata the Tags field will be empty, and for tag metadata the metric field is empty. 

For Tag metadata, metadata is returned that includes the key/value pairs provided as an argument. So for example, `host=a` would also return metadata that is tagged `host=a,interface=eth0`.


Example:

```
alert meta {
    template = meta
    $metric = os.mem.free
    warn = avg(q("avg:rate:$metric{host=*bosun*}", "5m", ""))
}

template meta {
    body = `
        <h1>Metric Metadata</h1>
        <!-- Metric Metadata as slice -->
        {{ $metricMetadata := .GetMeta .Alert.Vars.metric "" "" }}
        {{ if notNil $metricMetadata }}
            <h2>Metric Metadata as Slice</h2>
            <table>
                <tr>
                    <th>Property</th>
                    <th>Value</th>
                </tr>
                {{ range $prop := $metricMetadata }}
                    <tr>
                        <td>{{ $prop.Name }}</td>
                        <td>{{ $prop.Value }}</td>
                    </tr>
                {{ end }}
            </table>
        {{ else }}
            {{ .LastError }}
        {{ end }}
        
        <h2>Metric Metadata as string values</h2>
        <!-- Metric Metadata as strings (specific keys) -->
        Desc: {{ .GetMeta .Alert.Vars.metric "desc" "" }}</br>
        Unit: {{ .GetMeta .Alert.Vars.metric "unit" "" }}</br>
        RateType: {{ .GetMeta .Alert.Vars.metric "rate" "" }}</br>
        
        <h1>Tag Metadata<h1>
        <h2>Tag Metadata as slice</h2>
        {{ $tagMeta := .GetMeta "" "" "host=ny-web01" }}
        {{ if notNil $tagMeta }}
            <table>
                <tr>
                    <th>Property</th>
                    <th>Tags</th>
                    <th>Value</th>
                    <th>Last Touched Time</th>
                </tr>
                {{ range $metaSend := $tagMeta }}
                    <tr>
                        <td>{{ $metaSend.Name }}</td>
                        <td>{{ $metaSend.Tags }}</td>
                        <td>{{ $metaSend.Value }}</td>
                        <td>{{ $metaSend.Time }}</td>
                    </tr>
                {{ end }}
            </table>
        {{ else }}
            {{ .LastError }}
        {{ end }}
        
        <h2>Keyed Tag Metadata</h2>
        <!-- Will return first match -->
        {{ $singleTagMeta := .GetMeta "" "memory" "host==ny-web01" }}
        {{ if notNil $singleTagMeta }}
            {{ $singleTagMeta }}
        {{ else }}
            {{ .LastError }}
        {{ end }}
    `
    subject = `meta example`
}
```

##### .Graph(string|Expression, yAxisLabel string) (image)
{: .func}

Creates a graph of the expression. It will error (that can not be handled) if the return type of the expression is not a `seriesSet` ([see data types](/expressions#data-types)). If the expression is a an OpenTSDB query, it will be auto downsampled so that there are approx no more than 1000 points per series in the graph. Like `.Eval`, it filters the results to only those that include the tag key/value pairs of the alert instance. In other words, in the example, for an alert on `host=a` only the series for host a would be graphed.

If the optional yAxisLabel argument is provided it will be shown as a label on the y axis.

When the rendered graph is viewed in Bosun's UI (either the config test UI, or the dashboard) than the Graph will be an SVG. For email notifications the graph is rendered into a PNG. This is because most email providers don't allow SVGs embedded in emails.

If there is an error executing `.Graph` than a string showing the error will be returned instead of an image and `.Errors` will be appended to.

Example:

```
alert graph {
    template = graph
    $series = merge(series("host=a", 0, 1, 15, 2, 30, 3), series("host=b", 0, 2, 15, 3, 30, 1))
    $r = avg($series)
    crit = $r
}

template graph {
    body = `
    {{$v := .Graph .Alert.Vars.series "Random Numbers" }}
    <!-- If $v is not nil (which is what .Graph returns on errors) -->
    {{ if notNil $v }}
        {{ $v }}
    {{ else }}
        {{ .LastError }}
    {{ end }}
`
    subject = `graph example`
}
```

##### .GraphAll(string|Expression, yAxisLabel string) (image)
{: .func}

`.GraphAll` behaves exactly like the [`.Graph` function](/definitions#graphstringexpression-unit-string-image) but does not filter results to match the tagset of the alert. So if you changed the call in the example for `.Graph` to be `.GraphAll`, in an alert about `host=a` the series for both host a and host b would displayed (unlike Graph where only the series for host a would be displayed). 

##### .GraphLink(string) (string)
{: .func}

`.GraphLink` creates a link to Bosun's rule editor page. This is useful to provide a quick link to the view someone would use to edit the alert. This is generated using the [system configuration's Hostname](/system_configuration#hostname) value as the root of the link. The link will set the the alert, which template should be rendered, and time on the rule editor page. The time that represents "now" will be the time of the alert. The rule editor's alert will be set to point to the alert definition that corresponds to this alert. However, it will always be loading the current definitions, so it is possible that the alert or template definitions will have changed since the template was rendered.

##### .Group() (TagSet)
{: .func}

A map of tags keys to their corresponding values for the alert.

##### .HTTPGet(url string) string
{: .func}

`.HTTPGet` fetches a url and returns the raw text as a string, unless there is an error or an http response code `>= 300` in which case the response code or error is displayed and `.Errors` is appended to. The client will identify itself as Bosun via the user agent header. Since templates are procedural and this meant to for fetching extra information, the timeout is set to five seconds. Otherwise this function could severely slow down the delivery of notifications.

Example:

```
template httpget {
    body = `
    {{ .HTTPGet "http://localhost:9090"}}
    `
    subject = `httpget example`
}

alert httpget {
    template = httpget
    warn = 1
}
```

##### .HTTPGetJSON(url string) (*jsonq.JsonQuery)
{: .func}

(TODO: Document, link to jsonq library and how to work with objects. Note limitation about top level object being an array)

##### .HTTPPost(url, bodyType, data string) (string)
{: .func}

`.HTTPPost` sends a HTTP POST request to the specified url. The data is provided a string, and bodyType will set the Content-Type HTTP header. It will return the response or error as a string in the same way that the [`.HTTPGet` template function](/definitions#httpgeturl-string-string) behaves. It will also shares [`.HTTPGet`'s](/definitions#httpgeturl-string-string) timeout.

Example:

```
template httppost {
    body = `
    {{ .HTTPPost "http://localhost:9090" "application/json" "{ \"Foo\": \"bar\" }" }}
    `
    subject = `httppost example`
}

alert httppost {
    template = httppost
    warn = 1
}
```

##### .Incident() (string)
{: .func}

`.Incident` creates a link to Bosun's incident view. This is generated using the [system configuration's Hostname](/system_configuration#hostname) value as the root of the link.

##### .Last() (string)
{: .func}

The most recent [Event](/definitions#event) in the `.History` array. This does not return any events when using Bosun's testing UI.

##### .LastError() (string)
{: .func}

Returns the string representation of the last Error from the [`.Errors` alert variable](/definitions#errors), or an empty string if there are no errors. This only contains errors from context-bound functions.

##### .LeftJoin(expression|string) ([][]Result)
{: .func}

`LeftJoin` allows you to construct tables from the results of multiple expressions. `LeftJoin` takes two or more expressions that return numberSets as arguments. The function evaluates each expression. It then joins the results of other expressions to the first expression. The join is based on the tag sets of the results. If the tagset is a subset or equal the results of the first expression, it will be joined. 

The output can be thought of as a table that is structured as an array of rows, where each row is an array. More technically it is a slice of slices that point to [Result](/definitions#result) objects where each result will be a numberSet type.

If the expression results in an error nil will be returned and `.Errors` will be appended to.

Example:

```
alert leftjoin {
    template = leftjoin
    # Host Based
    $osDisksMinPercentFree = last(q("min:os.disk.fs.percent_free{host=*}", "5m", ""))
    
    # Host and Disk Based
    $osDiskPercentFree = last(q("sum:os.disk.fs.percent_free{disk=*,host=*}", "5m", ""))
    $osDiskUsed = last(q("sum:os.disk.fs.space_used{disk=*,host=*}", "5m", ""))
    $osDiskTotal = last(q("sum:os.disk.fs.space_total{disk=*,host=*}", "5m", ""))
    $osDiskFree = $osDiskTotal - $osDiskUsed
    
    #Host Based Alert
    warn = $osDiskPercentFree > 5
}

template leftjoin {
    body = `    
    <h3>Disk Space Utilization</h3>
    {{ $joinResult := .LeftJoin .Alert.Vars.osDiskPercentFree .Alert.Vars.osDiskUsed .Alert.Vars.osDiskTotal .Alert.Vars.osDiskFree }}
    <!-- $joinResult will be nill if there is an error from .LeftJoin -->
    {{ if notNil $joinResult }}    
        <table>
        <tr>
            <th>Mountpoint</th>
            <th>Percent Free</th>
            <th>Space Used</th>
            <th>Space Free</th>
            <th>Space Total</th>
        </tr>
        <!-- loop over each row of the result. In this case, each host/disk -->
        {{ range $x := $joinResult }}
            <!-- Each column in the row is the results in the same order as they 
            were passed to .LeftJoin. The index function is built-in to Go's template
            language and gets the nth element of a slice (in this case, each column of 
            the row -->
            {{ $pf :=  index $x 0}}
            {{ $du :=  index $x 1}}
            {{ $dt :=  index $x 2}}
            {{ $df :=  index $x 3}}
            <!-- .LeftJoin is like EvalAll and GraphAll in that it does not filter
            results to the tags of the alert instance, but rather returns all results.
            So we compare the result's host to that of the host for the alert to only
            show disks related to the host that the alert is about. -->
            {{ if eq $pf.Group.host $.Group.host }}
                <tr>
                    <td>{{$pf.Group.disk}}</td>
                    <td>{{$pf.Value | pct}}</td>
                    <td>{{$du.Value | bytes }}</td>
                    <td>{{$df.Value | bytes }}</td>
                    <td>{{$dt.Value | bytes}}</td>
                </tr>
            {{end}}
        {{end}}
    {{ else }}
        Error Creating Table: {{ .LastError }}
    {{end}}
    `
    subject = `leftjoin example`
}
```

##### .Lookup(table string, key string) (string)
{: .func}

`.Lookup` returns a string to the corresponding value in a [lookup table](/definitions#lookup-tables) given the table and key. It uses the tags of the alert instance as the tags used against the lookup tables.

See the [main lookup example](/definitions#main-lookup-example) for example usage in a template.

##### .LookupAll(table string, key string, tags string|tagset) (string)
{: .func}

`.LookupAll` behaves like `Lookup` except that you specify the tags. The tags can me a string such as `"host=a,dc=us"` or can be a tagset (i.e. the return of the [.Group](/definitions#group-tagset) template function).

See the [main lookup example](/definitions#main-lookup-example) for example usage in a template.

#### Global Functions

##### bytes(string|int|float) (string)
{: .func}

`bytes` converts a number of bytes into a human readable number with a postfix (such as KB or MB). Conversions are base ten and not base two.

Example:

```
template bytes {
    body = `
        <!-- bytes uses base ten, *not* base two -->
        {{ .Alert.Vars.ten | bytes }},{{ .Alert.Vars.two | bytes }}
        <!-- results are 97.66KB,1000.00KB -->
    `
    subject = `bytes example`
}

alert bytes {
    template = bytes
    $ten = 100000
    $two = 1024000
    warn = $ten
}
```

##### html(string) (htemplate.HTML)
{: .func}

`html` takes a string and makes it part of the template. This allows you to inject HTML from variables set in the alert definition. A use case for this is when you have many alerts that share a template and fields you have standardized. For example, you might have a `$notes` variable that you attach to all alerts. This way when filling out the notes for an alert, you can include things like HTML links.

Example:

```
template htmlFunc {
    body = `All our templates always show the notes: <!-- the following will be rendered as subscript -->
    {{ .Alert.Vars.notes | html }}`
    subject = `html example`
}

alert htmlFunc {
    template = htmlFunc
    $notes = <sub>I'm ashmaed about the reason for this alert so this is in subscript...</sub>. In truth, the solution to this alert is the solution to all technical problems, go ask on <a href="https://stackoverflow.com" target="blank">StackOverflow</a>
    warn = 1 
}
```

##### notNil(value) (bool)
{: .func}

`notNil` returns true if the value is nil. This is only meant to be used with error checking on context-bound functions.


##### parseDuration(string) (*time.Duration)
{: .func}

`parseDuration` maps to Golang's [time.ParseDuration](http://golang.org/pkg/time/#ParseDuration). It returns a pointer to a time.Duration. If there is an error nil will be returned. Unfortunately the error message for this particular can not be seen.

Example:

```
template parseDuration {
    body = `
        <!-- More commonly you would use .Last.Time.Add , but .Last does not function in the testing interface -->
        Doomsday: {{ .Start.Add (parseDuration (.Eval .Alert.Vars.secondsUntilDoom | printf "%fs"))}}
        <!-- result is: Doomsday: 2021-02-11 15:11:06.727332631 +0000 UTC -->
    `
    subject = `parseDuration example`
}

alert parseDuration {
    template = parseDuration
    $secondsUntilDoom = 123453245
    warn = $secondsUntilDoom
} 
```

##### pct(number) (string)
{: .func}

`pct` formats a number as percent. It preserves two decimal places and adds a "%" suffix. It does not do any calculations (in other words, it does *not* multiply the number by 100).

Example:

```
template pct {
    body = `
    <!-- Need to eval to get number type instead of string -->
    {{ .Eval .Alert.Vars.value | pct }}
    <!-- result is: 55.56% -->
    `
    subject = `pct example`
}

alert pct {
    template = pct
    $value = 55.55555
    warn = $value
}
```

##### replace(s, old, new string, n int) (string)
{: .func}

`replace` maps to golang's [strings.Replace](http://golang.org/pkg/strings/#Replace) function. Which states:

> Replace returns a copy of the string s with the first n non-overlapping instances of old replaced by new. If old is empty, it matches at the beginning of the string and after each UTF-8 sequence, yielding up to k+1 replacements for a k-rune string. If n < 0, there is no limit on the number of replacements

Example:

```
template replace {
    body = `
    {{ replace "Foo.Bar.Baz" "." " " -1 }}
    <!-- result is: Foo Bar Baz -->
    `
    subject = `replace example`
}

alert replace {
    template = replace
    warn = 1
}
```

##### short(string) (string)
{: .func}

`short` Trims the string to everything before the first period. Useful for turning a FQDN into a shortname. For example: `{{short "foo.baz.com"}}` in a template will return `foo`

##### V(string) (string)
{: .func}

The `V` func allows you to access [global variables](/definitions#global-variables) from within templates. This does not recognize variables defined in alerts.

Example:

```
$myGlobalVar = Kon'nichiwa
$overRide = I shall be seen

template globalvar {
    body = `
    <p>{{ V "$myGlobalVar" }}</p>
    <!-- renders to Kon'nichiwa -->
    <p>{{ .Alert.Vars.myGlobalVar }}</p>
    <!-- renders an empty string -->
    <p>{{ V "$overRide" }}</p>
    <!-- render to "I shall be seen" since expression variable overrides do *not* work in templates -->
`
    subject = `V example`
}

alert globalvar {
    template = globalvar
    $overRide = I am not seen
    warn = 1
}
```

### Types available in Templates
Since templating is based on Go's template language, certain types will be returned. Understanding these types can help you construct richer alert notifications.

#### Action
{: .type}

An Action is an object that represents actions that people do on an incident. It has the following fields:

 * `User`: a string of the username for the person that took the action
 * `Message`: a string of an optional message that a user added to the action when taking it
 * `Time`: a time.Time object representing the time the action it was taken
 * `ActionType`: an int representing the type of action. The values and their strings are:
   * 0: "none"
   * 1: "Acknowledged"
   * 2: "Closed"
   * 3: "Forgotten"
   * 4: "ForceClosed"
   * 5: "Purged"
   * 6: "Note"

Example usage can be seen under the [`.Actions` template variable](/definitions#actions).

#### Event
{: .type}

An Event represent a change in the [severity state](/usage#severity-states) within the [duration of an incident](/usage#the-lifetime-of-an-incident). When an incident triggers, it will have at least one event.  An Event contains the following fields

 * `Warn`: A pointer to an [Event Result](definitions#event-result) that the warn expression generated if the event has a warning status.
 * `Crit`: A pointer to an [Event Result](definitions#event-result) if the event has a critical status.
 * `Status`: An integer representing the current severity (normal, warning, critical, unknown). As long as it is printed as a string, one will get the textual representation. The status field has identification methods: `IsNormal()`, `IsWarning()`, `IsCritical()`, `IsUnknown()`, `IsError()` which return a boolean.
 * `Time`: A [Go time.Time object](https://golang.org/pkg/time/#Time) representing the time of the event. All the methods you find in Go's documentation attached to time.Time are available in the template
 * `Unevaluated`: A boolean value if the alert was unevaluated. Alerts on unevaluated when the current was using the [`depends` alert keyword](/definitions#depends) to depend on another alert, and that other alert was non-normal. 

It is important to note that the `Warn` and `Crit` fields are pointers. So if there was no `Warn` result and you attempt to access a property of `Warn` then you would get a template error at runtime. Therefore when referecing any fields of `Crit` or `Warn` such as `.Crit.Value`, it is vital that you ensure the `Warn` or `Crit` property of the Event is not a nil pointer first.

 See the example under the [`.Events` template variable](/definitions#events) to see how to use events inside a template.

#### Event Result 
{: .type}

An Event Result (note: in the code this is actually a `models.Result`) has two properties:

* `Expr`: A string representation of the full expression used to generate the value of the Result's Value.
* `Value`: A float representing the calculated result of the expression.

There is a third property **Computations**. But it is not recommended that you access it even though it is available and it will not be documented.

#### Expr
{: .type}

A `.Expr` is a bosun expression. Although various properties and methods are attached to it, it should only be used for printing (to see the underlying text) and for passing it to function that evaluate expressions such as [`.Eval`](/definitions#evalstringexpression-resultvalue) within templates.

#### Result
{: .type}

A `Result` as two fields:

 1. `Group`: The Group is the TagSet of the result. 
 2. `Value`: The Value of the Result

A tagset is a map of of string to string (`map[string]string` in Go). The keys represent the tag key and their corresponding values are the tag values.

The Value can be of different types. Technically, it is a go `interface{}` with two methods, `Type()` which returns the type of the `Value()` which returns an `interface{}` as well, but will be of the type returned by the `Type()` method.

The most common case of dealing with Results in a ResultSlice is to use the `.EvalAll` func on an expression that would return a NumberSet. See the example under [EvalAll](/definitions#evalall).

#### ResultSlice
{: .type}

A `ResultSlice` is returned by using the [`.EvalAll` template function](/definitions#evalallstringexpressionresultslice-result). It is a slice of pointers to [`Result` objects](/definitions#result-1). Each result represents the an item in the set when the type is something like a NumberSet or a SeriesSet.

#### Status 
{: .type}

The `Status` type is an integer that represents the current severity status of the incident with associated string repsentation. The possible values and their string representation is:

 * 0 for "none"
 * 1 for "normal"
 * 2 for "warning"
 * 3 for "critical"
 * 4 for "unknown"

#### TagSet
A `TagSet` (technically an `opentsdb.TagSet`, but is not actually particular to OpenTSDB) a map of key values to key tags. Both the value and key are strings (`map[string]string`). 

## Notifications
Notifications are referenced by alerts via [warnNotification](/definitions#warnnotification) and [critNotification](/definitions#critnotification) keywords. They specify actions (such as email) to perform when incidents change severity or user performed actions on incidents (See the [lifetime of an incident](/usage#the-lifetime-of-an-incident)). When actions execute they use the templates that are pointed to by the alert that references the notification.

Notifications are independent of each other and executed concurrently (if there are many notifications for an alert, one will not block the other).

More specific documentation on how to fully customize notifications can be found on [this page](/notifications).

### Chained Notifications
Notifications can also be chained to other notifications (or even itself) using the optional `next` and `timeout` notification keywords. Chained notifications will execute until an alert is acknowledged or closed. 

### Notification keywords

#### bodyTemplate
{: .keyword}
Specify a template name to use for the notification body. Default is `body`, or for email notifications `emailBody` if it is present.

#### contentType
{: .keyword}

If your body for a POST notification requires a different Content-Type header than the default of `application/x-www-form-urlencoded`, you may set the `contentType` variable.

#### email
{: .keyword}

`email` is a list of email addresses. The format is comma separated email addresses in the format of either `Person Name <addr@domain.com>` or `addr@domain.com`. When this is specified emails are enabled. They will use the subject and body fields of the template that the alert references.

#### emailSubjectTemplate
{: .keyword}
Specify a template name to use for the email subject. Defualts to `emailSubject`, or just `subject` if the template doesn't have one.

#### get
{: .keyword}

`get` will make an HTTP get call to the url provided as a value.

#### getTemplate
{: .keyword}

`getTemplate` will use the specified template as a URL, and will make an HTTP request call to it.

#### groupActions
{: .keyword}
chooses whether or not multiple actions performed at once (like a user acking multiple alerts), should be sent as one notification, or as many. Default is `true`. Set to `false` to get one notification per alert key.

#### next
{: .keyword}

`next` is name of next notification to execute after `timeout` and is how you construct notification chains. It can be itself.

#### post
{: .keyword}

`post` will send an HTTP post to specified url. The subject field of the template referenced from the alert is sent as the request body. Content type is set as `application/x-www-form-urlencoded` by default, but may be overriden by setting the `contentType` variable for the notification.

#### postTemplate
{: .keyword}

`postTemplate` will use the specified template as a URL, and will make an HTTP post request to it.

#### print
{: .keyword}

`print` Prints template subject to stdout. The value of `print` is ignored, so just use: `print = true`. 

#### runOnActions
{: .keyword}
Specifies which actions types this notification will run on. If set to `all` or `true`, will send all actions. If set to `none` or `false`, it will send on none.
Otherwise, this should be a comma-seperated list of action types to include, from `Ack`, `Close`, `Forget`, `ForceClose`, `Purge`, `Note`, `DelayedClose`, or `CancelClose`.

#### timeout
{: .keyword}

`timeout` is the duration to wait until the notification specified in `next` is executed. If `next` is specified without a `timeout` then it will happen immediately.

#### unknownMinGroupSize
{: .keyword}
Minimum grouping of unknown alert keys that should be sent together. Bosun will try to group related unkowns by common tags if it has at least this many. Set to `0` to disable grouping, and send a notification per alert key.

#### unknownThreshold
{: .keyword}

Maximum number of unknown notifications to send in a single 'batch'. After this many are sent, bosun will send the remainder of the unkown notifications in a single notification using the "multiple unknown groups" template. Set to `0` to specify no limit.

#### action templates
{: .keyword}
You can specify templates to use for actions by setting keys of the form ``action{TemplateType}{ActionType?}`

Where "templateType" is one of `Body`, `Get`, `Post`, or `EmailSubject`, and "ActionType" if present, is one of `Ack`, `Close`, `Forget`, `ForceClose`, `Purge`, `Note`, `DelayedClose`, or `CancelClose`. If Action Type is not specified, it will apply to all actions types, unless specifically overridden.

If nothing is specified for an action type, a built-in template will be used.

See [this page](/notifications) for more details on customizing action notifications.

#### unknown templates
{: .keyword}

Set `unknownBody`, `unknownPost`, `unknownGet`, and `unknownEmailSubject`

or

`unknownMultiBody`, `unknownMultiPost`, `unknownMultiGet`, and `unknownMultiEmailSubject`.

to control which template is used for unknown notifications. If not specified, default built-in templates will be used.

See [this page](/notifications) for more details on customizing unknown notifications.

### Notification Examples

```
# HTTP Post to a chatroom, email in 10m if not ack'd
notification chat {
	next = email
	timeout = 10m
	post = http://chat.example.com/room/1?key=KEY&message=whatever
}

# email foo and bar each day until ack'd
notification email {
	email = foo@example.com, bar@example.com
	next = email
	timeout = 1d
}



# post to a slack.com chatroom via Incoming Webhooks integration
notification slack{
	post = https://hooks.slack.com/services/abcdef
	bodyTemplate = slackBody
}

template slack {
    slackBody = {"text": "{{.Subject}}"}
    jsonBody = {"text": "{{.Subject}}", "apiKey"="2847abc23"}
}

#post json
notification json{
	post = https://someurl.com/submit
	body = jsonBody
	contentType = application/json
}
```

## Lookup tables
Lookup tables are tables you create that store information about tags. They can be used in 3 main ways:

 1. To set different values (i.e. thresholds) in expressions for different tags in a response set
 2. To change the notification of an alert based on the tags in the response (notification lookups)
 3. To associate information with tags that can be referenced from templates

Lookup tables are named, then have entries based on Tags, and within each entry there are arbirary key / value pairs. To access the data in expressions either the [lookup](/expressions#lookuptable-string-key-string-numberset) or [lookupSeries](/expressions#lookupseriesseries-seriesset-table-string-key-string-numberset) expression functions are used.

When using notification lookups, the "lookup" function is actually different from the expression lookup function behind the scenes. Users using `lookupSeries` in expressions should still use `lookup` when defining notification lookups.

The syntax is:

```
lookup <tableName> {
    entry <tagKey=(tagValue|glob),(tagKey=...) {
        <key> = <value>
        <key2> = <value>
        ...
    }
    entry ...
}
```

Multiple tags can be used:

```
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
```

### Main Lookup Example
This example shows all three uses of lookups: expression value switching, notification lookups, and lookup usage in templates.

```
notification uhura {
    print = true
}

notification spock {
    print = true
}

lookup exampleTable {
    entry host=a {
        threshold = 9
        fish = Ohhh... a Red Snapper - Hmmm... Very Tasty
        contact_crew = spock
    }
    # You took the Box! Lets see whats in the box! 
    entry host=* {
        threshold = 3
        fish = Nothing! Absolutely Nothing! Stupid! You so Stupid!
        contact_crew = uhura
    }
}

alert exampleTable {
    template = lookup
    $series = merge(series("host=a", 0, 10), series("host=b", 0, 2))
    $r = avg($series)
    
    # lookup depends on Bosun's index of datapoints to get possible tag values
    $lk = $r > lookup("exampleTable", "threshold")
    
    # lookupSeries uses the series to get the possible tag values
    $lks = $r > lookupSeries($series, "exampleTable", "threshold")
    
    warn = $lks
    
    # spock will be contacted for host a, uhura for all others
    warnNotification = lookup("exampleTable", "contact_crew")
}

template lookup {
    body = `
        <h1>.Lookup</h1>
        
        <p>You Got a: {{ .Lookup "exampleTable" "fish" }}</p>
        <!-- For host a this will render to "Ohhh... a Red Snapper - Hmmm... Very Tasty" -->
        <!-- It is just a shorthand for {{.LookupAll "exampleTable" "fish" .Group }} -->
        
        <h2>.LookupAll</h2>
        
        <p>The fish for host "b" will always be {{ .LookupAll "exampleTable" "fish" "host=b" }}</p>
        <!-- For host a this will render to "Nothing! Absolutely Nothing! Stupid! You so Stupid!"  
        since we requested host=b specifically -->
    `
    subject = `lookup example`
}
```

## Macros

Macros are sections that can define anything (including variables). It is not an error to reference an unknown variable in a macro. Other sections can reference the macro with `macro = name`. The macro's data will be expanded with the current variable definitions and inserted at that point in the section. Multiple macros may be thus referenced at any time. Macros may reference other macros. For example:

```
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
```

Will yield a warn expression for the os.high_cpu alert:

```
avg(q("avg:rate:os.cpu{host=ny-nexpose01}", "2m", "")) > 80
```

and set `warnNotification = default` for that alert.

{% endraw %}

</div>
</div>
