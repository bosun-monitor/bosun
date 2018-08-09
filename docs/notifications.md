---
layout: default
title: Customizing Notifications
---
{% raw %}

# Customizing notifications

Bosun supports full customization over notification contents, and provides a templating engine to generate whatever
content you need from an alert. Multiple components affect how your notifications look:

1. **Alerts** reference exactly one *template* and any number of *notifications*.
2. **Templates** define *what content* to generate for an alert.
3. **Notifications** define *which* content to send, and *where* to send it.

We support sending notifications via:

- Email
- HTTP Request
- Printing to console

There are also various different types of notification events, each having different kinds of underlying data and semantics:

- **Alert Notifications** are sent whenever an alert triggers (or gets worse). The content is rendered at the time the alert triggers, and stored for future re-use.
- **Action Notifications** are sent when a user performs an action in bosun (Ack, Close, etc..). For these, the content is rendered on-demand as actions occur.
- **Unknown Notifications** occur as alerts go "unknown" because data is not available. These get batched up and sent periodically.

# Alert

Alerts mostly define the *rules* that we alert on. We can control notifications with the `template`, `critNotification`, and `warnNotification` keys.

```
alert high_cpu {
    crit = 1
    template = high_cpu
    critNotification = email,slack 
}
```

defines an alert linked to the `high_cpu` template, and the `email` and `slack` notifications.

# Templates

Templates define specific templates for rendering your content. For guidance on constructing templates, see the [relevant documentation](definitions#templates). A template is essentially a set
of key/value pairs. It may define as many keys as it wishes with whatever keys it likes. There are a few special keys however:

- `body` and `subject` are the only required template keys. These are what show up on the bosun dashboard, as well as the default for most other notifications.
- `emailBody` and `emailSubject` are not required, but if present, they will be used as the body and subject for email notifications.

Any other template key will may be defined and will be used by any notifications that select it.  

## Template inheritance

A template may `inherit` another template, which copies all of the key/value pairs into the child template. This is useful if you have some set of common formatting templates that may be shared among multiple templates. An example:

~~~
# base template for all slack notifications. Creates json message using alert subject.
template slack {
  slackBody = `{
  "text": "{{.Subject}} <{{.Incident}}|view in bosun>",
  "username": "bosun",
  "icon_url": "https://i.imgur.com/ogj0wkj.png",
}`
}

template high_cpu {
    body = {{.Subject}}
    subject = `High CPU on {{.Group.host}}`
    # inherit slack template
    inherit = slack
}

notification slack {
  post = ${sys.SLACK_URL}
  #select slack body template
  bodyTemplate = slackBody
}

alert high_cpu {
  crit = avg(series("host=server01", epoch(), 1))
  template = high_cpu
  critNotification = slack
}
~~~

## Text vs HTML templates

There are a few situations where it matters if we use *plain text* templates, or *html* templates. Html templates perform some extra sanitization for when we expect to display the content, and they also perform css-inlining to be more compatible with email clients. The rules are simple:

1. `body` and `emailBody` are always rendered as html templates.
1. Any custom template key ending with `HTML` (like `myCustomHTML`) will be rendered as html.
1. Anything else (including `subject`) will be rendered as plain-text.

# Notifications

A notification's job is to choose what content gets sent, and where to send it. It is common to make a unique notification for each unique email address or list that bosun sends to, and for each url/api it calls. For alerts, the notification choses which of the pre-rendered templates to send. For actions and unknowns, it will pick the right template, and render it on the fly. Rules for template selection are as follows:

## Email Alerts

For alert emails, the subject and body are chosen according to the following priorities:

### Subject

1. If the notification sets `emailSubjectTemplate`, use that key from the template.
1. If the associated template has an `emailSubject` key defined, use that.
1. Otherwise, the `subject` template will be used. It will be rendered with the `.IsEmail` flag set, if you would like to customize part of it for email. 

### Body

1. If the notification sets the `bodyTemplate`,  use that key.
1. If the associated template has an `emailBody` key defined, use that.
1. Otherwise use the `body` template, rendering with `.IsEmail` set to `true`.

## HTTP Alerts

Bosun can send http notifications using the following precedence rules:

### URL

1. If the notification sets `postTemplate` or `getTemplate`, those rendered templates will be used as the notification url.
1. Otherwise the plain `post` or `get` values are used.

### Post Body

1. If the notification sets `bodyTemplate`, use that rendered template as the post body.
1. Otherwise use the rendered `subject` template.

## Action Notifications

Action notifications are a little different than alert notifications. They are rendered as actions happen, and they use a different context than the alert templates, and has the following data available:

- `{{.States}}` is a list of all incidents affected.
- `{{.User}}` is the user who performed the action.
- `{{.Message}}` is the message they entered.
- `{{.ActionType}}` is the type of action.

If multiple actions are performed at once, they are grouped together by default. You can disable this, and send a notification for each individual alert key by setting `geoupActions = false` in the notification. You can get the first incident from `States` with `{{$first := index .States 0}}` if this is the case.

You can choose whether a notification sends action notifications or not on a per-action basis using the `runOnActions` key. You may set it to `all` or `none`, or to any comma separated list of action types from `Ack`, `Close`, `Forget`, `ForceClose`, `Purge`, `Note`, `DelayedClose`, or `CancelClose`.

If you do not override anything in the notification, bosun will use its' own built in action template for action notifications. You can otherwise specify a template to use for all actions, or to override only spcific actions. You may customize a number of fields individually as well. The general form for these keys is:

`action{TemplateType}{ActionType?}`

Where "templateType" is one of `Body`, `Get`, `Post`, or `EmailSubject`, and "ActionType" if present, is one of `Ack`, `Close`, `Forget`, `ForceClose`, `Purge`, `Note`, `DelayedClose`, or `CancelClose`. If Action Type is not specified, it will apply to all actions types, unless specifically overridden.

For example, setting `actionBody = keyX`, will use the `keyX` template for all action notification bodies for all action types, but `actionBodyAck = keyY`, will use the `keyY` template only for acknowledge actions.

Example Slack Action notification:

```
template slack {
  #format action like "steve_brown Acknowledged incident 45 (High CPU on server01): "I was running a load test"
  slackActionBody = `{{$first := index .States 0}}{
    "text": "{{.User}} {{.ActionType}} <{{.IncidentLink $first.Id}}|incident {{$first.Id}}> ({{$first.Subject}}): \"{{.Message}}\"",
    "username": "bosun",
    "icon_url": "https://i.imgur.com/ogj0wkj.png",
  }`
}

notification slack {
  post = ${sys.SLACK_URL}
  bodyTemplate = slackBody
  runOnActions = Ack
  groupActions = false
  actionBody = slackActionBody
}
```

## Unknown Notifications

When an alert goes "unknown", it will send a special notification to let you know. Similar to actions, these notifications are rendered on-demand, with a special context. Bosun attempts to group these appropriately to reduce spam. The context has:

- `{{.Time}}`, a timestamp of when the unknown event occurred.
- `{{.Name}}`, bosun's description of the tags or alert name for the grouping.
- `{{.Group}}`, list of alert keys that are unknown.

If you want to control the way bosun groups unknowns, you can set the `unknownMinGroupSize` value in a notification to override bosun's global default value. This controls the number of alert keys that need to share a common tag in order for us to consider them "similar". You can set this to 0 to never group unknowns together into groups, and send a notification for each alert key.

Bosun also has a threshold for how many unknown notifications it will send at all in a single batch of notifications. This can be overridden in a notification with the `unknownThreshold` value. Setting to 0 will again remove the limit. Once the limit is reached, the final batch will switch over to using the "multiple unknown groups" templates, which use a similar context, except `{{.Groups}}` is now a lookup of group names to a list of alert keys.

You can define which template keys a notification will use for unknown notifications by setting

`unknownBody`, `unknownPost`, `unknownGet`, and `unknownEmailSubject`

or

`unknownMultiBody`, `unknownMultiPost`, `unknownMultiGet`, and `unknownMultiEmailSubject`.

If a template is not set as needed by the notification type, a built-in default template will be used.

{% endraw %}