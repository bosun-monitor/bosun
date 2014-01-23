package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/StackExchange/wmi"
)

var namespace = flag.String("n", "", "WMI Namespace")
var enableWmi = flag.Bool("w", false, "WMI Query mode. If specified, all other flags are ignored except -n. Pass the query as the remaining arguments: tcollector -w select name, processid from win32_process. On successful exit, prints JSON to stdout.")

func init() {
	mains = append(mains, wmi_main)
}

func wmi_main() {
	if !*enableWmi {
		return
	}
	q := strings.Join(flag.Args(), " ")
	if len(q) == 0 {
		log.Fatal("wmi: no query specified")
	}
	var columns []string
	for i, v := range flag.Args() {
		if i == 0 {
			if strings.ToLower(v) != "select" {
				log.Fatal("wmi: expected select")
			}
			continue
		} else if v == "*" {
			log.Fatal("wmi: must specify columns, * not supported")
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
	if len(columns) == 0 {
		log.Fatal("wmi: no columns specified")
	}
	// WMI has heap corruption issues with the GC.
	debug.SetGCPercent(-1)
	var args []interface{}
	if *namespace != "" {
		args = append(args, nil, *namespace)
	}
	r, err := wmi.QueryGen(q, columns, args...)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(r[:1])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
	os.Exit(0)
}
