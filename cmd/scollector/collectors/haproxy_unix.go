package collectors

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

// HAProxy registers an HAProxy collector.
func HAProxy(user, pwd, url, tier string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_haproxy_csv(user, pwd, url)
		},
		name: fmt.Sprintf("haproxy-%s", url, tier),
	})
}

func c_haproxy_csv(user, pwd, url, tier string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	const metric = "haproxy"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, pwd)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	//csvFile, err := os.Open("/ha.csv")
	if err != nil {
		return nil, err
	}
	//defer csvFile.Close()
	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}
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
	for _, rec := range records[1:] {
		// There is a trailing comma in haproxy's csv
		if len(rec) != len(haproxyCsvMeta)+1 {
			return nil, fmt.Errorf("expected %v lines. got: %v",
				len(haproxyCsvMeta)+1, len(rec))
		}
		hType := haproxyType[rec[32]]
		pxname := rec[0]
		svname := rec[1]
		ts := opentsdb.TagSet{"pxname": pxname, "svname": svname}
		// TODO MERGE IN INSTANCE TAG
		for i, field := range haproxyCsvMeta {
			m := strings.Join([]string{metric, hType, field.Name}, ".")
			switch i {
			case 0, 1, 26, 27, 28, 31, 32, 56, 57:
				continue
			case 39, 40, 41, 42, 43, 44:
				sp := strings.Split(field.Name, "_")
				if len(sp) != 2 {
					return nil, fmt.Errorf("unexpected field name %v in hrsp", field.Name)
				}
				ts := ts.Copy().Merge(opentsdb.TagSet{"status_code": sp[1]})
				m = strings.Join([]string{metric, hType, sp[0]}, ".")
				v, err := parse(rec[i])
				if err != nil {
					return nil, err
				}
				Add(&md, m, v, ts, metadata.Counter, metadata.Response,
					fmt.Sprintf("The number of http responses with a %v status code.", sp[1]))
			case 17:
				v, ok := haproxyStatus[rec[i]]
				// Not distinging between MAINT and MAINT via...
				if !ok {
					v = 3
				}
				Add(&md, m, v, ts, field.RateType, field.Unit, field.Desc)
			case 36:
				if rec[i] == "" {
					continue
				}
				v, ok := haproxyCheckStatus[rec[i]]
				if !ok {
					return nil, fmt.Errorf("unknown check status %v", rec[i])
				}
				Add(&md, m, v, ts, field.RateType, field.Unit, field.Desc)
			default:
				v, err := parse(rec[i])
				if err != nil {
					return nil, err
				}
				Add(&md, m, v, ts, field.RateType, field.Unit, field.Desc)
			}
		}
	}
	return md, nil
}

type MetricMetaName struct {
	Name string
	MetricMeta
}

var haproxyType = map[string]string{
	"0": "frontend",
	"1": "backend",
	"2": "server",
	"3": "listen",
}

var haproxyCheckStatus = map[string]float64{
	"UNK":     0,
	"INI":     1,
	"SOCKERR": 2,
	"L4OK":    3,
	"L4TMOUT": 4,
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

var haproxyStatus = map[string]float64{
	"UP":    0,
	"DOWN":  1,
	"NOLB":  2,
	"MAINT": 3,
}

var haproxyCsvMeta = []MetricMetaName{
	MetricMetaName{
		Name: "pxname",
	},
	MetricMetaName{
		Name: "svname",
	},
	MetricMetaName{
		Name: "qcur",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "The current queued requests. For the backend this reports the number queued without a server assigned.",
		}},
	MetricMetaName{
		Name: "qmax",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "The max value of qcur.",
		}},
	MetricMetaName{
		Name: "scur",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The current number of sessions.",
		}},
	MetricMetaName{
		Name: "smax",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The maximum number of concurrent sessions seen.",
		}},
	MetricMetaName{
		Name: "slim",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The configured session limit.",
		}},
	MetricMetaName{
		Name: "stot",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Session,
			Desc: "The total number of sessions.",
		}},
	MetricMetaName{
		Name: "bin",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of bytes in.",
		}},
	MetricMetaName{
		Name: "bout",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of bytes out.",
		}},
	MetricMetaName{
		Name: "dreq",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of requests denied because of security concerns. For tcp this is because of a matched tcp-request content rule. For http this is because of a matched http-request or tarpit rule.",
		}},
	MetricMetaName{
		Name: "dresp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Response,
			Desc: "The number of responses denied because of security concerns. For http this is because of a matched http-request rule, or 'option checkcache'.",
		}},
	MetricMetaName{
		Name: "ereq",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of request errors. Some of the possible causes are: Early termination from the client before the request has been sent, a read error from the client, a client timeout, a client closed connection, various bad requests from the client or the request was tarpitted.",
		}},
	MetricMetaName{
		Name: "econ",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of number of requests that encountered an error trying to connect to a backend server. The backend stat is the sum of the stat for all servers of that backend, plus any connection errors not associated with a particular server (such as the backend having no active servers).",
		}},
	MetricMetaName{
		Name: "eresp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Response,
			Desc: " The number of response errors. srv_abrt will be counted here also. Some errors are: write error on the client socket (won't be counted for the server stat) and failure applying filters to the response.",
		}},
	MetricMetaName{
		Name: "wretr",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Retry,
			Desc: "The number of times a connection to a server was retried.",
		}},
	MetricMetaName{
		Name: "wredis",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Redispatch,
			Desc: "number of times a request was redispatched to another server. The server value counts the number of times that server was switched away from.",
		}},
	MetricMetaName{
		Name: "status",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Weight,
			Desc: "The current status: 0->UP, 1->Down, 2->NOLB, 3->Maintenance.",
		}},
	MetricMetaName{
		Name: "weight",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Weight,
			Desc: "The server weight (server), total weight (backend).",
		}},
	MetricMetaName{
		Name: "act",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Server,
			Desc: "If the server is active in the case of servers, or number of active servers in the case of a backend.",
		}},
	MetricMetaName{
		Name: "bck",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Server,
			Desc: "If the server is a backup in the case of servers, or number of backup servers in the case of a backend.",
		}},
	MetricMetaName{
		Name: "chkfail",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Check,
			Desc: "The number of failed checks. (Only counts checks failed when the server is up.",
		}},
	MetricMetaName{
		Name: "chkdown",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Transition,
			Desc: "The number of UP->DOWN transitions. The backend counter counts transitions to the whole backend being down, rather than the sum of the counters for each server.",
		}},
	MetricMetaName{
		Name: "lastchg",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Second,
			Desc: "The number of seconds since the last UP<->DOWN transition.",
		}},
	MetricMetaName{
		Name: "downtime",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Second,
			Desc: "The total downtime in seconds. The value for the backend is the downtime for the whole backend, not the sum of the server downtime.",
		}},
	MetricMetaName{
		Name: "qlimit",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			//Don't know the unit
			Desc: "The configured maxqueue for the server, or nothing in the value is 0 (default, meaning no limit)",
		}},
	MetricMetaName{
		Name: "pid",
		// Not a series or tag so skipping this.
	},
	MetricMetaName{
		Name: "iid",
		// Not a series or tag so skipping this.
	},
	MetricMetaName{
		Name: "sid",
		// Not a series or tag so skipping this.
	},
	MetricMetaName{
		Name: "throttle",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Pct,
			Desc: "The current throttle percentage for the server, when slowstart is active, or no value if not in slowstart.",
		}},
	MetricMetaName{
		Name: "lbtot",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			//Don't know the unit
			Desc: "The total number of times a server was selected, either for new sessions, or when re-dispatching. The server counter is the number of times that server was selected.",
		}},
	MetricMetaName{
		Name: "tracked",
		// This could be a tag, but I am have no use for it.
	},
	MetricMetaName{
		Name: "type",
		// This could be a tag, but I am have no use for it.
	},
	MetricMetaName{
		Name: "rate",
		// This could be a tag, but I am have no use for it.
	},
	MetricMetaName{
		Name: "rate_lim",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Session,
			Desc: "The configured limit on new sessions per second.",
		}},
	MetricMetaName{
		Name: "rate_max",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Session,
			Desc: "The max number of new sessions per second.",
		}},
	MetricMetaName{
		Name: "check_status",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "The status of last health check, one of: 0 -> unknown, 1 -> initializing, 2 -> socket error, 3 -> The check passed on layer 4, but no upper layers testing enabled, 4 -> layer 1-4 timeout, 5 -> layer 1-4 connection problem for example 'Connection refused' (tcp rst) or 'No route to host' (icmp), 6 -> check passed on layer 6, 7 -> layer 6 (SSL) timeout, 8 -> layer 6 invalid response - protocol error, 9 -> check passed on layer 7, 10 -> check conditionally passed on layer 7 for example 404 with disable-on-404, 11 -> layer 7 (HTTP/SMTP) timeout, 12 -> layer 7 invalid response - protocol error, 13 -> layer 7 response error, for example HTTP 5xx.",
		}},
	MetricMetaName{
		Name: "check_code",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.StatusCode,
			Desc: "The layer5-7 code, if available.",
		}},
	MetricMetaName{
		Name: "check_duration",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The time in ms it took to finish last health check.",
		}},
	MetricMetaName{
		Name: "hrsp_1xx",
		//These are transformed and aggregated: 1xx, 2xx, etc will be a tag.
	},
	MetricMetaName{
		Name: "hrsp_2xx",
	},
	MetricMetaName{
		Name: "hrsp_3xx",
	},
	MetricMetaName{
		Name: "hrsp_4xx",
	},
	MetricMetaName{
		Name: "hrsp_5xx",
	},
	MetricMetaName{
		Name: "hrsp_other",
	},
	MetricMetaName{
		Name: "hanafail",
		// The docs just say "failed health check details", so skipping this
		// for now
	},
	MetricMetaName{
		Name: "req_rate",
		// Not needed since data store can derive the rate from req_tot
	},
	MetricMetaName{
		Name: "req_rate_max",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Request,
			Desc: "The max number of HTTP requests per second observed.",
		}},
	MetricMetaName{
		Name: "req_tot",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Request,
			Desc: "The number of HTTP requests recieved.",
		}},
	MetricMetaName{
		Name: "cli_abrt",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Abort,
			Desc: "The number of data transfers aborted by the client.",
		}},
	MetricMetaName{
		Name: "srv_abrt",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Abort,
			Desc: "The number of data transfers aborted by the server.",
		}},
	MetricMetaName{
		Name: "comp_in",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of HTTP response bytes fed to the compressor.",
		}},
	MetricMetaName{
		Name: "comp_out",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of HTTP response bytes emitted by the compressor.",
		}},
	MetricMetaName{
		Name: "comp_byp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Bytes,
			Desc: "The number of bytes that bypassed the HTTP compressor (CPU/BW limit).",
		}},
	MetricMetaName{
		Name: "comp_rsp",
		MetricMeta: MetricMeta{RateType: metadata.Counter,
			Unit: metadata.Response,
			Desc: "The number of HTTP responses that were compressed.",
		}},
	MetricMetaName{
		Name: "lastsess",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.Second,
			Desc: "The number of seconds since last session assigned to server/backend.",
		}},
	MetricMetaName{
		Name: "last_chk",
		// Not a series or tag so skipping this.
	},
	MetricMetaName{
		Name: "last_agt",
		// Not a series or tag so skipping this.
	},
	MetricMetaName{
		Name: "qtime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average queue time in ms over the 1024 last requests.",
		}},
	MetricMetaName{
		Name: "ctime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average connect time in ms over the 1024 last requests.",
		}},
	MetricMetaName{
		Name: "rtime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average response time in ms over the 1024 last requests (0 for TCP).",
		}},
	MetricMetaName{
		Name: "ttime",
		MetricMeta: MetricMeta{RateType: metadata.Gauge,
			Unit: metadata.MilliSecond,
			Desc: "The average response time in ms over the 1024 last requests (0 for TCP).",
		}},
}
