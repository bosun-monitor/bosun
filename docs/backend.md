Features that work with certain setups

Bosun's full feature set is available with the following combination of services in use. However, it's core functionality is still available with other backends.
 * scollector
 * OpenTSDB 
 * Bosun indexing (generally via tsdbrelay)
 * Redis
 * Elastic (For Annotations)

However, people haven been successful using these other backends as their time series stores:
 * Graphite
 * InfluxDB

When it comes to the Elastic backend, it is primarily designed to allow you to query logs and get time series results about your logs (or the logs in notifications). The existing functions don't provide all the aggregations you might want to if using elastic as a pure time series store (as if it were graphite or OpenTSDB). So Elastic is more of a supplemental backend, however, you could still use Bosun with Elastic as your only backend. Pull Requests that would enhance elastic to be queried act like a fulltime series db.

Bosun does not currently support multiple backends *of the same type*. So you can have a Graphite, InfluxDB, and a OpenTSDB backend in a single Bosun instance. However, you can't have two OpenTSDB backends in a single Bosun instance at this time.

Our Agent scollector only sends data in OpenTSDB format currently. It does not write to Graphite or Elastic, so Bosun only reads from those data sources. However, influxdb can ingest metrics in the same format that the OpenTSDB API endpoint expects, Therefore you can use scollector with Bosun and Influx as the backend.

The following Feature only work with the requirements that follow the feature:
 * The Items Page: Bosun Indexing (Need link to what bosun indexing is here)
 * The Host Page: Bosun Indexing, scollector as the agent
 * The Graph Page: OpenTSDB datasource and Bosun indexing
 * Annotations: An elastic instance setup
 * Integration with Opserver: OpenTSDB, Bosun indexing, scollector, and tsdbrelay (with some specific settings)


The following should always work with any supported backend, a monitoring agent that works with your backend, and Bosun:
 * The expr page, and graphing on the expr page
 * Sending alerts and notifications
 * Notification templates
 * Historical testing of alerts and notification templates
 * and more

When it comes to redis / the built-in redis server "ledis", there should be no feature difference. However, we recommend redis for production deployments from an oeprations and performance perspective. 

So essentially the most critical features of Bosun will work with with alternate setups that he "fully supported" setup. 

