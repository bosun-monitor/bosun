package collectors

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/slog"
	"github.com/bosun-monitor/scollector/opentsdb"
	"github.com/bosun-monitor/scollector/util"
)

type ProgramCollector struct {
	Path     string
	Interval time.Duration
}

func InitPrograms(cpath string) {
	cdir, err := os.Open(cpath)
	if err != nil {
		slog.Infoln(err)
		return
	}
	idirs, err := cdir.Readdir(0)
	if err != nil {
		slog.Infoln(err)
		return
	}
	for _, idir := range idirs {
		i, err := strconv.Atoi(idir.Name())
		if err != nil || i < 0 {
			slog.Infoln("invalid collector folder name:", idir.Name())
			continue
		}
		interval := time.Second * time.Duration(i)
		dir, err := os.Open(filepath.Join(cdir.Name(), idir.Name()))
		if err != nil {
			slog.Infoln(err)
			continue
		}
		files, err := dir.Readdir(0)
		if err != nil {
			slog.Infoln(err)
			continue
		}
		for _, file := range files {
			if !isExecutable(file) {
				continue
			}
			collectors = append(collectors, &ProgramCollector{
				Path:     filepath.Join(dir.Name(), file.Name()),
				Interval: interval,
			})
		}
	}
}

func isExecutable(f os.FileInfo) bool {
	switch runtime.GOOS {
	case "windows":
		exts := strings.Split(os.Getenv("PATHEXT"), ";")
		fileExt := filepath.Ext(strings.ToUpper(f.Name()))
		for _, ext := range exts {
			if filepath.Ext(strings.ToUpper(ext)) == fileExt {
				return true
			}
		}
		return false
	default:
		return f.Mode()&0111 != 0
	}
}

func (c *ProgramCollector) Run(dpchan chan<- *opentsdb.DataPoint) {
	if c.Interval == 0 {
		for {
			next := time.After(DefaultFreq)
			if err := c.runProgram(dpchan); err != nil {
				slog.Infoln(err)
			}
			<-next
			slog.Infoln("restarting", c.Path)
		}
	} else {
		for {
			next := time.After(c.Interval)
			c.runProgram(dpchan)
			<-next
		}
	}
}

func (c *ProgramCollector) Init() {
}

func (c *ProgramCollector) runProgram(dpchan chan<- *opentsdb.DataPoint) (progError error) {
	cmd := exec.Command(c.Path)
	pr, pw := io.Pipe()
	s := bufio.NewScanner(pr)
	cmd.Stdout = pw
	er, ew := io.Pipe()
	cmd.Stderr = ew
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		progError = cmd.Wait()
		pw.Close()
		ew.Close()
	}()
	go func() {
		es := bufio.NewScanner(er)
		for es.Scan() {
			line := strings.TrimSpace(es.Text())
			slog.Error(line)
		}
	}()
Loop:
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		sp := strings.Fields(line)
		if len(sp) < 3 {
			slog.Errorf("bad line in program %s: %s", c.Path, line)
			continue
		}
		ts, err := strconv.ParseInt(sp[1], 10, 64)
		if err != nil {
			slog.Errorf("bad timestamp in program %s: %s", c.Path, sp[1])
			continue
		}
		val, err := strconv.ParseInt(sp[2], 10, 64)
		if err != nil {
			slog.Errorf("bad value in program %s: %s", c.Path, sp[2])
			continue
		}
		dp := opentsdb.DataPoint{
			Metric:    sp[0],
			Timestamp: ts,
			Value:     val,
			Tags:      opentsdb.TagSet{"host": util.Hostname},
		}
		for _, tag := range sp[3:] {
			tags, err := opentsdb.ParseTags(tag)
			if v, ok := tags["host"]; ok && v == "" {
				delete(dp.Tags, "host")
			} else if err != nil {
				slog.Errorf("bad tag in program %s, metric %s: %v", c.Path, sp[0], tag)
				continue Loop
			} else {
				dp.Tags.Merge(tags)
			}
		}
		dpchan <- &dp
	}
	if err := s.Err(); err != nil {
		return err
	}
	return
}

func (c *ProgramCollector) Name() string {
	return c.Path
}
