Features that work with certain setups

Bosun's full feature set is available with the following combination of services in use. However,it's core fuctionality is still available with other backends:
 * scollector
 * OpenTSDB 
 * Bosun indexing (generally via tsdbrelay)
 * Redis
 * Elastic (For Annotations)

However, people haven been successful using these other backends as their time series stores:
 * Graphite
 * InfluxDB

When it comes to the Elastic backend, it is primarily designed to allow you to query logs and get time series results about your logs (or the logs in notifications). The existing functions don't provide all the aggregations you might want to if using elastic as a pure time series store (like graphite or OpenTSDB). So Elastic is more of a supplemental backend, however, you could still use bosun with Elastic as your only backend. Pull Requests that would enhance elastic to be queried act like a full time series db.

Bosun does not currently support multiple backends *of the same type*. So you can have a Graphite, InfluxDB, and a OpenTSDB backend in a single Bosun instance. However, you can't have two OpenTSDB backends in a single Bosun instance at this time.

Our Agent scollector only sends data in OpenTSDB format currently. It does not write to Graphite or Elastic, so Bosun only reads from those datasources.  





| InUse / Feature        | Items Page | Host View  |
| -----------------------|------------| ---------- |
| OpenTSDB with Indexing | Yes        | ... |
| InfluxDB               |            |   ... |
| Scollector             | ...      |    ... |



Things you may be using or not using with Bosun: 
 * scollector
 * OpenTSDB
 * Influx
 * Elastic (Querying)
 * Elastic (Annotations) 
 * Graphite
 * Bosun Indexing Metrics (tsdbrelay, or bosun as relay, something else hitting index with opentsdb like formats i.e. tcollector))
 * Redis *or* ledis

Features that may work depending on combination of above:
 - The Graph Page
 - Items Listing
 - Grafana Plugin:
     - QueryHelper (expressions, incidents still work)
	 - Annotations
	 - 
 - Host View
 - Integration with OpServer 
 - Annotations

