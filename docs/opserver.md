---
layout: default
title: Opserver Integratoin
---

# Overview
Integration with Opserver is ongoing work. It works very well at Stack Overflow, but currently the way it integrates with Bosun requires some specific setup.

*Current* Requirements:

 * scollector
 * bosun
 * OpenTSDB
 * tsdbrelay

Over time we would like to reduce some of these requirements. However, if you would like to get started with Opserver, this following covers what needs to be done on the Bosun side.

# What Opserver Requests from Bosun and OpenTSDB

 * General host information from `bosun/api/host`
 * Available metrics from `bosun/api/metadata/metrics`
 * Time Series queries directly to OpenTSDB via `opentsdb/api/query`. However, it relies on information gathered by Bosun to know which queries to make

# How the host API constructs the data

The Host API in Bosun uses to main sources of data to populate it:

 * Metadata sent to Bosun via scollector (That Bosun stores in Redis or Ledis)
 * Datapoints sent via scollector that are then indexed by Bosun (Also stored in Redis / Ledis)

So the host API is expecting specific metadata and metrics to exist. scollector generates that data. It would be possible to do this with something other than scollector, but the metrics would have to line up, or we will need to introduce some configuration about sources. Nothing like that currently exists.

# tsdbrelay
In order to know about what metrics are available and their most recent values Bosun receives a copy of all datapoints and stores the most recent datapoint. You could relay though bosun, but tsdbrelay is an independent process that is less likely to have any downtime as compared to Bosun.

Some of the key metrics displayed in Bosun are things like CPU are slow to query for long periods of time due to the number of tags. In order to make these fast, we denormalize the metrics to have the host in the name. tsdbrelay does this denormalization, so it is currently a requirement. The configuration expected by Opserver for tsdbrelay is the following:

```
-denormalize=os.cpu__host,os.mem.used__host,os.net.bytes__host,os.net.bond.bytes__host,os.net.other.bytes__host,os.net.tunnel.bytes__host,os.net.virtual.bytes__host
```

In the future we will make this particular denormalization native to scollector, so this configuration isn't required.

# Conclusion

If you configure bosun with these requirements, you should be able to mimic or current Opserver integration and get some very nice dashboards. Since it is under active development this should be considered experimental, but it is quite functional if you are willing to be on the bleeding edge.



