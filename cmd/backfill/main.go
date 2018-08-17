// Backfill denormalizes historic OpenTSDB data.
//
// For ongoing denormalization use the functionality in tsdbrelay.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"strconv"
	"time"

	"bosun.org/cmd/tsdbrelay/denormalize"
	"bosun.org/collect"
	"bosun.org/opentsdb"
)

var (
	start     = flag.String("start", "2013/01/01", "Start date to backfill.")
	end       = flag.String("end", "", "End date to backfill. Will go to now if not specified.")
	ruleFlag  = flag.String("rule", "", "A denormalization rule. ex `os.cpu__host`")
	tsdbHost  = flag.String("host", "", "OpenTSDB host")
	batchSize = flag.Int("batch", 500, "batch size to send points to OpenTSDB")
)

func main() {
	flag.Parse()
	if *tsdbHost == "" {
		flag.PrintDefaults()
		log.Fatal("host must be supplied")
	}
	putUrl := (&url.URL{Scheme: "http", Host: *tsdbHost, Path: "api/put"}).String()

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
	var metric string
	for k, v := range rules {
		metric = k
		rule = v
	}

	query := &opentsdb.Query{Metric: metric, Aggregator: "avg"}
	query.Tags, err = queryForAggregateTags(query)
	if err != nil {
		log.Fatal(err)
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

	backfill := func(batchStart, batchEnd time.Time) (err error) {
		startTimeString := batchStart.Format(opentsdb.TSDBTimeFormat)
		endTimeString := batchEnd.Format(opentsdb.TSDBTimeFormat)
		defer func() {
			if err != nil {
				log.Fatalf("Error on batch %s - %s. %v \n", startTimeString, endTimeString, err)
			}
		}()
		req := opentsdb.Request{Start: startTimeString, End: endTimeString, Queries: []*opentsdb.Query{query}}
		resp, err := req.Query(*tsdbHost)
		if err != nil {
			return err
		}
		dps := []*opentsdb.DataPoint{}
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
				dps = append(dps, dp)
			}
		}
		fmt.Printf("%s - %s: %d dps\n", startTimeString, endTimeString, len(dps))
		total := 0
		for len(dps) > 0 {
			count := len(dps)
			if len(dps) > *batchSize {
				count = *batchSize
			}
			putResp, err := collect.SendDataPoints(dps[:count], putUrl)
			if err != nil {
				return err
			}
			defer putResp.Body.Close()
			// Drain up to 512 bytes and close the body to let the Transport reuse the connection
			io.CopyN(ioutil.Discard, putResp.Body, 512)

			if putResp.StatusCode != 204 {
				return fmt.Errorf("Non 204 status code from opentsdb: %d", putResp.StatusCode)
			}
			dps = dps[count:]
			total += count
		}
		fmt.Printf("Relayed %d data points.\n", total)
		return nil
	}

	// walk backwards a day at a time
	curEnd := endDate
	for curEnd.After(startDate) {
		curStart := curEnd.Add(-24 * time.Hour)
		if curStart.Before(startDate) {
			curStart = startDate
		}
		backfill(curStart, curEnd)
		curEnd = curEnd.Add(-24 * time.Hour)
	}
}

func queryForAggregateTags(query *opentsdb.Query) (opentsdb.TagSet, error) {
	req := opentsdb.Request{}
	req.Queries = []*opentsdb.Query{query}
	req.Start = "1h-ago"
	resp, err := req.Query(*tsdbHost)
	if err != nil {
		return nil, err
	}
	if len(resp) < 1 {
		return nil, fmt.Errorf("No points in last hour to learn aggregate tags")
	}
	tagset := make(opentsdb.TagSet)
	for _, t := range resp[0].AggregateTags {
		tagset[t] = "*"
	}
	return tagset, nil
}
