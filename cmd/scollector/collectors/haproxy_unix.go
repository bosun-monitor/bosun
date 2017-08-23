package collectors

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, h := range c.HAProxy {
			for _, i := range h.Instances {
				ii := i
				collectors = append(collectors, &IntervalCollector{
					F: func() (opentsdb.MultiDataPoint, error) {

						return haproxyFetch(h.User, h.Password, ii.Tier, ii.URL)
					},
					name: fmt.Sprintf("haproxy-%s-%s", ii.Tier, ii.URL),
				})
			}
		}
	})
}

func haproxyFetch(user, pwd, tier, url string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var err error
	const metric = "haproxy"
	parse := func(v string) (int64, error) {
		var i int64
		if v != "" {
			i, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, err
			}
			return i, nil
		}
		return i, nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Close connection after request. Default cached connections will get
	// failures in the event of server closing idle connections.
	// See https://github.com/golang/go/issues/8946
	req.Close = true
	req.SetBasicAuth(user, pwd)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}
	// can't rely on number of colums with new (>=1.7) haproxy versions, lets check if there any data
	if len(records[1]) < 16 {
		return nil, fmt.Errorf("expected more columns with data. got: %v", len(records[1]))
	}
	header := parseHeader(records[0])
	for _, rec := range records[1:] {
		typePos, ok := header["type"]
		if !ok {
			return nil, fmt.Errorf("type not found in haproxy header")
		}
		pxnamePos, ok := header["pxname"]
		if !ok {
			return nil, fmt.Errorf("pxname not found in haproxy header")
		}
		svnamePos, ok := header["svname"]
		if !ok {
			return nil, fmt.Errorf("svname not found in haproxy header")
		}
		hType := haproxyType[rec[typePos]]
		pxname := rec[pxnamePos]
		svname := rec[svnamePos]
		ts := opentsdb.TagSet{"pxname": pxname, "svname": svname, "tier": tier}
		for _, field := range haproxyCSVMeta {
			fieldPos, ok := header[field.Name]
			if !ok {
				return nil, fmt.Errorf("%s not found in haproxy header", field.Name)
			}
			m := strings.Join([]string{metric, hType, field.Name}, ".")
			value := rec[fieldPos]
			if field.Ignore == true {
				continue
			} else if strings.HasPrefix(field.Name, "hrsp") {
				sp := strings.Split(field.Name, "_")
				if len(sp) != 2 {
					return nil, fmt.Errorf("unexpected field name %v in hrsp", field.Name)
				}
				ts := ts.Copy().Merge(opentsdb.TagSet{"status_code": sp[1]})
				m = strings.Join([]string{metric, hType, sp[0]}, ".")
				v, err := parse(value)
				if err != nil {
					return nil, err
				}
				Add(&md, m, v, ts, metadata.Counter, metadata.Response,
					fmt.Sprintf("The number of http responses with a %v status code.", sp[1]))
			} else if field.Name == "status" {
				v, ok := haproxyStatus[value]
				// Not distinging between MAINT and MAINT via...
				if !ok {
					v = 3
				}
				Add(&md, m, v, ts, field.RateType, field.Unit, field.Desc)
			} else if field.Name == "check_status" {
				if value == "" {
					continue
				}
				// A star is added if the check is in progress.
				value = strings.Trim(value, "* ")
				v, ok := haproxyCheckStatus[value]
				if !ok {
					return nil, fmt.Errorf("unknown check status %v", value)
				}
				Add(&md, m, v, ts, field.RateType, field.Unit, field.Desc)
			} else {
				v, err := parse(value)
				if err != nil {
					return nil, err
				}
				Add(&md, m, v, ts, field.RateType, field.Unit, field.Desc)
			}
		}
	}
	return md, nil
}

// Convert the first line of the CSV data into a map
// of column name -> column position.
func parseHeader(fields []string) map[string]int {
	header := map[string]int{}
	for i, name := range fields {
		if i == 0 {
			header[strings.TrimLeft(name, " #")] = i
		} else {
			header[name] = i
		}
	}
	return header
}

// MetricMetaHAProxy is a super-structure which adds a friendly Name,
// as well as an indicator on if a metric is to be ignored.
type MetricMetaHAProxy struct {
	Name   string
	Ignore bool
	MetricMeta
}

var haproxyType = map[string]string{
	"0": "frontend",
	"1": "backend",
	"2": "server",
	"3": "listen",
}

var haproxyCheckStatus = map[string]int{
	"UNK":     0,
	"INI":     1,
	"SOCKERR": 2,
	"L4OK":    3,
	"L4TOUT":  4,
	"L4CON":   5,
	"L6OK":    6,
	"L6TOUT":  7,
	"L6RSP":   8,
	"L7OK":    9,
	"L7OKC":   10,
	"L7TOUT":  11,
	"L7RSP":   12,
	"L7STS":   13,
}

var haproxyStatus = map[string]int{
	"UP":    0,
	"DOWN":  1,
	"NOLB":  2,
	"MAINT": 3,
}

// A slice of fields which are presented by haproxy's CSV data.
// See "CSV format" in http://www.haproxy.org/download/1.5/doc/configuration.txt
var haproxyCSVMeta = []MetricMetaHAProxy{
	{
		Name:   "pxname",
		Ignore: true,
	},
	{
		Name:   "svname",
		Ignore: true,
	},
	{
		Name: "qcur",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "The current queued requests. For the backend this reports the number queued without a server assigned.",
		}},
	{
		Name: "qmax",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "The max value of qcur.",
		}},
	{
		Name: "scur",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The current number of sessions.",
		}},
	{
		Name: "smax",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The maximum number of concurrent sessions seen.",
		}},
	{
		Name: "slim",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The configured session limit.",
		}},
	{
		Name: "stot",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Session,
			Desc: "The total number of sessions.",
		}},
	{
		Name: "bin",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of bytes in.",
		}},
	{
		Name: "bout",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of bytes out.",
		}},
	{
		Name: "dreq",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of requests denied because of security concerns. For tcp this is because of a matched tcp-request content rule. For http this is because of a matched http-request or tarpit rule.",
		}},
	{
		Name: "dresp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Response,
			Desc: "The number of responses denied because of security concerns. For http this is because of a matched http-request rule, or 'option checkcache'.",
		}},
	{
		Name: "ereq",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of request errors. Some of the possible causes are: Early termination from the client before the request has been sent, a read error from the client, a client timeout, a client closed connection, various bad requests from the client or the request was tarpitted.",
		}},
	{
		Name: "econ",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of number of requests that encountered an error trying to connect to a backend server. The backend stat is the sum of the stat for all servers of that backend, plus any connection errors not associated with a particular server (such as the backend having no active servers).",
		}},
	{
		Name: "eresp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Response,
			Desc: " The number of response errors. srv_abrt will be counted here also. Some errors are: write error on the client socket (won't be counted for the server stat) and failure applying filters to the response.",
		}},
	{
		Name: "wretr",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Retry,
			Desc: "The number of times a connection to a server was retried.",
		}},
	{
		Name: "wredis",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Redispatch,
			Desc: "number of times a request was redispatched to another server. The server value counts the number of times that server was switched away from.",
		}},
	{
		Name: "status",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Weight,
			Desc: "The current status: 0->UP, 1->Down, 2->NOLB, 3->Maintenance.",
		}},
	{
		Name: "weight",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Weight,
			Desc: "The server weight (server), total weight (backend).",
		}},
	{
		Name: "act",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Server,
			Desc: "If the server is active in the case of servers, or number of active servers in the case of a backend.",
		}},
	{
		Name: "bck",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Server,
			Desc: "If the server is a backup in the case of servers, or number of backup servers in the case of a backend.",
		}},
	{
		Name: "chkfail",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Check,
			Desc: "The number of failed checks. (Only counts checks failed when the server is up.)",
		}},
	{
		Name: "chkdown",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Transition,
			Desc: "The number of UP->DOWN transitions. The backend counter counts transitions to the whole backend being down, rather than the sum of the counters for each server.",
		}},
	{
		Name: "lastchg",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Second,
			Desc: "The number of seconds since the last UP<->DOWN transition.",
		}},
	{
		Name: "downtime",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Second,
			Desc: "The total downtime in seconds. The value for the backend is the downtime for the whole backend, not the sum of the server downtime.",
		}},
	{
		Name: "qlimit",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			//Don't know the unit
			Desc: "The configured maxqueue for the server, or nothing in the value is 0 (default, meaning no limit)",
		}},
	{
		Name:   "pid",
		Ignore: true,
		// Not a series or tag so skipping this.
	},
	{
		Name:   "iid",
		Ignore: true,
		// Not a series or tag so skipping this.
	},
	{
		Name:   "sid",
		Ignore: true,
		// Not a series or tag so skipping this.
	},
	{
		Name: "throttle",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Pct,
			Desc: "The current throttle percentage for the server, when slowstart is active, or no value if not in slowstart.",
		}},
	{
		Name: "lbtot",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			//Don't know the unit
			Desc: "The total number of times a server was selected, either for new sessions, or when re-dispatching. The server counter is the number of times that server was selected.",
		}},
	{
		Name:   "tracked",
		Ignore: true,
		// This could be a tag, but I am have no use for it.
	},
	{
		Name:   "type",
		Ignore: true,
		// This could be a tag, but I am have no use for it.
	},
	{
		Name:   "rate",
		Ignore: true,
		// This could be a tag, but I am have no use for it.
	},
	{
		Name: "rate_lim",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The configured limit on new sessions per second.",
		}},
	{
		Name: "rate_max",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Session,
			Desc: "The max number of new sessions per second.",
		}},
	{
		Name: "check_status",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "The status of last health check, one of: 0 -> unknown, 1 -> initializing, 2 -> socket error, 3 -> The check passed on layer 4, but no upper layers testing enabled, 4 -> layer 1-4 timeout, 5 -> layer 1-4 connection problem for example 'Connection refused' (tcp rst) or 'No route to host' (icmp), 6 -> check passed on layer 6, 7 -> layer 6 (SSL) timeout, 8 -> layer 6 invalid response - protocol error, 9 -> check passed on layer 7, 10 -> check conditionally passed on layer 7 for example 404 with disable-on-404, 11 -> layer 7 (HTTP/SMTP) timeout, 12 -> layer 7 invalid response - protocol error, 13 -> layer 7 response error, for example HTTP 5xx.",
		}},
	{
		Name: "check_code",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "The layer5-7 code, if available.",
		}},
	{
		Name: "check_duration",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The time in ms it took to finish last health check.",
		}},
	{
		Name: "hrsp_1xx",
		//These are transformed and aggregated: 1xx, 2xx, etc will be a tag.
	},
	{
		Name: "hrsp_2xx",
	},
	{
		Name: "hrsp_3xx",
	},
	{
		Name: "hrsp_4xx",
	},
	{
		Name: "hrsp_5xx",
	},
	{
		Name: "hrsp_other",
	},
	{
		Name: "hanafail",
		// The docs just say "failed health check details", so skipping this
		// for now
	},
	{
		Name: "req_rate",
		// Not needed since data store can derive the rate from req_tot
	},
	{
		Name: "req_rate_max",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "The max number of HTTP requests per second observed.",
		}},
	{
		Name: "req_tot",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of HTTP requests recieved.",
		}},
	{
		Name: "cli_abrt",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Abort,
			Desc: "The number of data transfers aborted by the client.",
		}},
	{
		Name: "srv_abrt",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Abort,
			Desc: "The number of data transfers aborted by the server.",
		}},
	{
		Name: "comp_in",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of HTTP response bytes fed to the compressor.",
		}},
	{
		Name: "comp_out",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of HTTP response bytes emitted by the compressor.",
		}},
	{
		Name: "comp_byp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of bytes that bypassed the HTTP compressor (CPU/BW limit).",
		}},
	{
		Name: "comp_rsp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Response,
			Desc: "The number of HTTP responses that were compressed.",
		}},
	{
		Name: "lastsess",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Second,
			Desc: "The number of seconds since last session assigned to server/backend.",
		}},
	{
		Name:   "last_chk",
		Ignore: true,
		// Not a series or tag so skipping this.
	},
	{
		Name:   "last_agt",
		Ignore: true,
		// Not a series or tag so skipping this.
	},
	{
		Name: "qtime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average queue time in ms over the 1024 last requests.",
		}},
	{
		Name: "ctime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average connect time in ms over the 1024 last requests.",
		}},
	{
		Name: "rtime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average response time in ms over the 1024 last requests (0 for TCP).",
		}},
	{
		Name: "ttime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average total session time in ms over the 1024 last requests.",
		}},
	{
		Name:   "agent_status",
		Ignore: true,
		// Not a series or tag so skipping this.
	},
	{
		Name:   "agent_code",
		Ignore: true,
		// Unused
	},
	{
		Name: "agent_duration",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "Time in ms taken to finish last check",
		}},
	{
		Name:   "check_desc",
		Ignore: true,
		// Can't parse
	},
	{
		Name:   "agent_desc",
		Ignore: true,
		// Can't parse
	},
	{
		Name: "check_rise",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "Server's 'rise' parameter used by checks",
		}},
	{
		Name: "check_fall",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "Server's 'fall' parameter used by checks",
		}},
	{
		Name: "check_health",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "Server's health check value",
		}},
	{
		Name: "agent_rise",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "Agents's 'rise' parameter",
		}},
	{
		Name: "agent_fall",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "Agents's 'fall' parameter",
		}},
	{
		Name: "agent_health",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "Agents's 'health' parameter",
		}},
	{
		Name:   "addr",
		Ignore: true,
		// Can't parse
	},
	{
		Name:   "cookie",
		Ignore: true,
		// Can't parse
	},
	{
		Name:   "mode",
		Ignore: true,
		// Can't parse
	},
	{
		Name:   "algo",
		Ignore: true,
		// Can't parse
	},
	{
		Name: "conn_rate",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "Number of connections over the last elapsed second",
		}},
	{
		Name: "conn_rate_max",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "Highest known conn_rate",
		}},
	{
		Name: "conn_tot",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "Cumulative number of connections",
		}},
	{
		Name: "intercepted",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "Cumulative number of intercepted requests (monitor, stats)",
		}},
	{
		Name: "dcon",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "Requests denied by 'tcp-request connection' rules",
		}},
	{
		Name: "dses",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "Requests denied by 'tcp-request session' rules",
		}},
}
