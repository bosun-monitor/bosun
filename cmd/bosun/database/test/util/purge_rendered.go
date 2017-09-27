package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"bosun.org/cmd/bosun/database"
	"bosun.org/models"
	"github.com/garyburd/redigo/redis"
)

var fredis = flag.String("r", "", "redis host:port")

var deleteIfAge = time.Hour * 24 * 60

func main() {
	flag.Parse()
	if *fredis == "" {
		flag.PrintDefaults()
		return
	}
	ch := make(chan *models.IncidentState)
	db := database.NewDataAccess(*fredis, true, 0, "")
	myConn := db.Get()
	total := int64(0)
	earliestOk := time.Now().UTC().Add(-1 * deleteIfAge)
	ids := []int64{}
	go func() {
		for st := range ch {
			if st.Open {
				continue
			}
			if len(st.Actions) == 0 || st.Actions[len(st.Actions)-1].Time.Before(earliestOk) {
				ids = append(ids, st.Id)
			} else {
				continue
			}
			fmt.Println(st.Id, len(st.Events), st.Open)
			s, err := redis.String(myConn.Do("GET", fmt.Sprintf("renderedTemplatesById:%d", st.Id)))
			if err != nil {
				log.Fatal(err)
			}
			total += int64(len(s))
		}
	}()
	err := db.State().ScanRenderedTemplates(ch)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(total, "bytes!", len(ids), "incidents")
	err = db.State().DeleteRenderedTemplates(ids)
	if err != nil {
		log.Fatal(err)
	}
}
