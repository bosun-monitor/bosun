package collectors

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
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

func setMeta(m string, s string) error {
	meta_field := strings.Split(s, "=")
	if len(meta_field) == 2 {
		switch meta_field[0] {
		case "meta.Unit":
			unit := meta_field[1]
			metadata.AddMeta(m, nil, "unit", unit, false)
			return nil
		case "meta.Desc":
			desc := meta_field[1]
			metadata.AddMeta(m, nil, "desc", desc, false)
			return nil
		case "meta.Rate":
			rate := meta_field[1]
			metadata.AddMeta(m, nil, "rate", rate, false)
			return nil
		default:
			err := "should be Unit, Desc or Rate"
			return errors.New(err)
		}
	}
	err := "meta is empty"
	return errors.New(err)
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

var setupExternalCommand = func(cmd *exec.Cmd) {}

func (c *ProgramCollector) runProgram(dpchan chan<- *opentsdb.DataPoint) (progError error) {
	cmd := exec.Command(c.Path)
	setupExternalCommand(cmd)
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
		val, err := strconv.ParseFloat(sp[2], 64)
		if err != nil {
			slog.Errorf("bad value in program %s: %s", c.Path, sp[2])
			continue
		}
		if !opentsdb.ValidTag(sp[0]) {
			slog.Errorf("bad metric in program %s: %s", c.Path, sp[0])
		}
		dp := opentsdb.DataPoint{
			Metric:    sp[0],
			Timestamp: ts,
			Value:     val,
			Tags:      opentsdb.TagSet{"host": util.Hostname},
		}
		for _, field := range sp[3:] {
			if strings.HasPrefix(field, "meta.") {
				if err := setMeta(sp[0], field); err != nil {
					slog.Errorf("bad meta in program %s, metric %s: %v: %v", c.Path, sp[0], field, err)
					continue Loop
				} else {
					continue
				}
			}
			tags, err := opentsdb.ParseTags(field)
			if v, ok := tags["host"]; ok && v == "" {
				delete(dp.Tags, "host")
			} else if err != nil {
				slog.Errorf("bad tag in program %s, metric %s: %v: %v", c.Path, sp[0], field, err)
				continue Loop
			} else {
				dp.Tags.Merge(tags)
			}
		}
		dp.Tags = AddTags.Copy().Merge(dp.Tags)
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
