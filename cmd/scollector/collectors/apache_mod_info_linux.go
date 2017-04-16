package collectors

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const (
	apacheModInfoBaseURL = "http://127.0.0.1/server-info"
)

func init() {
	collectors = append(
		collectors,
		&IntervalCollector{
			F:        c_apache_mod_info,
			Enable:   enableURL(apacheModInfoBaseURL, "Apache Server Information"),
			Interval: time.Minute * 30,
		})
}

/* extractTimeouts processes the "?server" output and extracts timeout settings.
More specifically it finds the line that looks like:
 "Timeouts: connection: 60 keep-alive: 15"
and returns the two values contained therein.
*/
func extractTimeouts(doc *html.Node) (isServerInfo bool, connectionTimeout, keepAlive int, err error) {

	// Walk the node tree.
	// If the node is a "dt",
	//   walk its children until we find contents "Timeouts:"
	//   At which point we walk those children to find "connection".
	// Else, keep walking.
	//   Then continue walking Children to find contents starts with "connection":
	//     Return contents

	var walkSubtree func(*html.Node) string
	var searchTimeouts func(*html.Node) string

	walkSubtree = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "h1" {
			if n := n.FirstChild; n != nil && n.Type == html.TextNode && n.Data == "Apache Server Information" {
				isServerInfo = true
			}
		} else if n.Type == html.ElementNode && n.Data == "dt" {
			if val := searchTimeouts(n); val != "" {
				return val
			}
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if val := walkSubtree(c); val != "" {
					return val
				}
			}
		}
		return ""
	}

	searchTimeouts = func(n *html.Node) string {
		if n.Type == html.TextNode && strings.HasPrefix(n.Data, "connection:") {
			return n.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if value := searchTimeouts(c); value != "" {
				return value
			}
		}
		return ""
	}

	original := walkSubtree(doc)
	parts := strings.Fields(original)
	if len(parts) < 4 {
		err = fmt.Errorf("fewer than 4 fields found on connection:/keep-alive line")
		return
	}
	if (parts[0] != "connection:") || (parts[2] != "keep-alive:") {
		err = fmt.Errorf("format changed in connection:/keep-alive: line")
		return
	}

	connectionTimeout, err = strconv.Atoi(parts[1])
	if err != nil {
		err = fmt.Errorf("connection timeout is not an integer")
		return
	}
	keepAlive, err = strconv.Atoi(parts[3])
	if err != nil {
		err = fmt.Errorf("keep-alive value is not an integer")
		return
	}
	return
}

/* extractMpmInfo processes the "?server" parse tree and extracts MPM settings.
More specifically it finds the line that looks like:
 "MPM Information: Max Daemons: 64 Threaded: no Forked: yes"
and returns the three values contained therein.
*/
func extractMpmInfo(doc *html.Node) (int, bool, bool, error) {

	// Walk the node tree.
	// If the node is a "dt",
	//   walk its children until we find contents "Timeouts:"
	//   At which point we walk those children to find "connection".
	// Else, keep walking.
	//   Then continue walking Children to find contents starts with "connection":
	//     Return contents

	var walkSubtree func(*html.Node) string
	var hasMpmInfo func(*html.Node) bool
	var getMpmInfo func(*html.Node) string

	walkSubtree = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "dt" {
			if hasMpmInfo(n) {
				if sib := n.FirstChild.NextSibling; sib != nil {
					return getMpmInfo(sib)
				}
				return ""
			}
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if val := walkSubtree(c); val != "" {
					return val
				}
			}
		}
		return ""
	}

	hasMpmInfo = func(n *html.Node) bool {
		if n.Type == html.TextNode && strings.HasPrefix(n.Data, "MPM Information:") {
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if value := hasMpmInfo(c); value {
				return true
			}
		}
		return false
	}

	getMpmInfo = func(n *html.Node) string {
		// Scan across the siblings:
		for s := n; s != nil; s = s.NextSibling {
			// if it is a text node, see if it what we want and return it.
			if s.Type == html.TextNode {
				if strings.HasPrefix(s.Data, "Max Daemons:") {
					return s.Data
				}
			} else if s.Type == html.ElementNode {
				// Resursively check the children:
				for c := s.FirstChild; c != nil; c = c.NextSibling {
					if val := getMpmInfo(c); val != "" {
						return val
					}
				}
			}
		}
		return ""
	}

	original := walkSubtree(doc)
	parts := strings.Fields(original)
	if len(parts) < 7 {
		return 0, false, false, fmt.Errorf("fewer than 7 fields found on MPM Information line")
	}
	// Max Daemons: 64 Threaded: no Forked: yes
	// 0   1        2  3         4  5       6
	if (parts[0] != "Max") || (parts[1] != "Daemons:") || (parts[3] != "Threaded:") || (parts[5] != "Forked:") {
		return 0, false, false, fmt.Errorf("wrong format found on MPM Information line")
	}

	max_daemons, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, false, false, fmt.Errorf("Max Daemons is not an integer: %v", parts[2])
	}

	threaded := parts[4] == "yes"
	forked := parts[6] == "yes"

	return max_daemons, threaded, forked, nil
}

func c_apache_mod_info() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	resp, err := http.Get(apacheModInfoBaseURL + "?server")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	n, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to parse ?server status page")
	}

	isServerInfo, connection_timeout, keepalive, err := extractTimeouts(n)
	if !isServerInfo {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	Add(&md, "apache.server.timeout.connection", connection_timeout, nil, metadata.Gauge, metadata.Second, "")
	Add(&md, "apache.server.timeout.keepalive", keepalive, nil, metadata.Gauge, metadata.Second, "")

	max_daemons, is_threaded, is_forked, err := extractMpmInfo(n)
	if err != nil {
		return nil, err
	}
	is_threaded_num := 0
	if is_threaded {
		is_threaded_num = 1
	}
	is_forked_num := 0
	if is_forked {
		is_forked_num = 1
	}
	Add(&md, "apache.mpm.daemons_max", max_daemons, nil, metadata.Gauge, metadata.Bool, "")
	Add(&md, "apache.mpm.threaded", is_threaded_num, nil, metadata.Gauge, metadata.Bool, "")
	Add(&md, "apache.mpm.forked", is_forked_num, nil, metadata.Gauge, metadata.Bool, "")

	return md, nil
}
