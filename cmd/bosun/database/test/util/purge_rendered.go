package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"bosun.org/slog"

	"bosun.org/cmd/bosun/database"
)

var fredis = flag.String("r", "", "redis host:port")

var deleteIfAge = time.Hour * 24 * 60

func main() {
	flag.Parse()
	if *fredis == "" {
		flag.PrintDefaults()
		return
	}
	db := database.NewDataAccess(*fredis, true, 0, "")
	earliestOk := time.Now().UTC().Add(-1 * deleteIfAge)

	keys, err := db.State().GetRenderedTemplateKeys()
	if err != nil {
		log.Fatal(err)
	}
	toPurge := []int64{}
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			slog.Errorf("Invalid rendered template redis key found: %s", key)
			continue
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			slog.Error(err)
			continue
		}
		state, err := db.State().GetIncidentState(id)
		if err != nil {
			if strings.Contains(err.Error(), "nil returned") {
				toPurge = append(toPurge, id)
				continue
			}
			slog.Error(err)
			continue
		}
		if state.End != nil && (*state.End).Before(earliestOk) {
			toPurge = append(toPurge, id)
		}
	}
	fmt.Println(len(toPurge), "rendered templates purgable")
	err = db.State().DeleteRenderedTemplates(toPurge)
	if err != nil {
		log.Fatal(err)
	}
}
