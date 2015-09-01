---
layout: default
title: External Collectors
---
  &nbsp;

## Using External Collectors with Scollector

External collectors are executables or scripts that scollector invokes, collects output from, and forwards data points on to the server like any other metric. These can be binaries, .sh/.py/.rb/.cmd/.bat/.ps1 scripts, or even .jar files ( See [InitPrograms](http://godoc.org/bosun.org/cmd/scollector/collectors#InitPrograms) function for implementation details ). External collectors are a great way to get started collecting data, but when possible it is recommended for applications to send data directly to Bosun or to update scollector so that it natively supports additional systems.

The [ColDir](http://godoc.org/bosun.org/cmd/scollector#hdr-Configuration_File) configuration key specifies the external collector directory, which is usually set to something like **/opt/scollector/collectors** in Linux or **C:\Program Files\scollector\collectors** in Windows. It should contain numbered directories just like the ones used in [OpenTSDB tcollector](http://opentsdb.net/docs/build/html/user_guide/utilities/tcollector.html#collecting-lots-of-metrics-with-tcollector). Each directory represents how often scollector will try to invoke the collectors in that folder (example: **60** = every 60 seconds). Use a directory named **0** for any executables or scripts that will run continuously and create output on their own schedule. Any non-numeric named directories will be ignored, and a lib and etc directory are often used for library and config data shared by all collectors.

  &nbsp;

### Simple data output format

The simple output format can be use to send raw metrics to scollector using the standard output stream. You won't be able to specify units, counter vs gauge, or descriptions, but those can always be specified manually in graphs/expressions or added later if desired.

{% highlight text %}
#Manually run script to see output: metric timestamp value tagk=tagv tagk=tagv
/opt/scollector/collectors/0/twitter
twitter.tweet_count 1441406996 0 query=stackoverflow-down
twitter.follower_count 1441406996 1337 account=stackoverflow
{% endhighlight %}

`Metric` is the name of the metric you want to create.

`Timestamp` is in Unix format (seconds since epoch).

`Value` is the floating point value that will be used for that data point.

`Tags` are optional. A host tag is automatically added, but overridden if specified.

`Errors` anything written to the standard error stream is included in the scollector logs (syslog on Linux, event log on Windows).

  &nbsp;

### JSON data output format

If you want to include metadata with your external collector metrics you can send JSON serialized instances of the [opentsdb.DataPoint](http://godoc.org/bosun.org/opentsdb#DataPoint) struct for metric data and the [metadata.Metasend](http://godoc.org/bosun.org/metadata#Metasend) struct for metadata. The JSON data would be streamed over standard output and look similar to the output listed below. It starts with metadata (only needed once, as scollector will re-send as needed) and then continues sending metrics. The 4th line is a log message which was sent to the standard error stream.

{% highlight text %}
{"metric":"exceptional.exceptions.count","name":"rate","value":"counter"}
{"metric":"exceptional.exceptions.count","name":"unit","value":"errors"}
{"metric":"exceptional.exceptions.count","name":"desc","value":"Exceptions per second stored in each Opserver data source."}
2015/08/05 15:32:00 lookup OR-SQL03: no such host
{"metric":"exceptional.exceptions.count","timestamp":1438788720,"value":5,"tags":{"application":"Careers","machine":"ny-web03","source":"NY_Status"}}
{"metric":"exceptional.exceptions.count","timestamp":1438788720,"value":0,"tags":{"application":"AdServer","machine":"ny-web03","source":"NY_Status"}}
{"metric":"exceptional.exceptions.count","timestamp":1438788720,"value":2,"tags":{"application":"AdServer","machine":"ny-web04","source":"NY_Status"}}
{"metric":"exceptional.exceptions.count","timestamp":1438788720,"value":1,"tags":{"application":"AdServer","machine":"ny-web06","source":"NY_Status"}}
{% endhighlight %}
