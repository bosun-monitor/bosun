---
layout: page
title: Download Bosun
---

{% if site.github != null %}

	{% assign releases = site.github.releases | where:"draft",false %}
	{% assign release = releases[0] %}
	{% assign relname = release.tag_name %}
	<h2>Latest release: <a href="{{release.html_url}}">{{relname}}</a> Published {{release.created_at | date_to_string}}
	<p>{{ release.body | markdownify  }}</p>
	<h2 id="binaries">Binaries</h2>

	<p>Binaries are provided below. All web assets are already bundled. Source instructions provided for developers.</p>
	<ul>
		<li><a href="https://github.com/bosun-monitor/bosun/releases/download/{{relname}}/bosun-linux-amd64"><strong>Linux</strong> amd64</a></li>
		<li><a href="https://github.com/bosun-monitor/bosun/releases/download/{{relname}}/bosun-linux-386"><strong>Linux</strong> 386</a></li>
		
		<li><a href="https://github.com/bosun-monitor/bosun/releases/download/{{relname}}/bosun-windows-amd64.exe"><strong>Windows</strong> amd64</a></li>
		<li><a href="https://github.com/bosun-monitor/bosun/releases/download/{{relname}}/bosun-windows-386.exe"><strong>Windows</strong> 386</a></li>
		
		<li><a href="https://github.com/bosun-monitor/bosun/releases/download/{{relname}}/bosun-darwin-amd64"><strong>Mac</strong> amd64</a></li>
		<li><a href="https://github.com/bosun-monitor/bosun/releases/download/{{relname}}/bosun-darwin-386"><strong>Mac</strong> 386</a></li>
	</ul>
	
	<p>View upcoming features on <a href="https://github.com/bosun-monitor/bosun/pulls?utf8=✓&q=merged%3A>%3D{{release.created_at | date_to_xmlschema}}">Github.</a>
{% else %}
	<h3>View latest release on <a href="https://github.com/bosun-monitor/bosun/releases/latest">Github</a></h3>
{% endif %}
<h2>From Source</h2>
<code>$ go get bosun.org/cmd/bosun</code>
