package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
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

	TagOverride
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
		idirname := idir.Name()
		i, err := strconv.Atoi(idirname)
		if err != nil || i < 0 {
			if idirname != "etc" && idirname != "lib" {
				slog.Infoln("invalid collector folder name:", idirname)
			}
			continue
		}
		interval := time.Second * time.Duration(i)
		dir, err := os.Open(filepath.Join(cdir.Name(), idirname))
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
		exts = append(exts, ".PS1")
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

func (c *ProgramCollector) Run(dpchan chan<- *opentsdb.DataPoint, quit <-chan struct{}) {
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
			select {
			case <-next:
			case <-quit:
				return
			}

		}
	}
}

func (c *ProgramCollector) Init() {
}

var setupExternalCommand = func(cmd *exec.Cmd) {}

func (c *ProgramCollector) runProgram(dpchan chan<- *opentsdb.DataPoint) (progError error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(c.Path), ".ps1") {
		cmd = exec.Command("powershell", "-NoProfile", "-NoLogo", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", c.Path)
	} else {
		cmd = exec.Command(c.Path)
	}
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
	for s.Scan() {
		var errs []error
		t := strings.TrimSpace(s.Text())
		if len(t) == 0 {
			continue
		}
		if dp, err := parseTcollectorValue(t); err == nil {
			dpchan <- dp
			continue
		} else {
			errs = append(errs, fmt.Errorf("tcollector: %v", err))
		}
		var dp opentsdb.DataPoint
		if err := json.Unmarshal([]byte(t), &dp); err != nil {
			errs = append(errs, fmt.Errorf("opentsdb.DataPoint: %v", err))
		} else if dp.Valid() {
			if dp.Tags == nil {
				dp.Tags = opentsdb.TagSet{}
			}
			setExternalTags(dp.Tags)
			c.ApplyTagOverrides(dp.Tags)
			dpchan <- &dp
			continue
		} else {
			errs = append(errs, fmt.Errorf("opentsdb.DataPoint: invalid data"))
		}
		var m metadata.Metasend
		if err := json.Unmarshal([]byte(t), &m); err != nil {
			errs = append(errs, fmt.Errorf("metadata.Metasend: %v", err))
		} else {
			if m.Tags == nil {
				m.Tags = opentsdb.TagSet{}
			}
			setExternalTags(m.Tags)
			if m.Value == "" || m.Name == "" || (m.Metric == "" && len(m.Tags) == 0) {
				errs = append(errs, fmt.Errorf("metadata.Metasend: invalid data"))
			} else {
				metadata.AddMeta(m.Metric, m.Tags, m.Name, m.Value, false)
				continue
			}
		}
		slog.Errorf("%s: unparseable line: %s", c.Path, t)
		for _, e := range errs {
			slog.Error(e)
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return
}

// setExternalTags adds and deletes system-level tags to tags. The host
// tag is set to the hostname if unspecified, or removed if present and
// empty. Command line tags (in AddTags) are then added.
func setExternalTags(tags opentsdb.TagSet) {
	if v, ok := tags["host"]; ok && v == "" {
		delete(tags, "host")
	} else if v == "" {
		tags["host"] = util.Hostname
	}
	for k, v := range AddTags {
		if _, ok := tags[k]; !ok {
			tags[k] = v
		}
	}
}

// parseTcollectorValue parses a tcollector-style line into a data point.
func parseTcollectorValue(line string) (*opentsdb.DataPoint, error) {
	sp := strings.Fields(line)
	if len(sp) < 3 {
		return nil, fmt.Errorf("bad line: %s", line)
	}
	ts, err := strconv.ParseInt(sp[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad timestamp: %s", sp[1])
	}
	val, err := strconv.ParseFloat(sp[2], 64)
	if err != nil {
		return nil, fmt.Errorf("bad value: %s", sp[2])
	}
	if !opentsdb.ValidTSDBString(sp[0]) {
		return nil, fmt.Errorf("bad metric: %s", sp[0])
	}
	dp := opentsdb.DataPoint{
		Metric:    sp[0],
		Timestamp: ts,
		Value:     val,
	}
	tags := opentsdb.TagSet{}
	for _, tag := range sp[3:] {
		ts, err := opentsdb.ParseTags(tag)
		if err != nil {
			return nil, fmt.Errorf("bad tag, metric %s: %v: %v", sp[0], tag, err)
		}
		tags.Merge(ts)
	}
	setExternalTags(tags)
	dp.Tags = tags
	return &dp, nil
}

func (c *ProgramCollector) Name() string {
	return c.Path
}
