---
layout: default
title: Customizing Notifications
---
{% raw %}

## Customizing notifications

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
- **Unknown Notifications** occur as alerts go "unknown" because data is not availible. These get batched up and sent periodically.

### Alert

Alerts mostly define the *rules* that we alert on. We can control notifications with the `template`, `critNotification`, and `warnNotification` keys.

```
alert high_cpu {
    crit = 1
    template = high_cpu
    critNotification = email,slack 
}
```

defines an alert linked to the `high_cpu` template, and the `email` and `slack` notifications.

### Templates

Templates define specific templates for rendering your content. For guidance on constructing templates, see the [relevant documentation](definitions#templates). A template is essentially a set
of key/value pairs. It may define as many keys as it wishes with whatever keys it likes. There are a few special keys however:

- `body` and `subject` are the only required template keys. These are what show up on the bosun dashboard, as well as the default for most other notifications.
- `emailBody` and `emailSubject` are not required, but if present, they will be used as the body and subject for email notifications.

Any other template key will may be defined and will be used by any notifications that select it.  

#### Template inheritance

A template may `inherit` another template, which copies all of the key/value pairs into the child template. This is useful if you have some set of common formatting templates that may be shared among multiple
templates:

```
template slack { 
    slackBody = {"text": {{.Subject}} }
}
```

#### Text vs HTML templates

There are a few situations where it matters if we use *plain text* templates, or *html* templates. Html templates perform some extra sanitization for when we expect to display the content, and they also perform css-inlining to be more compatible with email clients.

### Notifications



{% endraw %}