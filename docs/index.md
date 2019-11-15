---
layout: page
title: Home
order: 1
---

<script>
	 // Entypo pictograms by Daniel Bruce — www.entypo.com
</script>

<div class="row jumbotron">
	<div class="col-md-12">
		<div>
			<h1 class="hometitle">Bosun</h1>
			<p class="lead">is an open-source, MIT licensed, monitoring and alerting system by <a href="http://stackexchange.com">Stack Exchange</a>. It has an expressive domain specific language for evaluating alerts and creating detailed notifications. It also lets you test your alerts against history for a faster development experience.</p>
		</div>
	</div>
</div>

<div class="row">
	<div class="col-md-12">
		<div class="admonition tip">
				<p class="admonition-title">Project Status (2019-11)</p>
				<p><a href="https://skyscanner.net">Skyscanner</a> is delighted to announce that as of 18th November 2019, we'll be officially taking over Bosun's ownership and maintainership from <a href="http://stackexchange.com">Stack Exchange</a> (home of stackoverflow.com).</p>
                <p>We would like to thank StackExchange for their time, ensuring a smooth transition process, and for everyone’s hard work and input towards making this project a success. A special thanks to Kyle Brandit for initiating the Bosun project and leading it for many years.
                 </p>
                 <p> We will continue to welcome the open-source community code contributions on this project. Look out for updates on our plans and next steps in the near future.</p>
		</div>
	</div>
</div>
<div class="row" style="padding-top: 75px;">
	<div class="col-md-4">
		<h3>Try It</h3>
		<p>Want to jump right in? Follow our <strong><a href="/quickstart">quick start tutorial</a></strong> to get Bosun, OpenTSDB, and scollector up quickly using our published docker images.</p>
	</div>
	<div class="col-md-4">
		<h3>Download It</h3>
		<p>Download the prebuilt binaries for <strong><a href="/downloads">Bosun</a></strong> and our optional but valuable monitoring agent (Currently works only with OpenTSDB) <strong><a href="/scollector">scollector</a></strong> for Windows, Linux, and Mac.</p>
	</div>
	<div class="col-md-4">
		<h3>Get Help</h3>
		<p>Join us in our Slack room. <a href="/slackInvite">Get an invite.</a> You can <strong><a href="https://github.com/bosun-monitor/bosun/issues">open issues on GitHub</a></strong> to report bugs or discuss new features.</p>
	</div>
</div>
<div class="row" style="padding-top: 75px"><h2 class="text-center feature-header">Features</h2></div>
<hr class="feature">
<div class="row">
	<div class="col-md-6 left">
		<div class="col-md-2 left ">
			<img class="fpicon" src="public/sound-mute.svg" style="max-height: 40px; max-width: 40px;">
		</div>
		<div class="col-md-10">
			<p class="smaller">Use Bosun's flexible expression language to evaluate time series in an exacting way</p>
		</div>
	</div>
	<div class="col-md-6 right">
		<div class="col-md-2 left"><img class="fpicon" src="public/inbox.svg" style="max-height: 40px; max-width: 40px;"></div>
		<div class="col-md-10">
		<p class="smaller">Create notifications using Bosun's template language: include graphs, tables, and contextual information</p>
		</div>
	</div>
</div>

<div class="row">
	<div class="col-md-6 left">
		<div class="col-md-2 left"><img class="fpicon" src="public/hour-glass.svg" style="max-height: 40px; max-width: 40px;"></div>
		<div class="col-md-10">
			<p class="smaller">Save time by testing alerting against historical data and reduce alert noise before an alert goes into production</p>
		</div>
	</div>
	<div class="col-md-6 right">
		<div class="col-md-2 left"><img class="fpicon" src="public/database.svg" style="max-height: 40px; max-width: 40px;"></div>
		<div class="col-md-10">
			<p class="smaller">Supports querying <a href="http://opentsdb.net/">OpenTSDB</a>, <a href="http://graphite.readthedocs.org">Graphite</a>, and <a href="http://www.elasticsearch.org/overview/logstash/">Logstash-Elasticsearch</a></p>
		</div>
	</div>
</div>

<div class="row" style="padding-top: 10px;">
	<div class="col-md-6 left">
		<div class="col-md-2 left"><img class="fpicon" src="public/windows-store.svg" style="max-height: 40px; max-width: 40px;"><img class="fpicon" src="public/tux-large-bw.png" style="max-height: 40px; max-width: 40px; margin-left: -20px;"></div>
		<div class="col-md-10">
			<p class="smaller">Runs on Linux, Windows, Mac, or any operating system supported by <a href="https://golang.org/"></a>Go</p>
		</div>
	</div>
	<div class="col-md-6 right">
		<div class="col-md-2 left"><img class="fpicon" src="public/agent.png" style="max-height: 40px; max-width: 40px;"></div>
		<div class="col-md-10">
			<p class="smaller">Use our agent scollector to get monitoring of Windows, Linux, and many common applications</p>
		</div>
	</div>
</div>

<div class="row">
	<div class="col-md-6 left">
		<div class="col-md-2 left"><img class="fpicon" src="public/grid.svg" style="max-height: 40px; max-width: 40px;"></div>
		<div class="col-md-10">
			<p class="smaller">Supports arbitrary dimensions (not just host based), aggregations, and automatically incorporating new tags (hosts, services, ..) as they appear</p>
		</div>
	</div>
	<div class="col-md-6 right">
		<div class="col-md-2 left"><img class="fpicon" src="public/cog.svg" style="max-height: 40px; max-width: 40px;"></div>
		<div class="col-md-10">
			<p class="smaller">Scollector auto detects new services and starts sending metrics immediately; properly designed alerts will apply to these new services which reduces maintenance</p>
		</div>
	</div>
</div>
