package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"bosun.org/opentsdb"
	"github.com/influxdb/influxdb/client"
)

const (
	bosunHost  = "bosun"
	tsdbHost   = "localhost:4242"
	influxHost = "localhost:8086"
)

var (
	startQuery = time.Date(2016, time.January, 13, 12, 0, 0, 0, time.UTC)
	endQuery   = startQuery.Add(time.Hour - time.Second)
	resPath    = "results"
)

func main() {
	if _, err := os.Stat(resPath); err != nil {
		err := fetch()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("%s present; not fetching", resPath)
	}
	if err := toInflux(); err != nil {
		log.Fatal(err)
	}
	if err := toTSDB(); err != nil {
		log.Fatal(err)
	}
}

// 2015/04/16 00:25:12 to tsdb done 8m28.173734487s
// 100MB
func toTSDB() error {
	var total time.Duration
	for rs := range getResults() {
		log.Println("tsdb got", rs[0].Metric)
		md := make(opentsdb.MultiDataPoint, 0, 1024*10)
		for _, r := range rs {
			for k, v := range r.DPS {
				t, err := strconv.ParseInt(k, 10, 64)
				if err != nil {
					log.Fatal(err)
				}
				md = append(md, &opentsdb.DataPoint{
					Metric:    r.Metric,
					Timestamp: t,
					Value:     v,
					Tags:      r.Tags,
				})
			}
		}
		for len(md) > 0 {
			n := len(md)
			const max = 10000
			if n > max {
				n = max
			}
			nmd := md[:n]
			md = md[n:]
			b, err := json.Marshal(nmd)
			if err != nil {
				log.Fatal(err)
			}
			buf := bytes.NewBuffer(b)
			println(rs[0].Metric, len(nmd), len(b))
			start := time.Now()
			res, err := http.Post(fmt.Sprintf("http://%s/api/put", tsdbHost), "", buf)
			dur := time.Since(start)
			if err != nil {
				log.Fatal(err)
			} else if res.StatusCode != 204 {
				io.Copy(os.Stderr, res.Body)
				log.Println()
				log.Fatal(res.Status)
			}
			res.Body.Close()
			total += dur
		}
	}
	log.Println("to tsdb done", total)
	return nil
}

// 2015/04/15 02:13:35 to influx done 1h13m56.130517458s
// 7.6G
func toInflux() error {
	return nil
	u := &url.URL{
		Scheme:   "http",
		Host:     influxHost,
		Path:     "/query",
		RawQuery: url.Values{"q": {"CREATE DATABASE db"}}.Encode(),
	}
	res, err := http.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode != 200 {
		log.Println("influx db create:", res.Status)
		b, _ := ioutil.ReadAll(res.Body)
		log.Println(string(b))
	}
	conf := client.Config{
		URL: url.URL{
			Scheme: "http",
			Host:   influxHost,
		},
	}
	c, err := client.NewClient(conf)
	if err != nil {
		return err
	}
	var total time.Duration
	for rs := range getResults() {
		log.Println("influx got", rs[0].Metric)
		points := make([]client.Point, 0)
		for _, r := range rs {
			for k, v := range r.DPS {
				i, err := strconv.ParseInt(k, 10, 64)
				if err != nil {
					log.Fatal(err)
				}
				points = append(points, client.Point{
					Name:      r.Metric,
					Tags:      map[string]string(r.Tags),
					Timestamp: time.Unix(i, 0),
					Fields: map[string]interface{}{
						"value": float64(v),
					},
				})
			}
		}
		bp := client.BatchPoints{
			Points:   points,
			Database: "db",
		}
		start := time.Now()
		_, err := c.Write(bp)
		dur := time.Since(start)
		if err != nil {
			log.Fatal(err)
		}
		total += dur
	}
	log.Println("to influx done", total)
	return nil
}

func getResults() chan opentsdb.ResponseSet {
	f, err := os.Open(resPath)
	if err != nil {
		log.Fatal(err)
	}
	names, err := f.Readdirnames(0)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()
	ch := make(chan opentsdb.ResponseSet)
	go func() {
		for _, n := range names {
			f, err := os.Open(filepath.Join(resPath, n))
			if err != nil {
				log.Fatal(err)
			}
			var r opentsdb.ResponseSet
			if err := gob.NewDecoder(f).Decode(&r); err != nil {
				log.Fatal(err)
			}
			f.Close()
			ch <- r
		}
		close(ch)
	}()
	return ch
}

// Get all metrics. For each metric, get all tag keys. For all tag keys,
// query one hour of some day.
func fetch() error {
	if err := os.RemoveAll(resPath); err != nil {
		return err
	}
	os.Mkdir(resPath, 0644)
	b, err := bosunReq("/api/metric")
	if err != nil {
		return err
	}
	var metrics []string
	if err := json.Unmarshal(b, &metrics); err != nil {
		return err
	}
	start := time.Now()
	ch := make(chan bool, 20)
	get := func(mi int, m string) error {
		b, err := bosunReq("/api/tagk/" + m)
		if err != nil {
			return err
		}
		var tagks []string
		if err := json.Unmarshal(b, &tagks); err != nil {
			return err
		}
		tags := opentsdb.TagSet{}
		for _, t := range tagks {
			tags[t] = "*"
		}
		req := opentsdb.Request{
			Start: startQuery.Unix(),
			End:   endQuery.Unix(),
			Queries: []*opentsdb.Query{
				{
					Aggregator: "sum",
					Metric:     m,
					Tags:       tags,
				},
			},
		}
		rsall, err := req.Query(tsdbHost)
		<-ch
		if err != nil {
			return err
		}
		var rs opentsdb.ResponseSet
		for _, r := range rsall {
			if r.Metric != "" && len(r.DPS) > 0 {
				rs = append(rs, r)
			}
		}
		if len(rs) == 0 {
			log.Println(m, "is empty")
			return nil
		}
		f, err := os.Create(filepath.Join(resPath, m))
		if err != nil {
			return err
		}
		if err := gob.NewEncoder(f).Encode(rs); err != nil {
			return err
		}
		f.Close()
		return nil
	}
	var wg sync.WaitGroup
	wg.Add(len(metrics))
	for mi, m := range metrics {
		ch <- true
		log.Printf("%v of %v: %v", mi+1, len(metrics), m)
		go func(mi int, m string) {
			err := get(mi, m)
			if err != nil {
				log.Fatal(err)
			}
			wg.Done()
		}(mi, m)
	}
	println("waiting")
	wg.Wait()
	end := time.Since(start)
	fmt.Println("took", end)
	return nil
}

func bosunReq(path string) ([]byte, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s%s", bosunHost, path))
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s", b)
	}
	return b, nil
}
