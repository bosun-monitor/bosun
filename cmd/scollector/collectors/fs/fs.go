package fs

// filesystems information

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

const procFilesystems = "/proc/filesystems"

// PseudoFS is a mapping between filesystem type and pseudoFS
type PseudoFS map[string]bool

// IsPseudo tells if a filesystem type is a pseudoFS
func (pseudoFS PseudoFS) IsPseudo(name string) bool {
	if res, ok := pseudoFS[name]; ok {
		return res
	}
	// drop fs types not in /proc/filesystems
	return true
}

// GetPseudoFS returns mapping between filesystem to isPseudo()
// it is used to cache the results during a single run
func GetPseudoFS() (fs PseudoFS, err error) {
	f, err := os.Open(procFilesystems)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return getPseudoFS(f)
}

func getPseudoFS(r io.Reader) (fs PseudoFS, err error) {
	fs = make(PseudoFS)
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		name, isPseudoFS := parseFSLine(scan.Text())
		if name != "" {
			fs[name] = isPseudoFS
		}
	}
	if err = scan.Err(); err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return fs, errors.New(procFilesystems + " is empty")
	}
	return
}

func parseFSLine(line string) (name string, pseudoFS bool) {
	ss := strings.Split(line, "\t")
	// don't go further if line is not the way we want it
	if len(ss) != 2 {
		return "", true
	}
	if ss[0] != "nodev" {
		return ss[1], false
	}
	return ss[1], true
}
