package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func ReadLastLines(filepath string, count int, offset int64) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	if size < offset {
		offset = size
	}

	buf := make([]byte, offset)
	if _, err := f.ReadAt(buf, size-offset); err != nil && err != io.EOF {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(buf[:])))

	scanner.Split(bufio.ScanLines)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	to := len(lines)
	from := len(lines) - count

	if from < 0 {
		from = 0
	}

	return lines[from:to], nil
}

func ReadLastLine(filepath string, offset int64) (string, error) {
	res, err := ReadLastLines(filepath, 1, offset)

	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		err := fmt.Errorf("wrong slice size")
		return "", err
	}
	return res[0], nil
}
