---
layout: page
title: Download Bosun
---
<script>
	$(function(){
		 $.getJSON("https://api.github.com/repos/bosun-monitor/bosun/releases/latest").done(function (release) {
            var downloadURLBase = "https://github.com/bosun-monitor/bosun/releases/download/" + release.tag_name + "/";
			 $('.releaseLink').each(function() {
				var id = this.id
				$(this).attr("href",downloadURLBase+id)
			});
			$("#releaseLink").attr("href", release.html_url);
			$("#releaseTag").text(release.tag_name)
			$("#releaseDate").text(release.created_at);
			$("#releaseAuthor").text(release.author.login)
			$("#releaseInfo").show();
        });
	})

</script>
<div class="row" id="releaseInfo" style='display:none;'>
	<div class="col-md-12"  >
		<h3>Latest release: <a id='releaseLink'><span id='releaseTag'></span></a></h3>
		<h4>Published <span id='releaseDate'></span> by <span id='releaseAuthor'></span> </h4>
	</div>
</div>
<div class="row">
	<div class="col-md-12">
		<h2 id="binaries">Binaries</h2>
		<p>Binaries are provided below. All web assets are already bundled. Source instructions provided for developers.</p>
		<ul>
			<li><a class='releaseLink' href="https://github.com/bosun-monitor/bosun/releases/latest" id="bosun-linux-amd64"><strong>Linux</strong> amd64</a></li>
			<li><a class='releaseLink' href="https://github.com/bosun-monitor/bosun/releases/latest" id="bosun-linux-386"><strong>Linux</strong> 386</a></li>
			<li><a class='releaseLink' href="https://github.com/bosun-monitor/bosun/releases/latest" id="bosun-windows-amd64.exe"><strong>Windows</strong> amd64</a></li>
			<li><a class='releaseLink' href="https://github.com/bosun-monitor/bosun/releases/latest" id="bosun-windows-386.exe"><strong>Windows</strong> 386</a></li>
			<li><a class='releaseLink' href="https://github.com/bosun-monitor/bosun/releases/latest" id="bosun-darwin-amd64"><strong>Mac</strong> amd64</a></li>
			<li><a class='releaseLink' href="https://github.com/bosun-monitor/bosun/releases/latest" id="bosun-darwin-386"><strong>Mac</strong> 386</a></li>
		</ul>
		<h2>From Source</h2>
		<code>$ go get bosun.org/cmd/bosun</code>
	</div>
</div>