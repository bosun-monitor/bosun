package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/StackExchange/wmi"
)

var enableWmi = flag.Bool("w", false, "WMI query mode. Internal use only.")

func init() {
	mains = append(mains, wmi_main)
}

func wmi_main() {
	if !*enableWmi {
		return
	}
	// WMI has heap corruption issues with the GC.
	debug.SetGCPercent(-1)
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		t := s.Bytes()
		if len(t) == 0 {
			break
		}
		r := runQuery(t)
		b, _ := json.Marshal(r)
		fmt.Println(string(b))
	}
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

func runQuery(q []byte) (resp *wmi.Response) {
	resp = new(wmi.Response)
	var columns []string
	var query wmi.WmiQuery
	if err := json.Unmarshal(q, &query); err != nil {
		resp.Error = err.Error()
		return
	}
	for i, v := range strings.Fields(query.Query) {
		if i == 0 {
			if strings.ToLower(v) != "select" {
				resp.Error = "wmi: expected select"
				return
			}
			continue
		} else if v == "*" {
			resp.Error = "wmi: must specify columns, * not supported"
			return
		} else if strings.ToLower(v) == "from" {
			break
		}
		sp := strings.Split(v, ",")
		for _, s := range sp {
			if len(s) > 0 {
				columns = append(columns, s)
			}
		}
	}
	var args []interface{}
	if query.Namespace != "" {
		args = append(args, nil, query.Namespace)
	}
	r, err := wmi.QueryGen(query.Query, columns, args...)
	if err != nil {
		resp.Error = err.Error()
		return
	}
	resp.Response = r
	return
}
