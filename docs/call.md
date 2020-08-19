---
layout: default
title: Sunsetting Bosun at Stack Overflow and Call for New Bosun Maintainer(s)
---

# Sunsetting Bosun at Stack Overflow and Call for New Bosun Maintainer(s)

Stack Overflow has decided to move away from maintaining our own open source monitoring system and instead move to a commercial SaaS monitoring product. For the size of our company, maintaining an open source monitoring system properly of Bosun's scope is beyond our resources and does not align clearly with our objectives.  We want to be building more products and features for our Stack Overflow community and clients.  Building our own tools takes us away from that core mission.  To that end, Stack Overflow will be migrating away from Bosun by the end of 2019.

## What this means for the future of the Bosun Project

Bosun is a powerful monitoring system with a growing community.  We hope that someone in the community steps up and takes over as primary maintainer(s) of the project.

It is our hope that a new maintainer can be found and the project will continue.  If we can not find a new maintainer, we plan on sunsetting the project by the end of 2019.

### Support for the remainder of 2019

For the remainder of 2019 (or until a new maintainer is found), Stack commits to continuing our support:
 - Bug Fixes.
 - Surface level improvements:
   - Expression language: Additional Functions, non-breaking modifications.
   - Template Functions: New functions available in templates.
   - Minor UI improvements.

### Assistance for new maintainer

If someone/group/company wants to become the new primary maintainer (govern the project, decide what gets merged and what doesn't) then this will become less of an ending of the project and more of a handoff.

In this situation:

 - I (Kyle) would make myself available to mentor new maintainers and give feedback on ideas/features/code, code walkthroughs and things like this if able to.
 - Stack will endeavour to produce at least one major release before a transition to new maintainers trying to wrap a few major bugs.
 - Take a pass at code refactoring (non-impactful) and commenting to make it easier for someone to take over.
 - Help expand on what might need to change in the code to be able to do some of the items below once a maintainer has taken over.

# How can I/we learn more about becoming a maintainer?

You can email me at `kyle@stackoverflow.com`, or you can reach out to us in our [slack instance](https://bosun.org/slackInvite).

# Challenges/Ideas for Maintainers

There are a number of projects and/or features that have been discussed and in some cases designed but not implemented.  We hope the new maintainers consider the following projects:

<table>
    <thead>
        <th>Summary</th>
        <th>Areas</th>
        <th>Difficulty</th>
        <th>Description</th>
    </thead>
    <tr>
        <td>Refactor "State Machine"</td>
        <td>Backend</td>
        <td>Very Hard</td>
        <td>The code behind silences, when to send notifications, escalations etc (the sched package) is hard to understand and changes have often produced on expected results. Should be refactored into a set of finite state machines that are testable. If one wanted to make a solid 1.x.x release, I believe this would be the great challenge.</td>
    </tr>
    <tr>
        <td>Multi-File Support</td>
        <td>UI, Backend, Configuration Parser</td>
        <td>Hard</td>
        <td>Update the UI so rules (alert definitions, templates, notifications, etc) can be in different files. So the configuration does not become overwhelming long. Use Bosun's security middleware to limit certain files to certain Users/Groups.</td>
    </tr>
    <tr>
        <td>Clean up JS</td>
        <td>UI</td>
        <td>Medium</td>
        <td>Use some sort of packaging system for js libraries, update typescript, update d3, update angular, use more typescript objects</td>
    </tr>
    <tr>
        <td>Snippets and Expression Autocompletion</td>
        <td>UI</td>
        <td>Medium</td>
        <td>Use the abilities of the ace editor to enable snippets to pre-fill in config blocks and expression functions. Would make Bosun easier to use. Also support best practices within a company by adding custom snippets. See <a href="https://github.com/bosun-monitor/bosun/tree/snip">experimental snippet branch</a>. Could also integrate function documentation into language to make it available to the editor, see <a href="https://github.com/bosun-monitor/bosun/tree/exprDoc">experimental exprDoc branch</a>.</td>
    </tr>
    <tr>
        <td>API V2</td>
        <td>Backend, API</td>
        <td>Medium</td>
        <td>Make an APIv2 that is fully documented and has consistent response types, errors are always json etc</td>
    </tr>
    <tr>
        <td>Move incident Information to Relation Database</td>
        <td>Backend, Database</td>
        <td>Medium</td>
        <td>Instead of using redis for incident state information, use a relational database to allow for reporting. Also future enhancements to do things like collect "was this alert useful" information would be easier to implement and report on.</td>
    </tr>
    <tr>
        <td>Realtime Dashboard</td>
        <td>Backend, UI</td>
        <td>Medium</td>
        <td>Make dashboard based on websockets so dashboard can be refreshed, and other users get notifications from the actions of different users.</td>
    </tr>
    <tr>
        <td>Graphing UIs</td>
        <td>UI</td>
        <td>Medium</td>
        <td>Creating Graphing UIs for TSDBs besides OpenTSDB, like Prometheus and Graphite</td>
    </tr>
    <tr>
        <td>Use Grafana For UI</td>
        <td>UI</td>
        <td>Hard</td>
        <td>Deprecate Bosun's UI and rebuild it entirely as a Grafana App.</td>
    </tr>
</table>

