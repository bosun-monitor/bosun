---
layout: default
title: Expression Documentation
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

This section documents Bosun's expression language, which is used to define the trigger condition for an alert. At the highest level the expression language takes various time *series* and reduces them them a *single number*. True or false indicates whether the alert should trigger or not; 0 represents false (don't trigger an alert) and any other number represents true (trigger an alert). An alert can also produce one or more *groups* which define the alert's scope or dimensionality. For example could you have one alert per host, service, or cluster or a single alert for your entire environment.

# Fundamentals

## Data Types
There are three data types in Bosun's expression language:

 1. **Scalar**: This is the simplest type, it is a single numeric value with no group associated with it. Keep in mind that an empty group, `{}` is still a group.
 2. **NumberSet**: A number set is a group of tagged numeric values with one value per unique grouping. As a special case, a **scalar** may be used in place of a **numberSet** with a single member with an empty group.
 3. **SeriesSet**: A series is an array of timestamp-value pairs and an associated group.

In the vast majority of your alerts you will getting ***seriesSets*** back from your time series database and ***reducing*** them into ***numberSets***.

## Group keys
Groups are generally provided by your time series database. We also sometimes refer to groups as "Tags". When you query your time series database and get multiple time series back, each time series needs an identifier. So for example if I make a query with some thing like `host=*` then I will get one time series per host. Host is the tag key, and the various various values returned, i.e. `host1`, `host2`, `host3`.... are the tag values. Therefore the group for a single time series is something like `{host=host1}`. A group can have multiple tag keys, and will have one tag value for each key.

Each group can become its own alert instance. This is what we mean by ***scope*** or dimensionality. Thus, you can do things like `avg(q("sum:sys.cpu{host=ny-*}", "5m", "")) > 0.8` to check the CPU usage for many New York hosts at once. The dimensions can be manipulated with our expression language.

### Group Subsets
Various metrics can be combined by operators as long as one group is a subset of the other. A ***subset*** is when one of the groups contains all of the tag key-value pairs in the other. An empty group `{}` is a subset of all groups. `{host=foo}` is a subset of `{host=foo,interface=eth0}`, and neither `{host=foo,interface=eth0}` nor `{host=foo,parition=/}` are a subset of the other. Equal groups are considered subsets of each other.

## Operators

The standard arithmetic (`+`, binary and unary `-`, `*`, `/`, `%`), relational (`<`, `>`, `==`, `!=`, `>=`, `<=`), and logical (`&&`, `||`, and unary `!`) operators are supported. The binary operators require the value on at least one side to be a scalar or NumberSet. Arrays will have the operator applied to each element. Examples:

* `q("q") + 1`, which adds one to every element of the result of the query `"q"`
* `-q("q")`, the negation of the results of the query
* `5 > q("q")`, a series of numbers indicating whether each data point is more than five
* `6 / 8`, the scalar value three-quarters

### Precedence

From highest to lowest:

1. `()` and the unary operators `!` and `-`
1. `*`, `/`, `%`
1. `+`, `-`
1. `==`, `!=`, `>`, `>=`, `<`, `<=`
1. `&&`
1. `||`

## Numeric constants

Numbers may be specified in decimal (e.g., `123.45`), octal (with a leading zero like `072`), or hex (with a leading 0x like `0x2A`). Exponentials and signs are supported (e.g., `-0.8e-2`).

# The Anatomy of a Basic Alert
<pre>
alert haproxy_session_limit {
    template = generic
    $notes = This alert monitors the percentage of sessions against the session limit in haproxy (maxconn) and alerts when we are getting close to that limit and will need to raise that limit. This alert was created due to a socket outage we experienced for that reason
    $current_sessions = max(q("sum:haproxy.frontend.scur{host=*,pxname=*,tier=*}", "5m", ""))
    $session_limit = max(q("sum:haproxy.frontend.slim{host=*,pxname=*,tier=*}", "5m", ""))
    $q = ($current_sessions / $session_limit) * 100
    warn = $q > 80
    crit = $q > 95
    warnNotification = default
    critNotificaiton = default
}
</pre>

We don't need to understand everything in this alert, but it is worth highlighting a few things to get oriented:

 * `haproxy_session_limit` This is the name of the alert, an alert instance is uniquely identified by its alertname and group, i.e `haproxy_session_limit{host=lb,pxname=http-in,tier=2}`
 * `$notes` This is a variable. Variables are not smart, they are just text replacement. If you are familiar with macros in C, this is a similar concept. These variables can be referenced in notification templates which is why we have a generic one for notes
 * `q("sum:haproxy.frontend.scur{host=*,pxname=*,tier=*}", "5m", "")` is an OpenTSDB query function, it returns *N* series, we know each series will have the host, pxname, and tier tag keys in their group based on the query.
 * `max(...)` is a reduction function. It takes each **series** and **reduces** it to a **number** (See the Data types section above).
 * `$current_sessions / $session_limit` these variables represent **numbers** and will have subset group matches so there for you can use the / **operator** between them.
 *  `warn = $q > 80` if this is true (non-zero) then the `warnNotification` will be triggered.

# Query Functions

## Graphite Query Functions

### GraphiteQuery(query string, startDuration string, endDuration string, format string) seriesSet

Performs a graphite query.  the duration format is the internal bosun format (which happens to be the same as OpenTSDB's format).
Functions pretty much the same as q() (see that for more info) but for graphite.
The format string lets you annotate how to parse series as returned by graphite, as to yield tags in the format that bosun expects.
The tags are dot-separated and the amount of "nodes" (dot-separated words) should match what graphite returns.
Irrelevant nodes can be left empty.

For example:

`groupByNode(collectd.*.cpu.*.cpu.idle,1,'avg')`

returns seriesSet named like `host1`, `host2` etc, in which case the format string can simply be `host`.

`collectd.web15.cpu.*.cpu.*`

returns seriesSet named like `collectd.web15.cpu.3.idle`, requiring a format like  `.host..core..cpu_type`.

For advanced cases, you can use graphite's alias(), aliasSub(), etc to compose the exact parseable output format you need.
This happens when the outer graphite function is something like "avg()" or "sum()" in which case graphite's output series will be identified as "avg(some.string.here)".

### GraphiteBand(query string, duration string, period string, format string, num string) seriesSet

Like band() but for graphite queries.

## Logstash Query Functions

### lscount(indexRoot string, keyString string, filterString string, bucketDuration string, startDuration string, endDuration string) seriesSet

lscount returns the per second rate of matching log documents.

  * `indexRoot` is the root name of the index to hit, the format is expected to be `fmt.Sprintf("%s-%s", index_root, d.Format("2006.01.02"))`.
  * `keyString` creates groups (like tagsets) and can also filter those groups. It is the format of `"field:regex,field:regex..."` The `:regex` can be ommited.
  * `filterString` is an Elastic regexp query that can be applied to any field. It is in the same format as the keystring argument.
  * `bucketDuration` is in the same format is an opentsdb duration, and is the size of buckets returned (i.e. counts for every 10 minutes). In the case of lscount, that number is normalized to a per second rate by dividing the result by the number of seconds in the duration.
  * `startDuration` and `endDuration` set the time window from now - see the OpenTSDB q() function for more details.

For example:

`lscount("logstash", "logsource,program:bosun", "5s", "10m", "")`

queries the "logstash" named indexes (we autogenerate the date porition of the indexes based on the time frame) and returns a series with groups like `{logsrouce:ny-bosun01, program:bosun}, {logsrouce:ny-bosun02, program:bosun}`. The values of the series will be the count of log entries in 5 second buckets over the last 10 minutes.

### lsstat(indexRoot string, keyString string, filterString string, field string, rStat(avg|min|max|sum|sum_of_squares|variance|std_deviation) string, bucketDuration string, startDuration string, endDuration string) series

lstat returns various summary stats per bucket for the specified `field`. The field must be numeric in elastic. rStat can be one of `avg`, `min`, `max`, `sum`, `sum_of_squares`, `variance`, `std_deviation`. The rest of the fields behave the same as lscount except that there is no division based on `bucketDuration` since these are summary stats.

### Caveats
  * There is currently no escaping in the keystring, so if you regex needs to have a comma or double quote you are out of luck.
  * The regexs in keystring are applied twice. First as a regexp filter to elastic, and then as a go regexp to the keys of the result. This is because the value could be an array and you will get groups that should be filtered. This means regex language is the intersection of the golang regex spec and the elastic regex spec.
  * Elastic uses lucene style regex. This means regexes are always anchored ([see the documentation](http://www.elasticsearch.org/guide/en/elasticsearch/reference/current/query-dsl-regexp-query.html#_standard_operators)).
  * If the type of the field value in Elastic (aka the mapping) is a number then the regexes won't act as a regex. The only thing you can do is an exact match on the number, ie "eventlogid:1234". It is recommended that anything that is a identifier should be stored as a string since they are not numbers even if they are made up entirely of numerals.
  * As of January 15, 2015 - logstash functionality is new so these functions may change a fair amount based on experience using them in alerts.
  * Alerts using this information likely want to set ignoreUnknown, since only "groups" that appear in the time frame are in the results.

## OpenTSDB Query Functions

Query functions take a query string (like `sum:os.cpu{host=*}`) and return a seriesSet.

### q(query string, startDuration string, endDuration string) seriesSet

Generic query from endDuration to startDuration ago. If endDuration is the empty string (`""`), now is used. Support d( units are listed in [the docs](http://opentsdb.net/docs/build/html/user_guide/query/dates.html). Refer to [the docs](http://opentsdb.net/docs/build/html/user_guide/query/index.html) for query syntax. The query argument is the value part of the `m=...` expressions. `*` and `|` are fully supported. In addition, queries like `sys.cpu.user{host=ny-*}` are supported. These are performed by an additional step which determines valid matches, and replaces `ny-*` with `ny-web01|ny-web02|...|ny-web10` to achieve the same result. This lookup is kept in memory by the system and does not incur any additional OpenTSDB API requests, but does require scollector instances pointed to the bosun server.

### band(query string, duration string, period string, num scalar) seriesSet

Band performs `num` queries of `duration` each, `period` apart and concatenates them together, starting `period` ago. So `band("avg:os.cpu", "1h", "1d", 7)` will return a series comprising of the given metric from 1d to 1d-1h-ago, 2d to 2d-1h-ago, etc, until 8d. This is a good way to get a time block from a certain hour of a day or certain day of a week over a long time period.

### change(query string, startDuration string, endDuration string) numberSet

Change is a way to determine the change of a query from startDuration to endDuration. If endDuration is the empty string (`""`), now is used. The query must either be a rate or a counter converted to a rate with the `agg:rate:metric` flag.

For example, assume you have a metric `net.bytes` that records the number of bytes that have been sent on some interface since boot. We could just subtract the end number from the start number, but if a reboot or counter rollover occurred during that time our result will be incorrect. Instead, we ask OpenTSDB to convert our metric to a rate and handle all of that for us. So, to get the number of bytes in the last hour, we could use:

`change("avg:rate:net.bytes", "60m", "")`

Note that this is implemented using the bosun's `avg` function. The following is exactly the same as the above example:

`avg(q("avg:rate:net.bytes", "60m", "")) * 60 * 60`

### count(query string, startDuration string, endDuration string) scalar

Count returns the number of groups in the query as an ungrouped scalar.

### window(query string, duration string, period string, num scalar, funcName string) seriesSet

Window performs `num` queries of `duration` each, `period` apart, starting
`period` ago. The results of the queries are run through `funcName` which
must be a reduction function taking only one argument (that is, a function
that takes a series and returns a number), then a series made from those. So
`window("avg:os.cpu{host=*}", "1h", "1d", 7, "dev")` will return a series
comprising of the average of given metric from 1d to 1d-1h-ago, 2d to
2d-1h-ago, etc, until 8d. It is similar to the band function, except that
instead of concatenating series together, each series is reduced to a number,
and those numbers created into a series.

# Reduction Functions

All reduction functions take a seriesSet and return a numberSet with one element per unique group.

## avg(seriesSet) numberSet

Average (arithmetic mean).

## cCount(seriesSet) numberSet

Returns the change count which is the number of times in the series a value was not equal to the immediate previous value. Useful for checking if things that should be at a steady value are "flapping". For example, a series with values [0, 1, 0, 1] would return 3.

## dev(seriesSet) numberSet

Standard deviation.

## diff(seriesSet) numberSet

Diff returns the last point of each series minus the first point.

## first(seriesSet) numberSet

Returns the first (least recent) data point in each series.

## forecastlr(seriesSet, y_val numberSet|scalar) numberSet

Returns the number of seconds until a linear regression of each series will reach y_val.

## last(seriesSet) numberSet

Returns the last (most recent) data point in each series.

## len(seriesSet) numberSet

Returns the length of each series.

## max(seriesSet) numberSet

Returns the maximum value of each series, same as calling percentile(series, 1).

## median(seriesSet) numberSet

Returns the median value of each series, same as calling percentile(series, .5).

## min(seriesSet) numberSet

Returns the minimum value of each series, same as calling percentile(series, 0).

## percentile(seriesSet, p numberSet|scalar) numberSet

Returns the value from each series at the percentile p. Min and Max can be simulated using `p <= 0` and `p >= 1`, respectively.

## since(seriesSet) numberSet

Returns the number of seconds since the most recent data point in each series.

## streak(seriesSet) numberSet

Returns the length of the longest streak of values that evaluate to true (i.e. max amount of contiguous non-zero values found).

## sum(seriesSet) numberSet

Sum.

# Group Functions

Group functions modify the OpenTSDB groups.

## t(numberSet, group string) seriesSet

Transposes N series of length 1 to 1 series of length N. If the group parameter is not the empty string, the number of series returned is equal to the number of tagks passed. This is useful for performing scalar aggregation across multiple results from a query. For example, to get the total memory used on the web tier: `sum(t(avg(q("avg:os.mem.used{host=*-web*}", "5m", "")), ""))`.

How transpose works conceptually

Transpose Grouped results into a Single Result:  

Before Transpose (Value Type is NumberSet):  

Group       | Value  |
----------- | ----- |
{host=web01} | 1 |
{host=web02} | 7 |
{host=web03} | 4 |

After Transpose (Value Type is SeriesSet):  

Group        | Value  |
----------- | ----- |
{} | 1,7,4 |

Transpose Groups results into Multiple Results:  

Before Transpose by host (Value Type is NumberSet)  

Group        | Value  |
----------- | ----- |
{host=web01,disk=c} | 1 |
{host=web01,disc=d} | 3 |
{host=web02,disc=c} | 4 |

After Transpose by "host" (Value type is SeriesSet)  

Group        | Value  |
------------ | ------ |
{host=web01} | 1,3 |
{host=web02} | 4 |

Useful Example of Transpose
Alert if more than 50% of servers in a group have ping timeouts

~~~
  alert or_down {
    $group = host=or-*
    # bosun.ping.timeout is 0 for no timeout, 1 for timeout
    $timeout = q("sum:bosun.ping.timeout{$group}", "5m", "")
    # timeout will have multiple groups, such as or-web01,or-web02,or-web03.
    # each group has a series type (the observations in the past 10 mintutes)
    # so we need to *reduce* each series values of each group into a single number:
    $max_timeout = max($timeout)
    # Max timeout is now a group of results where the value of each group is a number. Since each
    # group is an alert instance, we need to regroup this into a sigle alert. We can do that by
    # transposing with t()
    $max_timeout_series = t("$max_timeout", "")
    # $max_timeout_series is now a single group with a value of type series. We need to reduce
    # that series into a single number in order to trigger an alert.
    $number_down_series = sum($max_timeout_series)
    $total_servers = len($max_timeout_series)
    $percent_down = $number_down_servers / $total_servers) * 100
    warnNotificaiton = $percent_down > 25
  }
~~~

Since our templates can reference any variable in this alert, we can show which servers are down in the notification, even though the alert just triggers on 25% of or-\* servers being down.

## ungroup(numberSet) scalar

Returns the input with its group removed. Used to combine queries from two differing groups.

# Other Functions

## alert(name string, key string) numberSet

Executes and returns the `key` expression from alert `name` (which must be
`warn` or `crit`). Any alert of the same name that is unknown or unevaluated
is also returned with a value of `1`. Primarily for use with `depends`.

Example: `alert("host.down", "crit")` returns the crit
expression from the host.down alert.

## abs(numberSet) numberSet

Returns the absolute value of each element in the numberSet.

## d(string) scalar

Returns the number of seconds of the [OpenTSDB duration string](http://opentsdb.net/docs/build/html/user_guide/query/dates.html).

## des(series, alpha scalar, beta scalar) series

Returns series smoothed using Holt-Winters double exponential smoothing. Alpha
(scalar) is the data smoothing factor. Beta (scalar) is the trend smoothing
factor.

## dropg(seriesSet, threshold numberSet|scalar) seriesSet

Remove any values greater than number from a series. Will error if this operation results in an empty series.

## dropge(seriesSet, threshold numberSet|scalar) seriesSet

Remove any values greater than or equal to number from a series. Will error if this operation results in an empty series.

## dropl(seriesSet, threshold numberSet|scalar) seriesSet

Remove any values lower than number from a series. Will error if this operation results in an empty series.

## drople(seriesSet, threshold numberSet|scalar) seriesSet

Remove any values lower than or equal to number from a series. Will error if this operation results in an empty series.

## dropna(seriesSet) seriesSet

Remove any NaN or Inf values from a series. Will error if this operation results in an empty series.

## epoch() scalar

Returns the Unix epoch in seconds of the expression start time (scalar).

## filter(seriesSet, numberSet) seriesSet

Returns all results in seriesSet that are a subset of numberSet and have a non-zero value. Useful with the limit and sort functions to return the top X results of a query.

## limit(numberSet, count scalar) numberSet

Returns the first count (scalar) results of number.

## lookup(table string, key string) numberSet

Returns the first key from the given lookup table with matching tags.

## nv(numberSet, scalar) numberSet

Change the NaN value during binary operations (when joining two queries) of unknown groups to the scalar. This is useful to prevent unknown group and other errors from bubbling up.

## rename(seriesSet, string) seriesSet

Accepts a series and a set of tags to rename in `Key1=NewK1,Key2=NewK2` format. All data points will have the tag keys renamed according to the spec provided, in order. This can be useful for combining results from seperate queries that have similar tagsets with different tag keys.

## sort(numberSet, (asc|desc) string) numberSet

Returns the results sorted by value in ascending ("asc") or descending ("desc")
order. Results are first sorted by groupname and then stably sorted so that
results with identical values are always in the same order.

</div>
