package util

import (
	"testing"

	"bosun.org/_third_party/github.com/stretchr/testify/assert"
)

func TestReadLastLinesZeroCount(t *testing.T) {
	fname := "test/lastlines.txt"

	res, err := ReadLastLines(fname, 0, 1024)
	assert.Nil(t, err)
	assert.Equal(t, res, []string{})

}

func TestReadLastLinesNormalCount(t *testing.T) {
	fname := "test/lastlines.txt"

	res, err := ReadLastLines(fname, 3, 1024)
	assert.Nil(t, err)
	assert.Equal(t, res, []string{"three", "four", "five"})
}

func TestReadLastLinesExcessCount(t *testing.T) {
	fname := "test/lastlines.txt"

	res, err := ReadLastLines(fname, 6, 1024)
	assert.Nil(t, err)
	assert.Equal(t, res, []string{"one", "two", "three", "four", "five"})
}

func TestReadLastLinesInsufficientBuffer(t *testing.T) {
	fname := "test/lastlines.txt"

	res, err := ReadLastLines(fname, 1, 1)
	assert.Nil(t, err)
	assert.Equal(t, res, []string{""})
}

func TestReadLastLinesEmptyFile(t *testing.T) {
	fname := "test/empty.txt"

	res, err := ReadLastLines(fname, 1, 1024)
	assert.Nil(t, err)
	assert.Equal(t, res, []string{})
}

func TestReadLastLine(t *testing.T) {
	fname := "test/lastlines.txt"

	res, err := ReadLastLine(fname, 1024)
	assert.Nil(t, err)
	assert.Equal(t, res, "five")
}
