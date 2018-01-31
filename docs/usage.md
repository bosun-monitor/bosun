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
This part of the documentation covers using Bosun's user interface and the incident workflow.

# Alerts and Incidents

## Overview
Each alert definition has the potential to turn into multiple incidents (an instantiation of the alert). Incidents get a unique global ID and are also associated with an Alert Key. The Alert Key is made up of the alert name and the tagset. Every possible group in your top level expression is evaluated independently. As an example, with an expression like `avg(q("avg:rate{counter,,1}:os.cpu{host=*}", "5m", ""))` you can have the potential to create an incident for every tag-value of the "host" tag-key that has sent data for the os.cpu metric.

## The lifetime of an incident

An incident gets created when the warn or crit expression evaluates to non-zero, or the alert goes unknown. Once an incident has been created it will notify users only when the lifetime severity of the incident increases. An exception to this is if you have set up notification chains, in which case the alert will send more notifications until someone acknowledges the alert.

Example:

 * You have an alert named high.cpu defined, and it has warn expression like `avg(q(os.cpu{host=*} ...)) > 50`. One of your hosts (web01) triggers the warn condition of the alert
 * We now have an incident, the incident will get a global ID like #23412 and will have an alert key of `high.cpu{host=web01}` and will have a current severity state of warn. Assuming a notification has been set up, the notification will be sent (i.e. an email)
 * The incident then goes back to normal severity, and then to warn again. When this happens, no notifications are sent. It is important to note that **notifications are only sent when the lifetime severity of an incident increases**. The lifetime of the incident continues until the alert has been closed - which is generally done by a user.
 * The incident can be closed when it goes back to normal state. Once the incident is closed, it is possible for a new incident to be created for the same Alert Key (`high.cpu{host=web01}`).

## Severity States

Incidents can be in one of the following severity levels (From highest to lowest):

* **Unknown**: When a warn or crit expression can not be evaluated because data is missing. When you define an alert bosun tracks each resulting tagset from the warn/crit expressions. If a tagset is no longer present, that instance goes into an unknown state. Since bosun has data pushed to it, unknown can mean that either data collection has failed, or that the source is down. Unknown triggers when there is no data for the tagset in 2x the check frequency duration. This means that if a query spans an hour, it will be one hour + 2x the check frequency before it triggers.
* **Error**: There is some sort of bosun internal error such as divide by zero or "response too large" with the alert. The error can be viewed by clicking the Errors button on the dashboard
* **Critical**: The expression that `crit` is equal to in the alert definition is non-zero (true). It is recommend that "Critical" be thought of as "has failed".
* **Warning**: The expression that `warn` is equal to in the alert definition is non-zero (true) *and* critical is not true. It is recommended that warning be thought of ha "could lead to failure".
* **Normal**: None of the above states.

## Additional States

* **Active**: The alert is currently in a non-normal state. This is indicated by an exclamation on the dashboard: <i class="fa fa-exclamation-circle fa-lg" aria-hidden="true"></i>.  Alerts don't disappear from the dashboard when they are no longer active until they are closed. This is to ensure that all alerts get handled - which reduces alert noise and fatigue.
* **Silenced**: Someone has created a silence rule that stops this alert from triggering any notification. It will also automatically close when the alert is no longer active. This is indicated by a volume off speaker icon: <i class="fa fa-volume-off fa-lg" aria-hidden="true"></i>.
* **Acknowledged**: Someone has acknowledged the alert, the reason and person should be available via the web interface. Acknowledged alerts stop sending notification chains as long as the severity doesn't increase.
* **Unacknowledged**: Nobody has acknowledged the alert yet at its current severity level.
* **Unevaluated**: An incident is unevaluated if the dependency expression as defined in the alert's depends keyword is non-zero. Unevaluated alerts do not change state or become unknown. If an incident is open then it will still show up on the dashboard, but with a question mark icon: <i class="fa fa-question-circle fa-lg" aria-hidden="true"></i>. New incidents will not be created.

# Dashboard

## Indicators

### Colors

The color of the major of the bar is the incident's last abnormal status. The color that makes up the sliver on the left side of the bar is the incident's current status.

* <span class="text-info">**Blue**:</span> Unknown
* <span class="text-danger">**Red**:</span> Critical
* <span class="text-warning">**Yellow**:</span> Warning
* <span class="text-success"> **Green**:</span> Normal

### Icons

* <i class="fa fa-exclamation-circle fa-lg" aria-hidden="true"></i> An exclamation icon means the alert is currently in an [active state](/usage#additional-states).
* <i class="fa fa-volume-off fa-lg" aria-hidden="true"></i> A silence icon means the alert has been [silenced](/usage#additional-states).
* <i class="fa fa-question-circle fa-lg" aria-hidden="true"></i> A question icon means the alert is [unevaluated](/usage#additional-states).
* <i class="fa fa-fire fa-lg" aria-hidden="true"></i> A fire icon means the alert is in an [error state](/usage#severity-states).


## Actions

* **Acknowledge**: Prevent further notifications unless there is a state increase. This also moves it to the acknowledged section of the dashboard. When you acknowledge something you enter a name and a reason. So this means that the person has committed to fixing the problem or the alert.
* **Close**: Make it disappear from the dashboard. This should be used when an alert is handled. Active (non-normal) alerts can not be closed (since all that will happen is that will reappear on the the dashboard after the next schedule run).
* **Forget**: Make bosun forget about this instance of the alert. This is used on active unknown alerts. It is useful when something is not coming back (i.e. you have decommissioned a host). This act is non-destructive because if that data gets sent to bosun again everything will come back.
* **Force Close**: Like close, but does not require alert to be in a normal state. In a few circumstances an alert can be "open" and "active" at the same time. This can occur when a host is decommissioned and an alert has ignoreUnknown set, for example. This may help to clear some of those "stuck" alerts.
* **Purge**: Will delete an active alert and *all* history for that alert key. Should only be used when you absolutely want to forget all data about a host, like when shutting it down. Like forget, but does not require an alert to be unknown.
* **History**: View a timeline of history for the selected alert instances.
* **Note**: Attach a note to an incident. This has no impact on the behavior of the alert and is purely for communication.

## Incident Filters

The open incident filter supports joining terms in `()` as well as the `AND`, `OR`, and `!` operators. The following query terms are supported and are always in the format of `something:something`:

<table>
    <tr>
        <th>Term Spec</th>
        <th>Description</th>
    </tr>
    <tr>
        <td><code>ack:(true|false)</code></td>
        <td>If <code>ack:true</code> incidents that have been acknowledge are returned, when <code>ack:false</code>                        incidents that have not been acknowledged are returned.</td>
    </tr>
    <tr>
        <td><code>ackTime:[<|>](1d)</code></td>
        <td>Returns incidents that were acknowledged before <code><</code> or incidents that were acknowledged after <code>></code> the
            relative time to now based on the duration. Duration can be in units of s (seconds), m (minutes),
            h (hours), d (days), w (weeks), n (months), y (years). If less than or greater than are not part
            of the value, it defaults to greater than (after). Now is clock time and is not related to the time
            range specified in Grafana. For example, <code>ackTime:<24h</code> shows incidents that were acknowledged more than 24 hours ago.</td>
    </tr>
    <tr>
        <td><code>hasTag:(tagKey|tagKey=|=tagValue|tagKey=tagValue)</code></td>
        <td>Determine if the tag key, value, or key=value pair. If there is no equals sign, it is treated as a tag
            key. Tag Values maybe have globs such has <code>hasTag:host=ny-*</code></td>
    </tr>
    <tr>
        <td><code>hidden:(true|false)</code></td>
        <td>If <code>hidden:false</code> incidents that are hidden will not be show. An incident is hidden if it
            is in a silenced or unevaluated state. </td>
    </tr>
    <tr>
        <td><code>name:(something*)</code></td>
        <td>Returns incidents where the alert name (not including the tagset) matches the value. Globs can be used
            in the value.</td>
    </tr>
    <tr>
        <td><code>user:(username*)</code></td>
        <td>Returns incidents where a user has taken any action on that incident. Globs can be used in the value</td>
    </tr>
    <tr>
        <td><code>notify:(notificationName*)</code></td>
        <td>Returns incidents where a the notificationName is somewhere in either the crit or warn notification chains.
            Globs can be used in the value</td>
    </tr>
    <tr>
        <td><code>silenced:(true|false)</code></td>
        <td>If <code>silenced:false</code> incidents that have not been silenced are returned, when <code>silenced:true</code>                        incidents that have not been silenced are returned.</td>
    </tr>
    <tr>
        <td><code>start:[<|>](1d)</code> </td>
        <td>Returns incidents that started before <code><</code> or incidents that started after <code>></code> the
            relative time to now based on the duration. Duration can be in units of s (seconds), m (minutes),
            h (hours), d (days), w (weeks), n (months), y (years). If less than or greater than are not part
            of the value, it defaults to greater than (after). Now is clock time and is not related to the time
            range specified in Grafana.</td>
    </tr>
    <tr>
        <td><code>unevaluated:(true|false)</code></td>
        <td>If <code>unevaluated:false</code> incidents that are not in an unevaluated state are returned, when
            <code>ack:true</code> incidents that are unevaluated are returned.</td>
    </tr>
    <tr>
        <td><code>status:(normal|warning|critical|unknown)</code></td>
        <td>Returns incidents that are currently in the requested state</td>
    </tr>
    <tr>
        <td><code>worstStatus:(normal|warning|critical|unknown)</code></td>
        <td>Returns incidents that have a worst status equal to the requested state</td>
    </tr>
    <tr>
        <td><code>lastAbnormalStatus:(warning|critical|unknown)</code></td>
        <td>Returns incidents that have a last abnormal status equal to the requested state</td>
    </tr>
    <tr>
        <td><code>subject:(something*)</code></td>
        <td>Returns incidents where the subject string matches the value. Globs can be used in the value</td>
    </tr>
    <tr>
        <td><code>since:[<|>](1d)</code> </td>
        <td>Returns incidents that in `status` more than <code><</code> or incidents that in `status` less than <code>></code> the
            relative time to now based on the duration. Duration can be in units of s (seconds), m (minutes),
            h (hours), d (days), w (weeks), n (months), y (years). If less than or greater than are not part
            of the value, it defaults to greater than (after). Now is clock time and is not related to the time
            range specified in Grafana.<br>
            e.g. `status:normal AND since:<15d` return alerts that are in `normal` more than 15 day's
        </td>
    </tr>
</table>

# Rule Editor
The rule editor allows you to edit the the definitions in the [RuleConf](/definitions), preview rendered templates, and test alerts against historical data.

## Rule Editor Image
![Rule Editor Image](/public/rule_editor.jpg)

## Textarea
The text area will be loaded with the running config when the Rule Editor view is loaded. A hash of the config when you start editing it is saved. If someone else edits the UI and saves it, Bosun will detect that the config hash has changed and show a warning above the text area.

When you run test your version of the config is saved in Bosun, and you can link to it so others can see it.

The editor is built using the open source [Ace editor](https://ace.c9.io/).

## Jump Buttons
The Jump drop downs <a href="/usage#rule-editor-image" class="image-number">①</a> will take you to defined sections within the config. In particular, the alert drop down selects which alert will be used for testing.

At the end there is a switcher that can be used when you are working on an alert. It allows you to just back and forth between the alert and the alert referenced in the template.

## Download / Validate
The download button <a href="/usage#rule-editor-image" class="image-number">②</a> will download the config file as a text file. Validate makes sure that Bosun considers the config valid using the same validation that is required for Bosun to start.

## Definition [Rule] Saving
The save button <a href="/usage#rule-editor-image" class="image-number">②</a> will bring up a dialogue that lets you save the config. This only appears if you have permission to save the config, and the [system configuration's `EnableSave`](/system_configuration#enablesave) has been set to true.

The save dialogue will show you a contextual diff of your config and the running config. There are several protections in place to prevent you from overwriting someone elses configuration changes:

  * The Rule Editor will show a warning if the config has been saved since you started editing it
  * A contextual-diff is shown of your changes versus the running config (and the save we fail if the contextual diff happens to change in the time window before you hit save)
  * When the file is being saved, a global lock is taken in Bosun so nobody else can save while the save his happening

If the config file is successfully saved then Bosun will reload the new definitions. Alerts that are currently being processed will be cancelled and restarted. In other words, a restart of the Bosun process is *not* required for the new changes to take effect.

An external command to run on saves can also be defined with the [CommandHookPath setting in the system configuration](/system_configuration#commandhookpath). This can be used to do things like create backups of the file or check the changes into version control. If this command returns a non-zero exit code, saving will also fail.

In all cases where a save fails, a reload will not happen and the save will not be persisted (the definitions file will not be changed).

## Alert Testing
Alerts can be tested before they are committed to production. This allows you to refine the trigger conditions to control the signal to noise and to preview the rendered templates to make sure alerts are informative. This done by selecting the alert the from the [Jump Alert Drop down](/usage#jump-buttons) at <a href="/usage#rule-editor-image" class="image-number">①</a> and the clicking the test alert button at <a href="/usage#rule-editor-image" class="image-number">④</a>.

There are two ways you can test alerts: 
 
  1. A single iteration (a snapshot of time)
  2. Multiple iterations over a period of time. 
  
Which behavior is used depends on the <span class="docFromLabel">From</span> and <label>To</label> fields at <a href="/usage#rule-editor-image" class="image-number">③</a>. If <span class="docFromLabel">From</span> is left blank, that a single iteration is tested with the time current time. If <span class="docFromLabel">From</span> is set to a time and <span class="docFromLabel">To</span> is unset, a single iteration will be done at that time. When doing single iteration testing the <span class="docFromLabel">Results</span> and <span class="docFromLabel">Template</span> <a href="/usage#rule-editor-image" class="image-number">⑤</a> tabs at will be populated. The <span class="docFromLabel">Results</span> tabs show the warn/crit results for each set, and a rendered template will be show in the  <span class="docFromLabel">Template</span> tab.

Which item from the result set that will be rendered in the Template tab is controlled by the <span class="docFromLabel">Template Group</span> field at <a href="/usage#rule-editor-image" class="image-number">④</a>. Which result to use for the template is picked by specifying a tagset in the format of `key=value,key=value`. The first result that has the specified tags will be used. If no results match, than the first result is chosen.

<div class="admonition">
<p class="admonition-title">Tip</p>
<p>When working on a template it is good to set the <span class="docFromLabel">From</span> time to a fixed date. That way when expressions are rerun they will likely hit Bosun's query cache and things will be faster.</p>
</div>

The <span class="docFromLabel">Email</span> field at <a href="/usage#rule-editor-image" class="image-number">④</a> makes it so when an alert is tested, the rendered template is emailed to the address specified in the field. This is so you can check for any differences between what you see in the <span class="docFromLabel">Template</span> tab.

Setting both <span class="docFromLabel">From</span> and <span class="docFromLabel">To</span> enables testing multiple iterations of the selected alert over time. The number of iterations depends on the setting to the two linked fields <span class="docFromLabel">Intervals</span> and <span class="docFromLabel">Step Duration</span> at <a href="/usage#rule-editor-image" class="image-number">③</a>. Changing one changes the other. Intervals will be the number of runs to do even spaced out over the duration of <span class="docFromLabel">From</span> to <span class="docFromLabel">To</span> and <span class="docFromLabel">Step Duration</span> is how much time in minutes should be between intervals. Doing a test over time will populate the <span class="docFromLabel">Timeline</span> tab <a href="/usage#rule-editor-image" class="image-number">⑤</a> which draws a clickable graphic of severity states for each item in the set:

![Rule Editor Timeline Image](/public/timeline.jpg)

Each row in the image is one of the items in the result set. The color squares represent the severity of that instance. The X-Axis is time. When you click the a square on the image, it will take you to the event you clicked and show you what the template would look like at that time for that particular item.

# Annotations

Annotations are currently stored in elastic. When annotations are enabled you can create, edit and visualize them on the the Graph page. There is also a Submit Annotations page that allows for creation and editing annotations. The API described in this [README](https://github.com/bosun-monitor/annotate/blob/master/web/README.md) gets injected into bosun under `/api/` - you can also find a description of the schema there. 

</div>
</div>