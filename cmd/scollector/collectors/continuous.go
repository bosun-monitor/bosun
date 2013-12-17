package collectors

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/tcollector/opentsdb"
)

type ContinuousCollector struct {
	Path     string
	Interval time.Duration
}

func InitContinuous(cpath string) {
	cdir, err := os.Open(cpath)
	if err != nil {
		l.Println(err)
		return
	}
	idirs, err := cdir.Readdir(0)
	if err != nil {
		l.Println(err)
		return
	}
	for _, idir := range idirs {
		i, err := strconv.Atoi(idir.Name())
		if err != nil {
			l.Println(err)
			continue
		}
		interval := time.Second * time.Duration(i)
		dir, err := os.Open(filepath.Join(cdir.Name(), idir.Name()))
		if err != nil {
			l.Println(err)
			continue
		}
		files, err := dir.Readdir(0)
		if err != nil {
			l.Println(err)
			continue
		}
		for _, file := range files {
			collectors = append(collectors, &ContinuousCollector{
				Path:     filepath.Join(dir.Name(), file.Name()),
				Interval: interval,
			})
		}
	}
}

func (c *ContinuousCollector) Run(dpchan chan<- *opentsdb.DataPoint) {
	cmd := exec.Command(c.Path)
	pr, pw := io.Pipe()
	s := bufio.NewScanner(pr)
	cmd.Stdout = pw
	err := cmd.Start()
	if err != nil {
		l.Println(err)
		return
	}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		sp := strings.Fields(line)
		if len(sp) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(sp[1], 10, 64)
		if err != nil {
			continue
		}
		dp := opentsdb.DataPoint{
			Metric:    sp[0],
			Timestamp: ts,
			Value:     sp[2],
			Tags:      opentsdb.TagSet{"host": host},
		}
		for _, tag := range sp[3:] {
			tsp := strings.Split(tag, "=")
			if len(tsp) != 2 {
				l.Fatal("bad tag", tsp)
				continue
			}
			dp.Tags[tsp[0]] = tsp[1]
		}
		dpchan <- &dp
	}
	if err := s.Err(); err != nil {
		l.Println(err)
	}
}

func (c *ContinuousCollector) Name() string {
	return c.Path
}
