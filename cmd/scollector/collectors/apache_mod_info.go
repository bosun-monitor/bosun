package collectors

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_apache_mod_info, Interval: time.Minute * 30})
}

// Extract from HTML recieved from http://127.0.0.1/server-info?server
// Find the line that looks like:
// "Timeouts: connection: 60 keep-alive: 15"
// Returns the two values contained therein.

func extract_timeouts(doc *html.Node) (string, string) {

	// Walk the node tree.
	// If the node is a "dt",
	//     walk its children until we find CONTENTS "Timeouts:"
	//         At which point we walk those children to find "connection".
	// Else, keep walking.
	//     Then continue walking Children to find CONTENTS startswith "connection":
	//     Return contents

	var value string
	var search_children func(*html.Node) string
	search_children = func(n *html.Node) string {
		if n.Type == html.TextNode {
			if strings.HasPrefix(n.Data, "connection:") {
				value = n.Data
				return value
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if value = search_children(c); value != "" {
				return value
			}
		}
		return value
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
	if (parts[0] != "connection:") || (parts[2] != "keep-alive:") {
		log.Fatal("Format of connection: / keep-alive: line changed")
	}
	return parts[1], parts[3]

}

func c_apache_mod_info() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	// body, err := ioutil.ReadFile("/Users/tal/Dropbox/work/apache/mi-server.html")
	resp, err := http.Get("http://127.0.0.1/server-info?server")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	n, _ := html.Parse(strings.NewReader(string(body)))

	connection_timeout, keepalive := extract_timeouts(n)
	Add(&md, "apache.server.timeout.connection", connection_timeout, nil, metadata.Gauge, metadata.Gauge, "")
	Add(&md, "apache.server.timeout.keepalive", keepalive, nil, metadata.Gauge, metadata.Gauge, "")

	return md, nil
}

/*

https://godoc.org/golang.org/x/net/html#example-Parse

/Users/tal/Dropbox/work/apache/mi-server.html
Timeouts: connection: 60    keep-alive: 15
MPM Information: Max Daemons: 64 Threaded: no Forked: yes
apache.server.connection_timeout
apache.server.keepalive_timeout
apache.server.max_daemons
apache.server.threaded 0/1
apache.server.forked 0/1

/Users/tal/Dropbox/work/apache/mi-prefork.c.html
ServerLimit 512
MaxClients 64
apache.prefork.serverlimit
apache.prefork.maxclients

/Users/tal/Dropbox/work/apache/mi-config.html
Cound how many times "<VirtualHost " is seen.
apache.config.virtualhost_count
*/
