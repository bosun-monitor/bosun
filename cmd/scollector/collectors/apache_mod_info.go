package collectors

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bosun.org/_third_party/golang.org/x/net/html"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const (
	apacheModInfoURL = "http://127.0.0.1/server-info"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_apache_mod_info, Enable: enableURL(apacheModInfoURL), Interval: time.Minute * 30})
}

/* extractTimeouts processes the "?server" output and extracts timeout settings.
More specifically it finds the line that looks like:
 "Timeouts: connection: 60 keep-alive: 15"
and returns the two values contained therein.
*/
func extractTimeouts(doc *html.Node) (int, int, error) {

	// Walk the node tree.
	// If the node is a "dt",
	//   walk its children until we find contents "Timeouts:"
	//   At which point we walk those children to find "connection".
	// Else, keep walking.
	//   Then continue walking Children to find contents starts with "connection":
	//     Return contents

	var search_children func(*html.Node) string
	search_children = func(n *html.Node) string {
		if n.Type == html.TextNode && strings.HasPrefix(n.Data, "connection:") {
			return n.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if value := search_children(c); value != "" {
				return value
			}
		}
		return ""
	}

	var walk_subtree func(*html.Node) string
	walk_subtree = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "dt" {
			if val := search_children(n); val != "" {
				return val
			}
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if val := walk_subtree(c); val != "" {
					return val
				}
			}
		}
		return ""
	}

	original := walk_subtree(doc)
	parts := strings.Fields(original)
	if len(parts) < 4 {
		return 0, 0, fmt.Errorf("more than 4 fields found on connection:/keep-alive line")
	}
	if (parts[0] != "connection:") || (parts[2] != "keep-alive:") {
		return 0, 0, fmt.Errorf("format changed in connection:/keep-alive: line")
	}

	c, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("connection timeout is not an integer")
	}
	k, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, 0, fmt.Errorf("keep-alive value is not an integer")
	}
	return c, k, nil
}

func c_apache_mod_info() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	resp, err := http.Get(apacheModInfoURL + "?server")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	n, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("unable to parse ?server status page")
	}

	connection_timeout, keepalive, err := extractTimeouts(n)
	if err != nil {
		return nil, err
	}
	Add(&md, "apache.server.timeout.connection", connection_timeout, nil, metadata.Gauge, metadata.Second, "")
	Add(&md, "apache.server.timeout.keepalive", keepalive, nil, metadata.Gauge, metadata.Second, "")

	return md, nil
}
