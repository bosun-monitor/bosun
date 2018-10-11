---
layout: default
title: Resources
---
  &nbsp;

## Getting Started

### [Installing Bosun for Production](https://medvedev.io/blog/posts/2015-06-21-bosun-install-1.html "Installing Bosun for production") by Dima Medvedev
> A detailed walk-through of installing Bosun and OpenTSDB using Apache Ambari to setup HBase/Hadoop. See also [these snippets](https://gist.github.com/gbrayut/3af4bcd0458d2aa57dd2) for using Cloudera CDH 5.4.4 on RHEL 6.

### [Stack Overflow's Bosun Architecture](http://kbrandt.com/post/bosun_arch/)
> Reference setup of Bosun in a production environment.

### [Bosun Fundamentals](https://www.youtube.com/playlist?list=PLWetmRzVkFTdnjRmE-a-JRx2m8qgB6iu9) video playlist by Kyle Brandt
> An introduction to the data model of Bosun’s primary time series backend OpenTSDB. The [first video](https://www.youtube.com/watch?v=DVavDN8tyIc&list=PLWetmRzVkFTdnjRmE-a-JRx2m8qgB6iu9&index=1) covers time series, metrics and tags, aggregation, rate calculation, and downsampling. The [second video](https://www.youtube.com/watch?v=7NYHOp3HlQc&list=PLWetmRzVkFTdnjRmE-a-JRx2m8qgB6iu9&index=2) covers data types, functions, operators, and set joins.

### Docker images
> There is a [full Bosun](https://hub.docker.com/r/stackexchange/bosun/) image that can be used for testing (Not recommended for production use), and an [OpenTSDB only](https://hub.docker.com/r/petergrace/opentsdb-docker/) image if desired.

### Chef Cookbooks by ptqa
> There is a cookbook for using Chef to deploy [scollector](https://supermarket.chef.io/cookbooks/scollector) and another work in progress for deploying [Bosun](https://github.com/ptqa/chef-bosun).

### Slack Chat Room and Questions on Stack Overflow and Server Fault
> If you have questions you can get an invite to [https://bosun.slack.com/](http://bosun.org/slackInvite) or use the Bosun tag on [Stack Overflow](http://stackoverflow.com/questions/tagged/bosun) or [Server Fault](http://serverfault.com/questions/tagged/bosun).

  &nbsp;

## Presentations

### [Monitorama 2015 (PDX)](https://vimeo.com/131581326) by Kyle Brandt
> This presentation has screencasts of building an alert starting about 15 minutes in. The first 15 minutes explain some of the design principles behind Bosun. After the first screen cast of building an alert there are also some example alerts.

### An overview of creating [Scollector](http://bit.ly/2015AUG04) and [Bosun](http://go-talks.appspot.com/github.com/captncraig/presentations/bosun/bosun.slide#1) by Craig Peterson and Greg Bray
> Presentation give to the Utah Gophers meetup about creating a monitoring and alerting system in Go.

### [LISA 2014](https://www.usenix.org/conference/lisa14/conference-program/presentation/brandt) by Kyle Brandt
> This was the first presentation on Bosun which was given right after the initial release. A great introduction to the underlying thoughts on problems with alerting and how Bosun solves them.

  &nbsp;

## Libraries and Collectors

### [BosunReporter.NET](https://github.com/bretcope/BosunReporter.NET) by Bret Copeland
> A .NET client to send metrics to Bosun/OpenTSDB. It is designed to encourage best-practices while making it easy to create counters and gauges, including multi-aggregate gauges.

### [Bosun collect package](http://godoc.org/bosun.org/collect)
> A Go package used by Bosun, scollector, and [various other applications](http://godoc.org/bosun.org/collect?importers) to send metrics to Bosun/OpenTSDB. Includes limited support for multi-aggregate gauges.

### [httpUnit](http://godoc.org/github.com/StackExchange/httpunit)
> A Go package and command line tool used by scollector to send HTTP, HTTPS, and TCP test results to Bosun.

### [bosun-jmx](https://github.com/PeterGrace/bosun-jmx) by Peter Grace
> A external collector that relays JMX metrics to Bosun/OpenTSDB via scollector. Also a good starting point for a custom Java collector.

### [go-metrics-bosun](https://bitbucket.org/oneoffcode/go-metrics-bosun/src) by Richard Bucker
> A Bosun port for the [go-metrics](https://github.com/rcrowley/go-metrics) library based on Coda Hale's [Metrics](https://github.com/dropwizard/metrics) library.

### [vertx-bosun](https://github.com/cyngn/vertx-bosun) by Jeremy Truelove
> Sends metrics from [Vert.x](http://vertx.io/) applications to Bosun.

### [ekg-bosun](https://hackage.haskell.org/package/ekg-bosun)
> Send [ekg](https://github.com/tibbe/ekg) metrics from Haskell applications to Bosun.

### [bosun_emitter](https://github.com/lukaspustina/bosun_emitter) by Lukas Pustina
> bosun_emitter is a Rust library and CLI standalone binary called `emit_bosun` that may be used on the command line or from any shell script to send a single metric datum to Bosun.

### [opsgenie-bosun](https://github.com/TrentScholl/opsgenie-bosun)
> OpsGenie Marid integration for Bosun.

### [rs-collector](https://github.com/lukaspustina/rs-collector) by Lukas Pustina
> rs-collector is a Rust based, scollector compatible telemetry collector for services currently not covered by scollector. It collects selected metrics for Galera, JVM, MongoDB, Postfix etc.

_See also the list of built-in collectors included with [scollector](http://bosun.org/scollector/)_

  &nbsp;

## Tools

### [hubot-bosun](https://github.com/lukaspustina/hubot-bosun) by Lukas Pustina
> Bosun integration for [Hubot](https://hubot.github.com) allowing to ack and close incidents as well as to create and clear silences with optional prettified Slack output.

### [discreet-scollector](https://forge.puppet.com/discreet/scollector) by Chris Pisano
> Puppet module for installing and configuring the SCollector agent.

### [VS Code Plugin](https://marketplace.visualstudio.com/items?itemName=tbutterwith.bosun) by Tom Butterwith
> Syntax highlighting, openTSDB metric path completion and config validate extension for [VS Code](https://code.visualstudio.com/).

  &nbsp;

## [Screenshots and other Media](/media)
