---
layout: page
title: Download Bosun
---

{% assign release = site.github.releases[0] %}
{% assign relname = release.tag_name %}

<div class="row">
	<div class="col-md-12">
		<h2>Latest release: <a href="{{release.html_url}}">{{relname}}</a> Published {{release.created_at | date_to_string}}
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
		<h2>From Source</h2>
		<code>$ go get bosun.org/cmd/bosun</code>
	</div>
</div>
