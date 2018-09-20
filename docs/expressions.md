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
 4. **VariantSet**: This is for generic functions. It can be a NumberSet, a SeriesSet, or Scalar. In the case of a NumberSet of a SeriesSet that same type will be returned, in the case of a Scalar a NumberSet is returned. Therefore the VariantSet type is never returned.

In the vast majority of your alerts you will getting ***seriesSets*** back from your time series database and ***reducing*** them into ***numberSets***.

## Group keys
Groups are generally provided by your time series database. We also sometimes refer to groups as "Tags". When you query your time series database and get multiple time series back, each time series needs an identifier. So for example if I make a query with some thing like `host=*` then I will get one time series per host. Host is the tag key, and the various various values returned, i.e. `host1`, `host2`, `host3`.... are the tag values. Therefore the group for a single time series is something like `{host=host1}`. A group can have multiple tag keys, and will have one tag value for each key.

Each group can become its own alert instance. This is what we mean by ***scope*** or dimensionality. Thus, you can do things like `avg(q("sum:sys.cpu{host=ny-*}", "5m", "")) > 0.8` to check the CPU usage for many New York hosts at once. The dimensions can be manipulated with our expression language.

### Group Subsets
Various metrics can be combined by operators as long as one group is a subset of the other. A ***subset*** is when one of the groups contains all of the tag key-value pairs in the other. An empty group `{}` is a subset of all groups. `{host=foo}` is a subset of `{host=foo,interface=eth0}`, and neither `{host=foo,interface=eth0}` nor `{host=foo,partition=/}` are a subset of the other. Equal groups are considered subsets of each other.

## Operators

The standard arithmetic (`+`, binary and unary `-`, `*`, `/`, `%`), relational (`<`, `>`, `==`, `!=`, `>=`, `<=`), and logical (`&&`, `||`, and unary `!`) operators are supported. Examples:

* `q("q") + 1`, which adds one to every element of the result of the query `"q"`
* `-q("q")`, the negation of the results of the query
* `5 > q("q")`, a series of numbers indicating whether each data point is more than five
* `6 / 8`, the scalar value three-quarters

### Series Operations

If you combine two seriesSets with an operator (i.e. `q(..)` + `q(..)`), then operations are applied for each point in the series if there is a corresponding datapoint on the right hand side (RH). A corresponding datapoint is one which has the same timestamp (and normal group subset rules apply). If there is no corresponding datapoint on the left side, then the datapoint is dropped. This is a new feature as of 0.5.0.

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
    $query = ($current_sessions / $session_limit) * 100
    warn = $query > 80
    crit = $query > 95
    warnNotification = default
    critNotification = default
}
</pre>

We don't need to understand everything in this alert, but it is worth highlighting a few things to get oriented:

 * `haproxy_session_limit` This is the name of the alert, an alert instance is uniquely identified by its alertname and group, i.e `haproxy_session_limit{host=lb,pxname=http-in,tier=2}`
 * `$notes` This is a variable. Variables are not smart, they are just text replacement. If you are familiar with macros in C, this is a similar concept. These variables can be referenced in notification templates which is why we have a generic one for notes
 * `q("sum:haproxy.frontend.scur{host=*,pxname=*,tier=*}", "5m", "")` is an OpenTSDB query function, it returns *N* series, we know each series will have the host, pxname, and tier tag keys in their group based on the query.
 * `max(...)` is a reduction function. It takes each **series** and **reduces** it to a **number** (See the Data types section above).
 * `$current_sessions / $session_limit` these variables represent **numbers** and will have subset group matches so there for you can use the / **operator** between them.
 *  `warn = $query > 80` if this is true (non-zero) then the `warnNotification` will be triggered.

# Query Functions

## Azure Monitor Query Functions

These functions are considered *preview* as of August 2018. The names, signatures, and behavior of these functions might change as they are tested in real word usage.

The Azure Monitor datasource queries Azure for metric and resource information. These functions are available when [AzureMonitorConf](#system-configuration#azuremonitorconf) is defined in the system configuration. 

These requests are subject to the [Azure Resource Manager Request Limits](https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-manager-request-limits) so when using the `az` and `azmulti` functions you should be mindful of how many API calls your alerts are making given your configured check interval. Also using the historical testing feature to query multiple intervals of time could quickly eat through your request limit.

Currently there is no special treatment or instrumentation of the rate limit by Bosun, other then errors are expected once the rate limit is hit and warning will be logged when a request responses with less than 100 reads remaining.

### PrefixKey

PrefixKey is a quoted string used to query Azure with different clients from a single instance of Bosun. It can be passed as a prefix to Azure query functions as in the example below. If there is no prefix used then the query will be made on default Azure client.

```
$resources = ["foo"]azrt("Microsoft.Compute/virtualMachines")
$filteresRes = azrf($resources, "client:.*")
["foo"]azmulti("Percentage CPU", "", $resources, "max", "5m", "1h", "")
```

### az(namespace string, metric string, tagKeysCSV string, rsg string, resName string, agType string, interval string, startDuration string, endDuration string) seriesSet
{: .exprFunc}

az queries the [Azure Monitor REST API](https://docs.microsoft.com/en-us/rest/api/monitor/) for time series data for a specific metric and resource. Responses will include at least to tags: `name=<resourceName>,rsg=<resourceGroupName>`. If the metric support multiple dimensions and tagKeysCSV is non-empty additional tag keys are added to the response.

 * `namespace` is the Azure namespace that the metric lives under. [Supported metric with Azure montior](https://docs.microsoft.com/en-us/azure/monitoring-and-diagnostics/monitoring-supported-metrics) contains a list of those namespaces, for example `Microsoft.Cache/redis` and `Microsoft.Compute/virtualMachines`.
 * `metric` is the name of the metric under the corresponding `namespace` that you want to query, for example `Percentage CPU`.
 * `tagKeysCSV` is comma-separated list of dimension keys that you want the response to group by. For example, the `Per Disk Read Bytes/sec` metric under `Microsoft.Compute/virtualMachines` has a SlotId metric, so if you pass `"SlotId"` for this argument `SlotId` will become a tag key in the response with the values corresponding to each slot (i.e `0`)
 * `rsg` is the name of the Azure resource group that the resource is in
 * `resName` is the name of the resource
 * `agType` is the type of aggregation to use can be `avg`, `min`, `max`, `total`, or `count`. If an empty string then the default is `avg`.
 * `interval` is the Azure timegrain to use without "PT" and in lower case (ISO 8601 duration format). Common supported timegrains are `1m`, `5m`, `15m`, `30m`, `1h`, `6h`, `12h`, and `1d`. 
 * `startDuration` and `endDuration` set the time window from now - see the OpenTSDB q() function for more details

 Examples:

 `az("Microsoft.Compute/virtualMachines", "Percentage CPU", "", "myResourceGroup", "myFavoriteVM", "avg", "5m", "1h", "")`

 `az("Microsoft.Compute/virtualMachines", "Per Disk Read Bytes/sec", "SlotId", "myResourceGroup", "myFavoriteVM", "max", "5m", "1h", "")`

### azrt(type string) azureResources
{: .exprFunc}

azrt (Azure Resources By Type) gets a list of Azure Resources that exist for a certain type. For example, `azrt("Microsoft.Compute/virtualMachines")` would return all virtualMachine resources. This list of resources can then be passed to `azrf()` (Azure Resource Filter) for additional filtering or to a query function that takes AzureResources as an argument like `azmulti()`.

An error will be returned if you attempt to pass resources fetched for an Azure client with a different client.  In other words, if the resources call (e.g. `azrt()`) uses a different prefix from the time series query (e.g. `azmulti()`)).

The underlying implementation of this fetches *all* resources and caches that information. So additional azrt calls within scheduled check cycle will not result in additional calls to Azure's API.

### azrf(resources azureResources, filter string) azureResources
{: .exprFunc}

azrf (Azure Resource Filter) takes a resource list and filters it to less resources based on the filter. The resources argument would usually be an `azrt()` call or another `azrf` call.

The filter argument supports filter supports joining terms in `()` as well as the `AND`, `OR`, and `!` operators. The following query terms are supported and are always in the format of something:something. The first part of each term (the key) is case insensitive.

 * `name:<regex>` where the resource name matches the regular expression. 
 * `rsg:<regex>` where the resource group of the resource matches the resource.
 * `otherText:<regex>` will match resources based on Azure tags. `otherText` would be the tag key and the regex will match against the tag's value. If the tag key does not exist on the resource then there will be no match.

Regular expressions use Go's regular expressions which use the [RE2 syntax](https://github.com/google/re2/wiki/Syntax). If you want an exact match and not a substring be sure to anchor the term with something like `rsg:^myRSG$`.

Example:

```
$resources = azrt("Microsoft.Compute/virtualMachines")
# Filter resources to those with client azure tag that has any value
$filteresRes = azrf($resources, "client:.*")
azmulti("Percentage CPU", "", $filteredRes, "max", "5m", "1h", "")
```

Note that `azrf()` does not take a prefix key since it is filtering resources that have already been retrieved. The resulting azureResources will still be associated with the correct client/prefix.

### azmulti(metric string, tagKeysCSV string, resources AzureResources, agType string, interval string, startDuration string, endDuration string) seriesSet

azmulti (Azure Multiple Query) queries a metric for multiple resources and returns them as a single series set. The arguments metric, tagKeysCSV, agType, interval, startDuration, and endDuration all behave the same as in the `az` function. Also like the `az` functions the result will be tagged with `rsg`, `name`, and any dimensions from tagKeysCSV.

The resources argument is a list of resources (an azureResourcesType) as returned by `azrt` and `azrf`. 

Each resource queried requires an Azure Monitor API call. So if there are 20 items in the set from return of the call, 20 calls are made that count toward the rate limit. This function exists because most metrics do not have dimensions on primary attributes like the machine name.

Example: 

```
$resources = azrt("Microsoft.Compute/virtualMachines")
azmulti("Percentage CPU", "", $resources, "max", "PT5M", "1h", "")
```

## Graphite Query Functions

### graphite(query string, startDuration string, endDuration string, format string) seriesSet
{: .exprFunc}

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

### graphiteBand(query string, duration string, period string, format string, num string) seriesSet
{: .exprFunc}

Like band() but for graphite queries.

## InfluxDB Query Functions

### influx(db string, query string, startDuration string, endDuration, groupByInterval string) seriesSet
{: .exprFunc}

Queries InfluxDB.

All tags returned by InfluxDB will be included in the results.

* `db` is the database name in InfluxDB
* `query` is an InfluxDB select statement
    NB: WHERE clauses for `time` are inserted automatically, and it is thus an error to specify `time` conditions in query.
* `startDuration` and `endDuration` set the time window from now - see the OpenTSDB q() function for more details
    They will be merged into the existing WHERE clause in the `query`.
* `groupByInterval` is the `time.Duration` window which will be passed as an argument to a GROUP BY time() clause if given. This groups values in the given time buckets. This groups (or in OpenTSDB lingo "downsamples") the results to this timeframe. [Full documentation on Group by](https://influxdb.com/docs/v0.9/query_language/data_exploration.html#group-by).

### Notes:

  * By default, queries will be given a suffix of `fill(none)` to filter out any nil rows.
  * Influx queries themselves often use both double and single (quoting issues are often encountered [as per the documentation](https://docs.influxdata.com/influxdb/v0.13/troubleshooting/frequently_encountered_issues/#single-quoting-and-double-quoting-in-queries)). So you will likely need to use triple single quotes (`'''`) for many queries. When using single quotes in triple single quotes, you may need a space. So for example `'''select max(value) from "my.measurement" where key = 'val''''` is not valid but `'''select max(value) from "my.measurement" where key = 'val' '''` is.

### examples:

These influx and opentsdb queries should give roughly the same results:

```
influx("db", '''SELECT non_negative_derivative(mean(value)) FROM "os.cpu" GROUP BY host''', "30m", "", "2m")

q("sum:2m-avg:rate{counter,,1}:os.cpu{host=*}", "30m", "")
```

Querying graphite sent to influx (note the quoting):

```
influx("graphite", '''select sum(value) from "df-root_df_complex-free" where env='prod' and node='web' ''', "2h", "1m", "1m")
```

## Elastic Query Functions

Elasitc replaces the deprecated logstash (ls) functions. It only works with Elastic v2+. It is meant to be able to work with any elastic documents that have a time field and not just logstash. It introduces two new types to allow for greater flexibility in querying. The ESIndexer type generates index names to query (based on the date range). There are now different functions to generate indexers for people with different configurations. The ESQuery type is generates elastic queries so you can filter your results. By making these new types, new Indexers and Elastic queries can be added over time.

You can view the generated JSON for queries on the expr page by bring up miniprofiler with Alt-P.

### PrefixKey
PrefixKey is a quoted string used to query different elastic cluster and can be passed as a prefix to elastic query functions mentioned below. If not used the query will be made on [default](system_configuration#elasticconfdefault) cluster.

Querying [foo](system_configuration#example-2) cluster:

```
$index = esindices("timestamp", "errors")
$filter = esquery("nginx", "POST")
crit = max(["foo"]escount($index, "host", $filter, "1h", "30m", "")) > 2 
```

### escount(indexRoot ESIndexer, keyString string, filter ESQuery, bucketDuration string, startDuration string, endDuration string) seriesSet
{: .exprFunc}

escount returns a time bucked count of matching documents. It uses the keystring, indexRoot, interval, and durations to create an [elastic Date Histogram Aggregation](https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations-bucket-datehistogram-aggregation.html).

  * `indexIndexer` will always be a function that returns an ESIndexer, such as `esdaily`.
  * `keyString` is a csv separated list of fields. The fields will become tag keys, and the values returned for fields become the correspond tag values. For example `host,errorCode`. If an empty string is given, then the result set will have a single series and will have an empty tagset `{}`. These keys become terms filters for the date histogram.
  * `filter` will be a funtion that returns an ESQuery. The queries further refine the results. The fields you filter on can match the fields in the keyString, but don't have too. If you don't want to filter you results, use `esall()` here.
  * `bucketDuration` is an opentsdb duration string. It sets the the span of time to bucket the count of documents. For example, "1m" will give you the count of documents per minute.
  * `startDuration` and `endDuration` set the time window from now - see the OpenTSDB q() function for more details.

### esstat(indexRoot ESIndexer, keyString string, filter ESQuery, field string, rStat string, bucketDuration string, startDuration string, endDuration string) seriesSet
{: .exprFunc}

estat returns various summary stats per bucket for the specified `field`. The field must be numeric in elastic. rStat can be one of `avg`, `min`, `max`, `sum`, `sum_of_squares`, `variance`, `std_deviation`. The rest of the fields behave the same as escount.

## Elastic Index Functions

### esdaily (timeField string, indexRoot string, layout string) ESIndexer
{: .exprFunc}

esdaily is for elastic indexes that have a date name for each day. Based on the timeframe of the enclosing es function (i.e. esstat and escount) to generate which indexes should be included in the query. It gets all indexes and won't include indices that don't exist. The layout specifier uses's [Go's time specification format](https://golang.org/pkg/time/#Parse). The timeField is the name of the field in elastic that contains timestamps for the documents.

### esmonthly (timeField string, indexRoot string, layout string) ESIndexer
{: .exprFunc}

esmonthly is like esdaily except that it is for monthly indices. It is expect the index name is the first of every month.

### esindices(timeField string, index string...) ESIndexer
{: .exprFunc}

esindices takes one or more literal indices for the enclosing query to use. It does not check for existance of the index, and passes back the elastic error if the index does not exist. The timeField is the name of the field in elastic that contains timestamps for the documents.

### esls(indexRoot string) ESIndexer
{: .exprFunc}

esls is a shortcut for esdaily("@timestamp", indexRoot+"-", "2006.01.02") and is for the default daily format that logstash creates.

## Elastic Query Generating Functions (for filtering)

### esall() ESQuery
{: .exprFunc}

esall returns an elastic matchall query, use this when you don't want to filter any documents.

### esregexp(field string, regexp string)
{: .exprFunc}

esregexp creates an [elastic regexp query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-regexp-query.html) for the specified field.

### esquery(field string, querystring string)
{: .exprFunc}

esquery creates a [full-text elastic query string query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-query-string-query.html).

### esand(queries.. ESQuery) ESQuery
{: .exprFunc}

esand takes one or more ESQueries and combines them into an [elastic bool query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-bool-query.html) where all the queries "must" be true.

### esor(queries.. ESQuery) ESQuery
{: .exprFunc}

esor takes one or more ESQueries and combines them into an [elastic bool query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-bool-query.html) so that at least one must be true.

### esnot(query ESQuery) ESQuery
{: .exprFunc}

esnot takes a query and inverses the logic using must_not from an [elastic bool query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-bool-query.html).

### esexists(field string) ESQuery
{: .exprFunc}

esexists is true when the specified field exists.

###esgt(field string, value Scalar) ESQuery
{: .exprFunc}

esgt takes a field (expected to be numeric field in elastic) and returns results where the value of that field is greater than the specified value. It creates an [elastic range query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-range-query.html).

### esgte(field string, value Scalar) ESQuery
{: .exprFunc}

esgt takes a field (expected to be numeric field in elastic) and returns results where the value of that field is greater than or equal to the specified value. It creates an [elastic range query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-range-query.html).

### eslt(field string, value Scalar) ESQuery
{: .exprFunc}

esgt takes a field (expected to be numeric field in elastic) and returns results where the value of that field is less than the specified value. It creates an [elastic range query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-range-query.html).

### eslte(field string, value Scalar) ESQuery
{: .exprFunc}

esgt takes a field (expected to be numeric field in elastic) and returns results where the value of that field is less than or equal to the specified value. It creates an [elastic range query](https://www.elastic.co/guide/en/elasticsearch/reference/2.4/query-dsl-range-query.html).


## OpenTSDB Query Functions

Query functions take a query string (like `sum:os.cpu{host=*}`) and return a seriesSet.

### q(query string, startDuration string, endDuration string) seriesSet
{: .exprFunc}

Generic query from endDuration to startDuration ago. If endDuration is the empty string (`""`), now is used. Support d( units are listed in [the docs](http://opentsdb.net/docs/build/html/user_guide/query/dates.html). Refer to [the docs](http://opentsdb.net/docs/build/html/user_guide/query/index.html) for query syntax. The query argument is the value part of the `m=...` expressions. `*` and `|` are fully supported. In addition, queries like `sys.cpu.user{host=ny-*}` are supported. These are performed by an additional step which determines valid matches, and replaces `ny-*` with `ny-web01|ny-web02|...|ny-web10` to achieve the same result. This lookup is kept in memory by the system and does not incur any additional OpenTSDB API requests, but does require scollector instances pointed to the bosun server.

### band(query string, duration string, period string, num scalar) seriesSet
{: .exprFunc}

Band performs `num` queries of `duration` each, `period` apart and concatenates them together, starting `period` ago. So `band("avg:os.cpu", "1h", "1d", 7)` will return a series comprising of the given metric from 1d to 1d-1h-ago, 2d to 2d-1h-ago, etc, until 8d. This is a good way to get a time block from a certain hour of a day or certain day of a week over a long time period.

### over(query string, duration string, period string, num scalar) seriesSet
{: .exprFunc}

Over's arguments behave the same way as band. However over shifts the time of previous periods to be now, tags them with duration that each period was shifted, and merges those shifted periods into a single seriesSet. This is useful for displaying time over time graphs. For example, the same day week over week would be `over("avg:1h-avg:rate:os.cpu{host=ny-bosun01}", "1d", "1w", 4)`.

### change(query string, startDuration string, endDuration string) numberSet
{: .exprFunc}

Change is a way to determine the change of a query from startDuration to endDuration. If endDuration is the empty string (`""`), now is used. The query must either be a rate or a counter converted to a rate with the `agg:rate:metric` flag.

For example, assume you have a metric `net.bytes` that records the number of bytes that have been sent on some interface since boot. We could just subtract the end number from the start number, but if a reboot or counter rollover occurred during that time our result will be incorrect. Instead, we ask OpenTSDB to convert our metric to a rate and handle all of that for us. So, to get the number of bytes in the last hour, we could use:

`change("avg:rate:net.bytes", "60m", "")`

Note that this is implemented using the bosun's `avg` function. The following is exactly the same as the above example:

`avg(q("avg:rate:net.bytes", "60m", "")) * 60 * 60`

### count(query string, startDuration string, endDuration string) scalar
{: .exprFunc}

Count returns the number of groups in the query as an ungrouped scalar.

### window(query string, duration string, period string, num scalar, funcName string) seriesSet
{: .exprFunc}

Window performs `num` queries of `duration` each, `period` apart, starting
`period` ago. The results of the queries are run through `funcName` which
must be a reduction function taking only one argument (that is, a function
that takes a series and returns a number), then a series made from those. So
`window("avg:os.cpu{host=*}", "1h", "1d", 7, "dev")` will return a series
comprising of the average of given metric from 1d to 1d-1h-ago, 2d to
2d-1h-ago, etc, until 8d. It is similar to the band function, except that
instead of concatenating series together, each series is reduced to a number,
and those numbers created into a series.

In addition to supporting Bosun's reduction functions that take on argument, percentile operations may be be done by setting `funcName` to p followed by number that is between 0 and 1 (inclusively). For example, `"p.25"` will be the 25th percentile, `"p.999"` will be the 99.9th percentile. `"p0"` and `"p1"` are min and max respectively (However, in these cases it is recommended to use `"min"` and `"max"` for the sake of clarity.

# Annotation Query Functions
These function are available when annotate is enabled via Bosun's configuration.

## Annotation Filters
For the following annotation functions, `filter` is a string with the following specification.

Items in a filter are in the format `keyword:value`. The value is either a glob pattern or literal string to match, or the reserved word `empty` which means that the value of the field is an empty string.

Possible keywords are: `owner`, `user`, `host`, `category`, `url`, and `message`. 

All items can be combined in boolean logic by using paranthesis groupging, `!` as not, `AND` as logical and, and `OR` as logical or.

For example, `"owner:sre AND ! user:empty"` would show things that belong to sre, and have a username specified. When annotations are created by a process, we don't specify a user.

## antable(filter string, fieldsCSV string, startDuration, endDuration) Table
Antable is meant for shoowing annotations in a Grafana table, where Grafana's "To Table Transform" under options is set to type "Table".

See Annotation Filters above to understand filters. FieldsCSV is a list of columns to display in the table. They can be in any order. The possible columns you can include are: `start`, `end`, `owner`, `user`, `host`, `category`, `url`, `link` `message`, `duration`. At least one column must be specified.

`link` is unlike the others in that it actually returns the HTML to construct a link, whereas `url` is the the text of the link. This is so when using a Grafana table and Grafana v3.1.1 or later, you can have a link in a table as long as you enable sanitize HTML within the Grafana Column Styles.

For example: `antable("owner:sre AND category:outage", "start,end,user,owner,category,message", "8w", "")` will return a table of annotations with the selected columns in FieldCSV going back 8 weeks from the time of the query.

## ancounts(filter string, startDuration string, endDuration string) seriesSet
{: .exprFunc}
ancounts returns a series representing the number of annotations that matched the filter for the specified period. One might expect a number instead of a series, but by having a series it has a useful property. We can count outages that span'd across the requested time frame and count them as fractional outages.

If an annotation's timespan is contained entirely within the request timespan, or the timespan of the request is within the the timespan of the annotation, a 1 is added to the series.

If an annotation either starts before the requested start time, or ends after the requested start time then it is counted as a fractional outage (Assuming the annotation ended or started respectively with the requested time frame).

If there are no annotations within the requested time period, then the value `NaN` will be returned.

For example:

The following request is made at `2016-09-21 14:49:00`.

```
$filter = "owner:sre AND category:outage"
$back = "1n"
$count = ancounts($filter, $back, "")
# TimeFrame of the Fractional annotation: "2016-09-21T14:47:56Z", "2016-09-21T14:50:53Z" (Duration: 2m56 sec)
$count
```

Returns:
```
{
  "0": 1,
  "1": 1,
  "2": 0.3615819209039548
}
```

The float values means that 36% of the annotation fell with the requested time frame. Once can get the sum of these by doing `sum($count)`, result of `2.36...` to get the fractional sum, or `len($count)`, result `3` to get the count.

Note: The index values above, 0, 1, and 2 are disregarded and are just there so we can use the same underlying type as a time series.


## andurations(filter string, startDuration, endDuration string) seriesSet
{: .exprFunc}

andurations behaves in a similiar way to ancounts. The difference is that the values you returned will be the duration of annotation in seconds. 

If the duration spans part of the requested time frame, only the duration of the annotation that falls within the timerange will be returns as a value for that annotation. If the annotation starts before the request and ends after the request, the duration of the request timeframe will be returned.

If there are no annotations within the requested time period, then the value `NaN` will be returned.

For example, a identical query to the example in ancounts but using andurations instead:

```
$filter = "owner:sre AND category:outage"
$back = "1n"
$durations = andurations($filter, $back, "")
# TimeFrame of the Fractional Outage: "2016-09-21T14:47:56Z", "2016-09-21T14:50:53Z",
$durations
```

Returns:

```
{
  "0": 402,
  "1": 758,
  "2": 64
}
```

# Reduction Functions

All reduction functions take a seriesSet and return a numberSet with one element per unique group.

## avg(seriesSet) numberSet
{: .exprFunc}

Average (arithmetic mean).

## cCount(seriesSet) numberSet
{: .exprFunc}

Returns the change count which is the number of times in the series a value was not equal to the immediate previous value. Useful for checking if things that should be at a steady value are "flapping". For example, a series with values [0, 1, 0, 1] would return 3.

## dev(seriesSet) numberSet
{: .exprFunc}

Standard deviation.

## diff(seriesSet) numberSet
{: .exprFunc}

Diff returns the last point of each series minus the first point.

## first(seriesSet) numberSet
{: .exprFunc}

Returns the first (least recent) data point in each series.

## forecastlr(seriesSet, y_val numberSet|scalar) numberSet
{: .exprFunc}

Returns the number of seconds until a linear regression of each series will reach y_val.

## linelr(seriesSet, d Duration) seriesSet
{: .exprFunc}


Linelr return the linear regression line from the end of each series to end+duration (an [OpenTSDB duration string](http://opentsdb.net/docs/build/html/user_guide/query/dates.html)). It adds `regression=line` to the group / tagset. It is meant for graphing with expressions, for example:

```
$d = "1w"
$q = q("avg:1h-avg:os.disk.fs.percent_free{}{host=ny-tsdb*,disk=/mnt*}", "2w", "")
$line = linelr($q, "3n")
$m = merge($q, $line)
$m
```

## last(seriesSet) numberSet
{: .exprFunc}

Returns the last (most recent) data point in each series.

## len(seriesSet) numberSet
{: .exprFunc}

Returns the length of each series.

## max(seriesSet) numberSet
{: .exprFunc}

Returns the maximum value of each series, same as calling percentile(series, 1).

## median(seriesSet) numberSet
{: .exprFunc}

Returns the median value of each series, same as calling percentile(series, .5).

## min(seriesSet) numberSet
{: .exprFunc}

Returns the minimum value of each series, same as calling percentile(series, 0).

## percentile(seriesSet, p numberSet|scalar) numberSet
{: .exprFunc}

Returns the value from each series at the percentile p. Min and Max can be simulated using `p <= 0` and `p >= 1`, respectively.

## since(seriesSet) numberSet
{: .exprFunc}

Returns the number of seconds since the most recent data point in each series.

## streak(seriesSet) numberSet
{: .exprFunc}

Returns the length of the longest streak of values that evaluate to true (i.e. max amount of contiguous non-zero values found).

## sum(seriesSet) numberSet
{: .exprFunc}

Sum.

# Aggregation Functions

Aggregation functions take a seriesSet, and return a new seriesSet.

## aggr(series seriesSet, groups string, aggregator string) seriesSet
{: .exprFunc}

Takes a seriesSet and combines it into a new seriesSet with the groups specified, using an aggregator to merge any series that share the matching group values. If the groups argument is an empty string, all series are combined into a single series, regardless of existing groups. 

The available aggregator functions are: `"avg"` (average), `"min"` (minimum), `"max"` (maximum), `"sum"` and `"pN"` (percentile) where N is a floating point number between 0 and 1 inclusive. For example, `"p.25"` will be the 25th percentile, `"p.999"` will be the 99.9th percentile. `"p0"` and `"p1"` are min and max respectively (However, in these cases it is recommended to use `"min"` and `"max"` for the sake of clarity.

The aggr function can be particularly useful for removing anomalies when comparing timeseries over periods using the over function. 

Example:

```
$weeks = over("sum:1m-avg:os.cpu{region=*,color=*}", "24h", "1w", 3)
$agg = aggr($weeks, "region,color", "p.50")
```

The above example uses `over` to load a 24 hour period over the past 3 weeks. We then use the aggr function to combine the three weeks into one, selecting the median (`p.50`) value of the 3 weeks at each timestamp. This results in a new seriesSet, grouped by region and color, that represents a "normal" 24 hour period with anomalies removed.

An error will be returned if a group is specified to aggregate on that does not exist in the original seriesSet.

The aggr function expects points in the original series to be aligned by timestamp. If points are not aligned, they are aggregated separately. For example, if we had a seriesSet,

Group       | Timestamp | Value |
----------- | --------- | ----- |
{host=web01} | 1 | 1 |
{host=web01} | 2 | 7 |
{host=web01} | 1 | 4 |

and applied the following aggregation:

```
aggr($series, "host", "max")
```

we would receive the following aggregated result:

Group       | Timestamp | Value | Timestamp | Value |
----------- | --------- | ----- | --------- | ----- |
{host=web01} | 1 | 4 | 2 | 7 |

aggr also does not attempt to deal with NaN values in a consistent manner. If all values for a specific timestamp are NaN, the result for that timestamp will be NaN. If a particular timestamp has a mix of NaN and non-NaN values, the result may or may not be NaN, depending on the aggregation function specified.

# Group Functions

Group functions modify the OpenTSDB groups.

## addtags(set variantSet, group string) (seriesSet|numberSet)
{: .exprFunc}

Accepts a series and a set of tags to add to the set in `Key1=NewK1,Key2=NewK2` format. This is useful when you want to add series to set with merge and have tag collisions.

## rename(variantSet, string) (seriesSet|numberSet)
{: .exprFunc}

Accepts a series and a set of tags to rename in `Key1=NewK1,Key2=NewK2` format. All data points will have the tag keys renamed according to the spec provided, in order. This can be useful for combining results from seperate queries that have similar tagsets with different tag keys.

## remove(variantSet, string) (seriesSet|numberSet)
{: .exprFunc}

Accepts a tag key to remove from the set. The function will error if removing the tag key from the set would cause the resulting set to have a duplicate item in it.

## t(numberSet, group string) seriesSet
{: .exprFunc}

Transposes N series of length 1 to 1 series of length N. If the group parameter is not the empty string, the number of series returned is equal to the number of tagks passed. This is useful for performing scalar aggregation across multiple results from a query. For example, to get the total memory used on the web tier: `sum(t(avg(q("avg:os.mem.used{host=*-web*}", "5m", "")), ""))`. See [Understanding the Transpose Function](/t) for more explanation.

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

```
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
    warnNotification = $percent_down > 25
  }
```

Since our templates can reference any variable in this alert, we can show which servers are down in the notification, even though the alert just triggers on 25% of or-\* servers being down.

## ungroup(numberSet) scalar
{: .exprFunc}

Returns the input with its group removed. Used to combine queries from two differing groups.

# Other Functions

## alert(name string, key string) numberSet
{: .exprFunc}

Executes and returns the `key` expression from alert `name` (which must be
`warn` or `crit`). Any alert of the same name that is unknown or unevaluated
is also returned with a value of `1`. Primarily for use with the [`depends` alert keyword](/definitions#depends).

Example: `alert("host.down", "crit")` returns the crit
expression from the host.down alert.

## abs(variantSet) (seriesSet|numberSet)
{: .exprFunc}

Returns the absolute value of each value in the set.

## crop(series seriesSet, start numberSet, end numberSet) seriesSet
{: .exprFunc}

Returns a seriesSet where each series is has datapoints removed if the datapoint is before start (from now, in seconds) or after end (also from now, in seconds). This is useful if you want to alert on different timespans for different items in a set, for example:

```
lookup test {
    entry host=ny-bosun01 {
        start = 30
    }
    entry host=* {
        start = 60
    }
}

alert test {
    template = test
    $q = q("avg:rate:os.cpu{host=ny-bosun*}", "5m", "")
    $c = crop($q, lookup("test", "start") , 0)
    crit = avg($c)
}
```

## d(string) scalar
{: .exprFunc}

Returns the number of seconds of the [OpenTSDB duration string](http://opentsdb.net/docs/build/html/user_guide/query/dates.html).

## tod(scalar) string
{: .exprFunc}

Returns an [OpenTSDB duration string](http://opentsdb.net/docs/build/html/user_guide/query/dates.html) that represents the given number of seconds. This lets you do math on durations and then pass it to the duration arguments in functions like `q()`

## des(series, alpha scalar, beta scalar) series
{: .exprFunc}

Returns series smoothed using Holt-Winters double exponential smoothing. Alpha
(scalar) is the data smoothing factor. Beta (scalar) is the trend smoothing
factor.

## dropg(seriesSet, threshold numberSet|scalar) seriesSet
{: .exprFunc}

Remove any values greater than number from a series. Will error if this operation results in an empty series.

## dropge(seriesSet, threshold numberSet|scalar) seriesSet
{: .exprFunc}

Remove any values greater than or equal to number from a series. Will error if this operation results in an empty series.

## dropl(seriesSet, threshold numberSet|scalar) seriesSet
{: .exprFunc}

Remove any values lower than number from a series. Will error if this operation results in an empty series.

## drople(seriesSet, threshold numberSet|scalar) seriesSet
{: .exprFunc}

Remove any values lower than or equal to number from a series. Will error if this operation results in an empty series.

## dropna(seriesSet) seriesSet
{: .exprFunc}

Remove any NaN or Inf values from a series. Will error if this operation results in an empty series.

## dropbool(seriesSet, seriesSet) seriesSet
{: .exprFunc}

Drop datapoints where the corresponding value in the second series set is non-zero. (See Series Operations for what corresponding means). The following example drops tr_avg (avg response time per bucket) datapoints if the count in that bucket was + or - 100 from the average count over the time period.

Example:

```
$count = q("sum:traffic.haproxy.route_tr_count{host=literal_or(ny-logsql01),route=Questions/Show}", "30m", "")
$avg = q("sum:traffic.haproxy.route_tr_avg{host=literal_or(ny-logsql01),route=Questions/Show}", "30m", "")
$avgCount = avg($count)
dropbool($avg, !($count < $avgCount-100 || $count > $avgCount+100))
```

## epoch() scalar
{: .exprFunc}

Returns the Unix epoch in seconds of the expression start time (scalar).

## filter(variantSet, numberSet) (seriesSet|numberSet)
{: .exprFunc}

Returns all results in variantSet that are a subset of numberSet and have a non-zero value. Useful with the limit and sort functions to return the top X results of a query.

## limit(set variantSet, count scalar) (seriesSet|numberSet)
{: .exprFunc}

Returns the first count (scalar) items of the set.

## lookup(table string, key string) numberSet
{: .exprFunc}

Returns the first key from the given lookup table with matching tags, this searches the built-in index and so only makes sense when using OpenTSDB and sending data to /index or relaying through bosun.

Using the lookup function will set [unJoinedOk](/definitions#unjoinedok) to true for the alert.

## lookupSeries(series seriesSet, table string, key string) numberSet
{: .exprFunc}

Returns the first key from the given lookup table with matching tags.
The first argument is a series to use from which to derive the tag information.  This is good for alternative storage backends such as graphite and influxdb.

Using the lookupSeries function will set [unJoinedOk](/definitions#unjoinedok) to true for the alert.

## map(series seriesSet, subExpr numberSetExpr) seriesSet
{: .exprFunc}

map applies the subExpr to each value in each series in the set. A special function `v()` which is only available in a numberSetExpr and it gives you the value for each item in the series.

For example you can do something like the following to get the absolute value for each item in the series (since the normal `abs()` function works on normal numbers, not series:

```
$q = q("avg:rate:os.cpu{host=*bosun*}", "5m", "")
map($q, expr(abs(v())))
```

Or for another example, this would get you the absolute difference of each datapoint from the series average as a new series:

```
$q = q("avg:rate:os.cpu{host=*bosun*}", "5m", "")
map($q, expr(abs(v()-avg($q))))
```

Since this function is not optimized for a particular operation on a seriesSet it may not be very efficent. If you find you are doing things that involve more complex expressions within the `expr(...)` inside map (for example, having query functions in there) than you may want to consider requesting a new function to be added to bosun's DSL.

## expr(expression)
{: .exprFunc}

expr takes an expression and returns either a numberSetExpr or a seriesSetExpr depending on the resulting type of the inner expression. This exists for functions like `map` - it is currently not valid in the expression language outside of function arguments.

## month(offset scalar, startEnd string) scalar
{: .exprFunc}

Returns the epoch of either the start or end of the month. Offset is the timezone offset from UTC that the month starts/ends at (but the returned epoch is representitive of UTC). startEnd must be either `"start"` or `"end"`. Useful for things like monthly billing, for example:

```
$hostInt = host=ny-nexus01,iname=Ethernet1/46
$inMetric = "sum:5m-avg:rate{counter,,1}:__ny-nexus01.os.net.bytes{$hostInt,direction=in}"
$outMetric = "sum:5m-avg:rate{counter,,1}:__ny-nexus01.os.net.bytes{$hostInt,direction=in}"
$commit = 100
$monthStart = month(-4, "start")
$monthEnd = month(-4, "end")
$monthLength = $monthEnd - $monthStart
$burstTime = ($monthLength)*.05
$burstableObservations = $burstTime / d("5m")
$in = q($inMetric, tod(epoch()-$monthStart), "") * 8 / 1e6
$out = q($inMetric, tod(epoch()-$monthStart), "") * 8 / 1e6
$inOverCount = sum($in > $commit)
$outOverCount = sum($out > $commit)
$inOverCount > $burstableObservations || $outOverCount > $burstableObservations
```

## series(tagset string, epoch, value, ...) seriesSet
{: .exprFunc}

Returns a seriesSet with one series. The series will have a group (a.k.a tagset). The tagset can be "" for the empty group, or in "key=value,key=value" format. You can then optionally pass epoch value pairs (if non are provided, the series will be empty). This is can be used for testing or drawing arbitary lines. For example:

```
$now = epoch()
$hourAgo =  $now-d("1h")
merge(series("foo=bar", $hourAgo, 5, $now, 10), series("foo=bar2", $hourAgo, 6, $now, 11))
```

## shift(seriesSet, dur string) seriesSet
{: .exprFunc}

Shift takes a seriesSet and shifts the time forward by the value of dur ([OpenTSDB duration string](http://opentsdb.net/docs/build/html/user_guide/query/dates.html)) and adds a tag for representing the shift duration. This is meant so you can overlay times visually in a graph.

## leftjoin(tagsCSV string, dataCSV string, ...numberSet) table
{: .exprFunc}

leftjoin takes multiple numberSets and joins them to the first numberSet to form a table. tagsCSV is a string that is comma delimited, and should match tags from query that you want to display (i.e., "host,disk"). dataCSV is a list of column names for each numberset, so it should have the same number of labels as there are numberSets.

The only current intended use case is for constructing "Table" panels in Grafana.

For Example, the following in Grafana would create a table that shows the CPU of each host for the current period, the cpu for the adjacent previous period, and the difference between them:

```
$cpuMetric = "avg:$ds-avg:rate{counter,,1}:os.cpu{host=*bosun*}{}"
$currentCPU = avg(q($cpuMetric, "$start", ""))
$span = (epoch() - (epoch() - d("$start")))
$previousCPU = avg(q($cpuMetric, tod($span*2), "$start"))
$delta = $currentCPU - $previousCPU
leftjoin("host", "Current CPU,Previous CPU,Change", $currentCPU, $previousCPU, $delta)
```

Note that in the above example is intended to be used in Grafana via the Bosun datasource, so `$start` and `$ds` are replaced by Grafana before the query is sent to Bosun.

## merge(SeriesSet...) seriesSet
{: .exprFunc}

Merge takes multiple seriesSets and merges them into a single seriesSet. The function will error if any of the tag sets (groups) are identical. This is meant so you can display multiple seriesSets in a single expression graph.

## nv(numberSet, scalar) numberSet
{: .exprFunc}

Change the NaN value during binary operations (when joining two queries) of unknown groups to the scalar. This is useful to prevent unknown group and other errors from bubbling up.

## sort(numberSet, (asc|desc) string) numberSet
{: .exprFunc}

Returns the results sorted by value in ascending ("asc") or descending ("desc")
order. Results are first sorted by groupname and then stably sorted so that
results with identical values are always in the same order.

## timedelta(seriesSet) seriesSet
{: .exprFunc}

Returns the difference between successive timestamps in a series. For example:

```
timedelta(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1))
```

Would return a seriesSet equal to:

```
series("foo=bar", 1466133610, 10, 1466133710, 100)
```

## tail(seriesSet, num numberSet) seriesSet
{: .exprFunc}

Returns the most recent num points from a series. If the series is shorter than the number of requeted points the series is unchanged as all points are in the requested window. This function is useful for making calculating on the leading edge. For example:

```
tail(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1), 2)
```

Would return a seriesSet equal to:

```
series("foo=bar", 1466133610, 1, 1466133710, 1)
```

</div>
