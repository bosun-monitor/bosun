---
layout: default
title: Usage
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
This part of the documentation covers using bosun after you have defined you configuration and your alerts.

# Alerts and Incidents

## Overview
Each alert definition has the potential to turn into multiple incidents (an instantation of the alert). Incidents get a unique global ID and are also associated with an Alery Key. The Alert Key is made up of the alert name and the tagset. Every possible group in your top level expression is evaluated independently. As an example, with an expression like `avg(q("avg:rate{counter,,1}:os.cpu{host=*}", "5m", ""))` you can have the potential to create an incident for every tag-value of the "host" tag-key that has sent data for the os.cpu metric.

### The lifetime of an incident

An incident gets created when the warn or crit expression evaulates to non-zero, or the alert goes unknown. Once an incident has been created it will notify users only when the lifetime severity of the incident increases. An exception to this is if you have set up notification chains, in which case the alert will send more notifications until someone acknowledges the alert.

#### Example
 * You have an alert named high.cpu defined, and it has warn expression like `avg(q(os.cpu{host=*} ...)) > 50`. One of your hosts (web01) triggers the warn condition of the alert
 * We now have an incident, the incident will get a global ID like #23412 and will have an alert key of `high.cpu{host=web01}` and will have a current severity state of warn. Assuming a notification has been set up, the notification will be sent (i.e. an email)
 * The incident then goes back to normal severity, and then to warn again. When this happens, no notifications are sent. It is important to note that **notifications are only sent when the lifetime severity of an incident increases**. The lifetime of the incident continues until the alert has been closed - which is generally done by a user.
 * The incident can be closed when it goes back to normal state. Once the incident is closed, it is possible for a new incident to be created for the same Alert Key (`high.cpu{host=web01}`).

#### Severity States

Incidents can be in one of the following severity levels (From highest to lowest):

* **Unknown**: When a warn or crit expression can not be evaluated because data is missing. When you define an alert bosun tracks each instance (aka group) for each expression used in the expression. If one of these is no longer present, that instance goes into an unknown state. Since bosun has data pushed to it, unknown can mean that either data collection has failed, or that the source is down. Unknown triggers when there is no data in a query + the check frequency. This means that if a query spans an hour, it will be one hour + the check frequency before it triggers.
* **Error**: There is some sort of bosun internal error such as divide by zero or "response too large" with the alert.
* **Critical**: The expression that `crit` is equal to in the alert definition is non-zero (true). It is recommend that "Critical" be thought of as "has failed".
* **Warning**: The expression that `warn` is equal to in the alert definition is non-zero (true) *and* critical is not true. It is recommended that warning be thought of ha "could lead to failure".
* **Normal**: None of the above states.

#### Additional States

* **Active**: The alert is currently in a non-normal state. This is indicated by an exclamation on the dashboard: ![Exclamation Glyph](public/exclamation.png).
* **Silenced**: Someone has created a silence rule that stops this alert from triggering any notification. It will also automatically close when the alert is no longer active. This is indicated by a speaker with an X icon on the dashboard: ![Silence Glyph](public/silence.png).
* **Acknowledged**: Someone has acknowledged the alert, the reason and person should be available via the web interface. Acknowledged alerts stop sending notification chains as long as the severity doesn't increase.
* **Unacknowledged**: Nobody has acknowledged the alert yet at its current severity level.

# Dashboard

## Indicators

### Colors

The color of the major of the bar is the incident's last abnormal status. The color that makes up the sliver on the left side of the bar is the incident's current status.

* **Blue**: Unknown
* **Red**: Critical
* **Yellow**: Warning
* **Green**: Normal

### Icons

* ![Exclamation Glyph](public/exclamation.png) An Exclamation means the alert is currently triggered (active). Alerts don't disappear from the dashboard when they are no longer active until they are closed. This is to ensure that all alerts get handled - which reduces alert noise and fatigue.
* ![Silence Glyph](public/silence.png) A silence icon means the alert has been silenced. Silenced alerts don't send notifications, and automatically close when no longer active.

## Actions

* **Acknowledge**: Prevent further notifications unless there is a state increase. This also moves it to the acknowledged section of the dashboard. When you acknowledge something you enter a name and a reason. So this means that the person has committed to fixing the problem or the alert.
* **Close**: Make it disappear from the dashboard. This should be used when an alert is handled. Active (non-normal) alerts can not be closed (since all that will happen is that will reappear on the the dashboard after the next schedule run).
* **Forget**: Make bosun forget about this instance of the alert. This is used on active unknown alerts. It is useful when something is not coming back (i.e. you have decommissioned a host). This act is non-destructive because if that data gets sent to bosun again everything will come back.
* **Force Close**: Like close, but does not require alert to be in a normal state. In a few circumstances an alert can be "open" and "active" at the same time. This can occur when a host is decomissioned and an alert has ignoreUnknown set, for example. This may help to clear some of those "stuck" alerts.
* **Purge**: Will delete an active alert and ALL history for that alert key. Should only be used when you absolutely want to forget all data about a host, like when shutting it down. Like forget, but does not require an alert to be unknown.

* **History**: View a timeline of history for the selected alert instances.

# Annotations

Annotations are currently stored in elastic. When annotations are enabled you can create, edit and visulize them on the the Graph page. There is also a Submit Annotations page that allows for creation and editing annotations. The API described in this [README](https://github.com/bosun-monitor/annotate/blob/master/web/README.md) gets injected into bosun under `/api/` - you can also find a description of the schema there. 

</div>
</div>