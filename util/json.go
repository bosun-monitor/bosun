package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
)

// MarshalGzipJson encodes and GZips the given JSON data into a `[]byte`
func MarshalGzipJson(data interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	g := gzip.NewWriter(buf)
	enc := json.NewEncoder(g)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	if err := g.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalGzipJson unzips and decodes the given JSON data from a `[]byte` into the given struct
func UnmarshalGzipJson(b []byte, dst interface{}) error {
	r := bytes.NewReader(b)
	g, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(g)
	return dec.Decode(dst)
}
