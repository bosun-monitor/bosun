---
layout: default
title: Quick Start
redirect_from: /gettingstarted.html
---

<div class="row">
<div class="col-sm-3" >
  <div data-spy="affix" data-offset-top="0" data-offset-bottom="0" markdown="1">

 * Some TOC
 {:toc}

  </div>
</div>

<div class="doc-body col-sm-9" markdown="1">

<p class="h1 title">{{page.title}}</p>

{% raw %}

This document is written as a Quick-Start to getting Bosun working in your environment. By following this tutorial, you should have a fully operational Bosun system which not only is aggregating collected metrics from selected machines but also alerting you on relevant data about those systems.  We will be using OpenTSDB.  For some Graphite pointers, see the [graphite](#graphite) section below.

# Bosun

This guide is based on using our docker image. At Stack Exchange we do not use Docker in production. For those that do not wish to use docker, we provide binaries for bosun at bosun.org, but you will also need to install OpenTSDB and HBase yourself. (If you install OpenTSDB yourself, we recommend using the [next branch](https://github.com/opentsdb/opentsdb/tree/next), which has support for GZIP connections used by scollector.) For HBase we recommend building a cluster using Cloudera manager.

## Docker

### Install Docker

If you do not already have docker installed on your system, you can install it following the instructions outlined in [https://docs.docker.com/get-docker/](https://docs.docker.com/get-docker/).

**Note:** Don’t forget to ensure the docker daemon is running before moving forward!

### Running a Bosun container

There are two ways to run Bosun in Docker. For a very quick start, you can run the version of Bosun that is published to
Docker Hub. The latest version there is 0.6 which is significantly behind the latest code in GitHub.

Alternatively, you can clone the repository from [Github](https://github.com/bosun-monitor/bosun) for the latest version. 

#### From Docker Hub

To pull the latest version published to Docker Hub, execute the following command:

	$ docker run -d -p 4242:4242 -p 8070:8070 stackexchange/bosun

The above command tells the Docker daemon that you would like to start a new daemonized instance of bosun and you wish to port-forward 8070 of your server into the docker container. 
After about 30 seconds, you should have a working Bosun instance on port 8070.
You can navigate to the instance by opening a browser and heading to http://docker-server-ip:8070 where docker-server is your server running the docker daemon.

#### From the Github repository

Clone the [Github repository](https://github.com/bosun-monitor/bosun) into a directory of your choice. From that 
directory, run the following two commands: 
    
    $ cd docker
    $ docker-compose up
    
This will launch two containers. One which runs OpenTSDB and HBase, Bosun's dependencies; and a second one that runs
Bosun, [scollector](#scollector), and TSDBrelay. These three are the main components of the Bosun repository.

Your Bosun is available at http://localhost:8070. OpenTSDB is also available at http://localhost:4242.

## Getting data into Bosun

The Bosun docker image self populates a fair amount of data. See the [scollector](#scollector) section below if you'd like to know more, but you can skip it for now.

## Checking for data in Bosun

Once scollector is running, assuming there are no firewalls preventing communication between the host and server on port 8070, Bosun should be getting statistics from the scollector running on the system. We can check this by going to http://docker-server-ip:8070/items. If you see a list of metrics, congratulations! You're now receiving data. At the bottom of the page (or in a second column if the web browser window is wide enough), you will see the hostname(s) sending data. If you click the hostname, and then click “Available Metrics”, you will see all of the different types of data you can monitor! There is a lot of variables here, but there are some basic stats that we’ll use to explore in this tutorial.

## Creating an Alert

Collecting metrics about our systems is fun but what makes a monitoring system useful is alerting when anomalies arise. This is the real strength of Bosun.

Bosun encourages a particular workflow that makes it easy to design, test, and deploy an alert. If you look at the top of the Bosun display, the tabs include Items, Graph, Expression, Rule, and Test config in left-to-right order; that reflects the phases you go through as you create an alert. In general, first you'll select an item (metric) that is the basis of the alert. Next you'll graph it to understand its behavior. You'll then turn that graph into an expression, and the expression will be used to build a rule. You can then test the rule before incorporating it into Bosun.

Let's do an example to see how this works.In our example, we will setup an alert that notifies us about high cpu. The metric we'll focus on is "os.cpu". We will create an alert that triggers if a particular host has high CPU for an hour.

Go to http://docker-server-ip:8070 to get started.

### Items

Click on the "Items" tab. You'll see a list of all the labels (names) used in metrics currently stored. Click on "os.cpu" and you'll be taken to the Graph tab with that metric pre-loaded.

### Graph

You should see the Graph tab with that metric pre-loaded and a graph displayed for all hosts. We want a single host, so enter in your hostname in that field and click the blue “Query” button. A new graph should show up. This graph is showing the last hour of cpu usage. Since you’ve only had your scollector running for a few minutes, you may not have a lot of data yet, but that’s not a problem for our tutorial.

(ProTip: You can get the same results by clicking on the Items tab, clicking on the host you are interested in, then the "Available Metrics" tab. Clicking on one of the metrics you see there will bring you to the Graph tab with both the metric name and the host name pre-filled.)

Now that you have a graph, if you scroll to the bottom of the page there is a section called “Queries.” This section shows you the syntax of the query used to generate the graph.

Also on the bottom of this page are links called "Expression" and "Rule". These take your current workspace and populate the Expression or Rule tabs respectively. The Expression tab lets us fine-tune the rule and is generally what you want to use. The Rule button skips the expression editor and takes you directly to the rule editor.

For the purpose of this demo, click on the Expression button.

### Expression

The expression page allows us to tweak the data set for our query. The expression bar should currently have a line that begins with “q(“sum:rate…” This is the recipe that tells Bosun you’re looking for the os.cpu metric for the past 1 hour. If you click the “show” button under the result column in the Queries section, you will see all of the data points as they were graphed. Each data point is a timestamp and a value.

In the course of making an alert, however, we are probably not interested in a huge set of numbers. We might instead want something like the average.

To get the average of the data points, we will surround our query in avg(). So, the query will go from this:

	q("sum:rate{counter,,1}:os.cpu{host=your-system-here}", "1h", "")

To this:

	avg(q("sum:rate{counter,,1}:os.cpu{host=your-system-here}", "1h", ""))

If we click the blue “Test” button, we’ll see the result column show a single number, which is the arithmetic average of all of the data points. At this point, we’ve now got a number we can use to alert whether our average cpu usage is too high. Let us click the “Rule” button, which is right-justified on the same line as the Test button.

### Rule

On the Rule page, we have two boxes, the Alert box and Template box. The alert box shows us the basic barebones alert that Bosun has generated for us based on what we’ve done on the previous graph and expression pages. The template shows the basic template that produces the outbound e-mail alert that Bosun would send out. Currently, the alert is set to go critical all the time. The reason is that the crit and warn variables are boolean. By virtue of us putting our average cpu in the crit field, it becomes nonzero and therefore true. We need to add some more logic into this alert to make it meaningful.

Change the alert in the box to this:

	alert cpu.is.too.high {
		template = test
		$metric = q("sum:rate{counter,,1}:os.cpu{host=your-system-here}", "1h", "")
		$avgcpu = avg($metric)
		crit = $avgcpu > 80
		warn = $avgcpu > 60
	}

This alert, if triggered, would produce a critical alarm if the average cpu is over 80%, and a warning alarm if the average cpu is over 60%. Now, there is still one thing that makes this alert somewhat useless, and that is the fact that we’re only targeting one host (your-system-here.) If you want to use this alert for all of your hosts, you can change host=your-system-here to host=* and the alert will calculate against all hosts! If there are certain hosts you do not wish to be part of the query, you can use the squelch directive in the alert body, but that’s beyond the scope of our quickstart.

Click the Test button towards the right side of the page, below the Template box. In the Results pane below, you should see a summary of all of your hosts and what status they are in, be it Critical, Warning or Normal. If you click the “Template” pane, you’ll see what is e-mailed out in the alert. The default template isn’t very awesome, so lets replace it with something nice and meaningful:

	template test {
		subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}
		body = `<p>Alert: {{.Alert.Name}} triggered on {{.Group.host}}
		<hr>
		<p><strong>Computation</strong>
		<table>
			{{range .Computations}}
				<tr><td><a href="{{$.Expr .Text}}">{{.Text}}</a></td><td>{{.Value}}</td></tr>
			{{end}}
		</table>
		<hr>
		{{ .Graph .Alert.Vars.metric }}
		<hr>
		<p><strong>Relevant Tags</strong>
		<table>
			{{range $k, $v := .Group}}
				<tr><td>{{$k}}</td><td>{{$v}}</td></tr>
			{{end}}
		</table>`
	}

When you hit “test” after putting the above template into the template field, the Template pane at the bottom of the page will show the results of our alert. As you can see in the template output, we can show a graph in the alert to give visual learners a bit more context to the alert. This is an svg and should display properly in most e-mail clients when e-mailed.

## Persisting your alert

All of the steps thus far have been geared towards getting your feet wet with Bosun. At this point, you have an alert for high cpu that produces a rather nice-looking alert, but at this point Bosun isn’t going to alert on it. In order for the alert to be incorporated into bosun, it must be added to the config file. We can test the syntax of our alert and config file by going to the “Test Config” pane of Bosun, or navigate directly at http://docker-server-ip:8070/config. Paste in your alert and template fields as shown above to the end of the config file and hit the test button. If Bosun says the config is valid, you are free to copy the config from that window and overwrite the existing bosun.conf file with your new alert and template.  To accomplish this, you may wish to use `docker exec` and modify `/data/bosun.conf` then restart bosun.

# scollector

Bosun relies on metrics provided by other programs. For the majority of metrics we will be using a program called **scollector**. scollector is an agent that runs on hosts and will produce valuable output data about the state of that system. scollector also allows you to write custom collectors which permit you to record data that the basic scollector program does not gather. scollector is already installed and running on the docker image.

Binaries are available for Linux, Windows, and Mac at [http://bosun.org/scollector/](http://bosun.org/scollector/).

## Configuring scollector

By default, scollector will send data to `http://bosun:80`. scollector can be configured to send to different server by specifying a host with the **-h** flag:

	$ scollector -h docker-server-ip:8070

You may instead create a `scollector.conf` file alongside the scollector binary with the following contents:

	host=docker-server-ip:8070

See the [scollector docs](http://godoc.org/bosun.org/cmd/scollector) for more information.

# graphite

Next to OpenTSDB, Bosun also supports querying Graphite and Logstash-Elasticsearch.
You can execute, view and graph expressions, develop and run Graphite/LS alerting rules, get notifications and use the dashboard.
The OpenTSDB specific feature, such as data proxying and the built in general purpose graphing interface don't apply here.
The alerting rules look the same, in fact the only difference is you will query data using [graphite specific functions](http://bosun.org/expressions#graphite-query-functions) such as graphiteQuery and graphiteBand.

Start Graphite in docker:

    $ docker run -d \
      --name graphite \
      -p 80:80 \
      -p 2003:2003 \
      -p 8125:8125/udp \
      hopsoft/graphite-statsd

[Collectd](http://collectd.org/) is commonly used to submit metrics into Graphite. (scollector does not support Graphite).
You can easily launch it like so:

    $ docker run -e HOST_NAME=localhost -e GRAPHITE_HOST=<your host eth0 ip> andreasjansson/collectd-write-graphite

verify that http://localhost loads with the graphite interface, go into Graphite> localhost> cpu, go into the hierarchy and toggle on some of the metrics. it might take a minute or two before data starts showing up.

In your config, set

    graphiteHost = http://localhost

Now you can run alerting rules like so:

    alert os.high.cpu {
        template = generic
        $d = graphite("*.cpu.*.cpu.idle)", "5m", "", "host..core..type")
        $q = avg($d)
        # purposely very harsh tresholds so we definitely get some alerts
        warn = $q < 100
        crit = $q <= 97
    }

the 4th argument of the graphite function is the format of how to parse the series that graphite will return. in this case the first field is the host, the 3rd the core, and the last the cpu usage type, so these fields will be turned into tags within bosun.


{% endraw %}
  </div>
</div>
