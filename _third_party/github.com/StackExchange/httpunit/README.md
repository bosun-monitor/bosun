# httpUnit

httpUnit tests web and net servers for basic functionality.

The tool can open an http/https connection and verify that the expected
status code is received. It can also verify that the resulting HTML output
contains a string or regular expression. It can direct the request to a
particular IP address, ignoring DNS (similar to `curl --resolve`). The tool
can also open a TCP connection and verify that the connection was completed.
For https connections it will output various TLS-related information.

Tests can be input three ways:

 * *Command line:* A single test can be listed on the command line. This is useful for interactive debugging.
 * *TOML file:* A list of tests, with a full range of features, can be listed in a TOML file. This is the recommended mode for tests done on a regular basis. The format is described below.
 * *JSON/Hiera mode:* A simple list tcp tests can be specified in JSON format. These are in the format of a iptables "set". This mode is highly specific to a local requirement.

When specifying a single test on the command line, the only tests performed
are status code, and regex. If url does not contain a scheme ("https://",
"http://"), "http://" is prefixed. The IP may be an empty string to indicate
all IP addresses resolved to from the URL's hostname.

Usage:

	httpUnit [flag] [-hiera="path/to/sets.json"] [-toml="/path/to/httpunit.toml"] [url] [ip] [code] [regex]

The flags are:

	-filter=""
		if specified, only uses this IP address; may end with "." to
		filter by IPs that start with the filter
	-no10=false
		no RFC1918 addresses
	-timeout="3s"
		connection timeout
	-header="X-Request-Guid"
		in more verbose mode, print this HTTP header
	-v
		verbose output: show successes
	-vv
		more verbose output: show -header, cert details

### URLs

URLs may be specified with various protocols: http, https, tcp,
udp, ip. "4" or "6" may be appended to tcp, udp, and ip (as per
[net/#Dial](http://golang.org/pkg/net/#Dial])). tcp and udp must specify
a port, or default to 0. http and https may specify a port to override
the default.

### TOML

The [toml file](https://github.com/toml-lang/toml) has two sections:
`Plan` is a list of test plans.
`IPs` are a table of search and replace regexes.

Each `[[plan]]` lists:

 * `label =` A label for documentation purposes. It must be unique.
 * `url =` The URL to retrieve.
 * `ips =` For http/https, a list of IPs to send the URL to. Default is "use DNS". Otherwise the connection is made to the IP address listed, ignoring DNS.
 * `code =` For http/https, the expected status code, default 200.
 * `string =` For http/https, a string we expect to find in the result.
 * `regex =` For http/https, a regular expression we expect to match in the result.

The test plan is run once for each item in the ips list, or more if macros
are in effect.

In the `ips` list, `*` will be substituted by all the A and AAAA records
returned when DNS is performed on the hostname of the URL.

The `[[IPs]]` section is for defining macros. Here are some typical use-cases:

Specify a value _n_ to mean "10.0.0.n". This may save you a lot of typing
in a big configuration file:

	'^(\d+)$' = ["10.0.0.$1"]

Similar to the previous example, but specify a base address:

	BASEIP = ["10.0.0."]
	'^(\d+)$' = ["BASEIP$1"]

Specify a value _n_ to mean the 16th IP address many CIDR bocks:

	'^(\d+)$' = ["10.0.0.$1", "10.1.1.$1", "10.2.2.$1", "10.3.3.$1", "10.4.4.$1"]

Specify a value _nINT_ to mean .n and .n+64, plus the whatever DNS returns:

	BASEIP = ["10.0.0."]
	'^(\d+)$' = ["BASEIP$1", "BASEIP($1+64)", "*"]

## Why we made this?

This tool makes it easy to do test-driven development on your web server.

Every request to our web server passes through three systems that are all
complex and error prone: the firewall, the load balancer, and the web server
itself.  Even when running in a test environment with automated tools that
generate the configurations we still needed a way to test our results.

Before this tool each change was followed by a few simple manual tests,
often by manually typing `curl` commands. We missed a lot of errors.

With this tool, we now have hundreds of tests that we can run with a single
command. The tests run in parallel therefore all the testing is done very
quickly.

When we need to make a change we first add the test, then we proceed making
the change until the test passes. This test-driven development has accumulated
200 tests that we run for any change, considerably more than we'd ever run
manual. This improves confidence in our ability to make changes quickly.

While making unrelated changes we often run it in a loop to make sure we
don't unintentionally break anything.

The command-line mode has been useful both in development of changes, and
diagnosing outages.

## A simple example file:

	# Verify that this URL returns text that matches "some regex":
	[[plan]]
	  label = "api"
	  url = "http://api.example.com/"
	  text = "API for example.com"
	  regex = "some regex"

	# Verify that this URL returns a redirect. Send to both
	# the IP address listed in DNS, plus 10.11.22.33 and 10.99.88.77.
	[[plan]]
	  label = "redirect"
	  url = "https://example.com/redirect"
	  ips = ["*", "10.11.22.33", "10.99.88.77"]
	  code = 301

## A more complex example file:

In this example we want an IP address to mean the IP address, but if we
specify a single number (e.g. "16") we want that to expand to the .16 address
of a few different CIDR blocks. We also want to be able to specify a number +
INT (e.g. "16INT") to indicate a slightly diffenent list.

	[IPs]
	  BASEIP = ["87.65.43."]
	  '^(\d+)$' = ["*", "BASEIP$1", "BASEIP($1+64)", "1.2.3.$1"]
	  '^(\d+)INT$' = ["10.0.1.$1", "10.0.2.$1", "BASEIP$1", "BASEIP($1+64)"]
	
	[[plan]]
	  label = "api"
	  url = "http://api.example.com/"
	  # This will generate the DNS A/AAAA records, 87.65.43.16, 87.65.43.80, 1.2.3.16 and 8.7.6.5:
	  ips = ["16", "8.7.6.5"]
	  text = "API for example.com"
	  regex = "some regex"
	
	[[plan]]
	  label = "redirect"
	  url = "https://example.com/redirect"
	  # This will generate the DNS A/AAAA records, 10.0.1.20, 10.0.2.20, 87.65.43.20, 87.65.43.84:
	  ips = ["*", "20INT"]
	  code = 301
	
	[[plan]]
	  label = "mail"
	  url = "tcp://mail-host.com:25"
