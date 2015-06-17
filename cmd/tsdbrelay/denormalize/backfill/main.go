package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"bosun.org/cmd/tsdbrelay/denormalize"
	"bosun.org/collect"
	"bosun.org/opentsdb"
)

var (
	q        = flag.String("q", "", "query to opentsdb. Include all tags. ex `avg:os.cpu{host=*}'")
	start    = flag.String("start", "2013/01/01", "Start date to backfill.")
	end      = flag.String("end", "", "End date to backfill. Will go to now if not specified.")
	ruleFlag = flag.String("rule", "", "A denormalization rule. ex `os.cpu__host`")
)

func main() {
	flag.Parse()
	if *q == "" {
		flag.PrintDefaults()
		log.Fatal("q must be specified")
	}
	query, err := opentsdb.ParseQuery(*q)
	if err != nil {
		log.Fatal(err)
	}
	if *ruleFlag == "" {
		flag.PrintDefaults()
		log.Fatal("rule must be supplied")
	}
	rules, err := denormalize.ParseDenormalizationRules(*ruleFlag)
	if err != nil {
		log.Fatal(err)
	}
	if len(rules) > 1 {
		log.Fatal("Please specify only one rule")
	}
	var rule *denormalize.DenormalizationRule
	for _, v := range rules {
		rule = v
	}
	startDate, err := opentsdb.ParseTime(*start)
	if err != nil {
		log.Fatal(err)
	}
	endDate := time.Now().UTC()
	if *end != "" {
		endDate, err = opentsdb.ParseTime(*end)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(startDate, endDate, rule)

	dpChan := make(chan *opentsdb.DataPoint)
	collect.BatchSize = 1000
	collect.MaxQueueLen = 10000000
	collect.DisableDefaultCollectors = true
	collect.Freq = time.Second
	collect.Debug = true

	u, err := url.Parse("http://devbosun:4242")
	if err != nil {
		log.Fatal(err)
	}

	err = collect.InitChan(u, "", dpChan)
	if err != nil {
		log.Fatal(err)
	}

	backfill := func(batchStart, batchEnd time.Time) (err error) {
		startTimeString := batchStart.Format(opentsdb.TSDBTimeFormat)
		endTimeString := batchEnd.Format(opentsdb.TSDBTimeFormat)
		defer func() {
			if err != nil {
				log.Fatalf("Error on batch %s - %s. %v \n", startTimeString, endTimeString, err)
			}
		}()
		req := opentsdb.Request{Start: startTimeString, End: endTimeString, Queries: []*opentsdb.Query{query}}
		resp, err := req.Query("devbosun:4242")
		if err != nil {
			return err
		}
		count := 0
		for _, r := range resp {
			for t, p := range r.DPS {

				timeStamp, err := strconv.ParseInt(t, 10, 64)
				if err != nil {
					return err
				}
				dp := &opentsdb.DataPoint{
					Timestamp: timeStamp,
					Metric:    r.Metric,
					Tags:      r.Tags,
					Value:     p,
				}
				err = rule.Translate(dp)
				if err != nil {
					return err
				}
				dpChan <- dp
				count++
			}
		}
		fmt.Printf("%s - %s: %d dps\n", startTimeString, endTimeString, count)

		return nil
	}

	// walk backwards a day at a time
	curEnd := endDate
	for curEnd.After(startDate) {
		curStart := curEnd.Add(-1 * time.Hour)
		if curStart.Before(startDate) {
			curStart = startDate
		}
		backfill(curStart, curEnd)
		curEnd = curEnd.Add(-1 * time.Hour)
	}
	for collect.QueueLength() > 0 {
		time.Sleep(1 * time.Second)
	}
}
