package escape // import "github.com/influxdata/influxdb/pkg/escape"

import "bytes"

func Bytes(in []byte) []byte {
	for b, esc := range Codes {
		in = bytes.Replace(in, []byte{b}, esc, -1)
	}
	return in
}

func Unescape(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}

	if bytes.IndexByte(in, '\\') == -1 {
		return in
	}

	i := 0
	inLen := len(in)
	var out []byte

	for {
		if i >= inLen {
			break
		}
		if in[i] == '\\' && i+1 < inLen {
			switch in[i+1] {
			case ',':
				out = append(out, ',')
				i += 2
				continue
			case '"':
				out = append(out, '"')
				i += 2
				continue
			case ' ':
				out = append(out, ' ')
				i += 2
				continue
			case '=':
				out = append(out, '=')
				i += 2
				continue
			}
		}
		out = append(out, in[i])
		i += 1
	}
	return out
}
