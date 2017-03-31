---
layout: default
title: Administration
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
This part of the documentation covers various aspects of Bosun administration. 

# Authentication
Bosun currently supports two types of authentication when authentication is enabled:

 1. LDAP authentication
 2. Token Based access

The two intended uses of these methods is user authentication and api authentication respectively. Authorization is a new feature in Bosun 0.6.0. Even when authorization is enabled, Bosun should still be run inside a trusted network.

## Setup
The authentication feature gets enabled when you define the [AuthConf section of the system configuration](/system_configuration#authconf). Authentication tokens can be set up via the UI by setting [AuthDisabled](/system_configuration#authdisabled) before authentication is enabled. `AuthDisabled` makes it so the authentication *feature* is enabled but authentication itself is not enabled. With `AuthDisabled` set to true anonymous users can create auth tokens via Bosun's user interface.

## Auth Token UI
When the authentication feature is enabled, you should see a <span class="docFromLabel">Manage Auth Tokens</span> menu item under your username in Bosun's UI in the upper right corner. You will be able to see this if `AuthDisabled` is true or if you have the `Manage Tokens` Permission set for your user.

From there you can create new auth tokens in two steps as show in the following images. Note that once you retrieve a token from the second screen, you will *not* be able to view the token itself again. You will still be able to see the name, description, permissions set, and the last time it was used.

First Screen:

![Create Token Image](/public/createToken.jpg)

Second Screen:

![Token Created Image](/public/createdToken.jpg)

## Permissions and Roles
Permissions provide the ability to certain things with both, and Roles are a collection of permissions for convenience. A user could have no role and an arbitrary collection of permissions.

<table>
    <tr>
        <th>Permission</th>
        <th>Roles</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>View Dashboard</td>
        <td>Admin, Writer, Reader</td>
        <td>Can view dashboard and alert state data, metrics, and graphs</td>
    </tr>
    <tr>
        <td>View Config</td>
        <td>Admin, Writer, Reader</td>
        <td>Can view bosun configuration page</td>
    </tr>
    <tr>
        <td>View Annotations</td>
        <td>Admin, Writer, Reader</td>
        <td>Can view annotations on graph page</td>
    </tr>
    <tr>
        <td>Put Data</td>
        <td>Admin, Writer</td>
        <td>Can put and index OpenTSDB data and metadata</td>
    </tr>
    <tr>
        <td>Actions</td>
        <td>Admin, Writer</td>
        <td>Can acknowledge and close alerts</td>
    </tr>
    <tr>
        <td>Run Tests</td>
        <td>Admin, Writer</td>
        <td>Can execute expressions, graphs, and rule tests</td>
    </tr>
    <tr>
        <td>Save Config</td>
        <td>Admin, Writer</td>
        <td>Can alter and save bosun rule config</td>
    </tr>
    <tr>
        <td>Silence</td>
        <td>Admin, Writer</td>
        <td>Can add and manage silences</td>
    </tr>
    <tr>
        <td>Manage Tokens</td>
        <td>Admin</td>
        <td>Can manage authorization tokens</td>
    </tr>
    <tr>
        <td>Set Username</td>
        <td>Admin</td>
        <td>Allows external services to set a different username in api requests</td>
    </tr>
</table>

## Syncing Tokens

</div>
</div>
