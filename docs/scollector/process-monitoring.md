---
layout: default
title: Process Monitoring
---
  &nbsp;

## Monitoring Processes with Scollector

### Linux

Scollector will monitor any Linux processes specified in the configuration file.

{% highlight cfg %}
[[Process]]
  Command = "/opt/bosun/bosun"
  Name = "bosun"

[[Process]]
  Command = "ruby"
  Name = "puppet-agent"
  Args = "puppet"

[[Process]]
  Command = "/haproxy$"
  Name = "haproxy-t1"
  Args = "/etc/haproxy-t1/haproxy-t1.cfg"

[[Process]]
  Command = '/usr/bin/redis-server \*:16389'
  Name = "redis-bosun-dev"
  IncludeCount = true
{% endhighlight %}

**Command** is a partial text match (older versions) or regular expression (as of 0.5.0-rc1) against the full path of all executing programs. You can use just the executable or include part of the path like "/bin/java". This will not match against arguments except when the cmdline is rewritten by the program (See below).

**Name** is the value that will be used in the name tag of the `linux.proc.*` and `os.proc.*` metrics that are created. If Name is not specified then a sanitized version of the command text will be used as the name tag value.

**Args** is a optional regular expression (case sensitive by default) used to select a specific instance of a program. If multiple processes match the given Command and Args they will be assigned incrementing id tag values starting at 1.

**IncludeCount** is a optional boolean (default is false) to enable the `linux.proc.count` metric as a count of matched processes on each system. This can be used to create alerts when count = 0 or count > 1.

When setting up process monitoring you can use `ps -ef` to see a list of running processes and their arguments. Some processes change the command stored in /proc/_pid_/cmdline to be more user friendly, but scollector is looking for NUL delimited arguments so this can cause issues. For instance, to monitor a redis process you must look at the raw value of the cmdline and set the Command to match since the arguments are delimited by 0x20 spaces instead of 0x00 NUL bytes:

{% highlight text %}
cat /proc/80156/cmdline | hexdump -C
00000000  2f 75 73 72 2f 62 69 6e  2f 72 65 64 69 73 2d 73  |/usr/bin/redis-s|
00000010  65 72 76 65 72 20 2a 3a  36 33 37 39 00 00 00 00  |erver *:6379....|
00000020  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000030  00                                                |.|
00000031

#Example for cmdline without NUL (00) delimiters between args (using \ to escape * in regex)
[[Process]]
  Command = 'redis-server \*:6379'
  Name = "redis-core"
{% endhighlight %}

### Systemd Services

Scollector uses the D-Bus API to determine the state of services managed by systemd and specified in the configuration file.

{% highlight text %}
[[SystemdService]]
  Name = "^(puppet|redis-.*|keepalived|haproxy-t.*)$"
  WatchProc = false

[[SystemdService]]
  Name = "^(scollector|memcached)$"
  WatchProc = true
{% endhighlight %}

**Name** is a regular expression that matches against the service unit names. If the Name matches then it will be monitored under the `linux.systemd.unit.*` and `os.service.*` metrics.

**WatchProc** is an optional boolean (default is false) to specify if the primary process for the service should be included as a monitored linux process under `linux.proc.*` and `os.proc.*`. This may not work for processes that fork, use init scripts, or launch multiple sub processes.

  &nbsp;

### Windows

Scollector will monitor any Windows processes or services specified in the configuration file.

{% highlight cfg %}
[[Process]]
  Name = "^scollector"

[[Process]]
  Name = "^chrome"

[[Process]]
  Name = "^(MSSQLSERVER|SQLSERVERAGENT)$"
{% endhighlight %}

**Name** is a regular expression (case sensitive by default) that matches against the Name property of the service or process WMI classes. Use `gwmi Win32_Service` and `gwmi Win32_Process` in PowerShell to see a list of possible values. If the Name matches a Win32_Service name then that process will be monitored under the `win.service.*` and `os.service.*` metrics. If the name matches a process or a service (with ProcessId != 0) that process will be monitored under the `win.proc.*` and `os.proc.*` metrics. If there is more than one matching process they will be assigned incrementing id tag values starting at 1.

Note: IIS worker pool processes are automatically monitored using iis_pool-name as the name tag value. These will also have additional metrics under `iis.*`

### .NET Processes

Scollector can also monitor any Windows processes using the .NET framework. If no ProcessDotNet settings are specified it will default to just monitoring the w3wp worker processes for IIS. You can specify which applications to monitor in the configuration file.

{% highlight cfg %}
[[ProcessDotNet]]
  Name = "^w3wp"

[[ProcessDotNet]]
  Name = "LINQPad"
{% endhighlight %}

**Name** is a regular expression (case sensitive by default) that matches against the Name property of the [.NET Performance counters](https://msdn.microsoft.com/en-us/library/w8f5kw2e.aspx). Matching process will be monitored under the `dotnet.*` metrics, and if there is more than one matching process they will be assigned incrementing id tag values starting at 1. Where possible the w3wp names will be changed to match the iis_pool-names used for process monitoring.
