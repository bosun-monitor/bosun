package fs

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"bosun.org/slog"
)

// some benchmark on linux platform
// shows /proc/filesystems mapping speed changes

// BenchmarkOldIsPseudoFS-2          100000             14176 ns/op
// BenchmarkIsPseudoFS-2           50000000                27 ns/op

func readLine(fname string, line func(string) error) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := line(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func OldIsPseudoFS(name string) (res bool) {
	err := readLine("/proc/filesystems", func(s string) error {
		ss := strings.Split(s, "\t")
		if len(ss) == 2 && ss[1] == name && ss[0] == "nodev" {
			res = true
		}
		return nil
	})
	if err != nil {
		slog.Errorf("can not read '/proc/filesystems': %v", err)
	}
	return
}

func BenchmarkOldIsPseudoFS(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = OldIsPseudoFS("titi")
	}
}

func BenchmarkIsPseudoFS(b *testing.B) {
	fs, _ := GetPseudoFS()
	for i := 0; i < b.N; i++ {
		_ = fs.IsPseudo("titi")
	}
}
