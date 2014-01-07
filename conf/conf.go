package conf

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func Parse(name string, r io.Reader) (*Conf, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	l := lex(name, string(b))
	c := Conf{
		Global:   make(Section),
		Sections: make(map[string]Section),
	}
	section := c.Global
	state := stateStart
	var i item
	var varname string
	parseError := func(reason string) error {
		return fmt.Errorf("expr: %s:%d: %s", name, l.lineNumber(), reason)
	}
Loop:
	for i = range l.items {
		if i.typ == itemError {
			return nil, parseError(i.val)
		}
		switch state {
		case stateStart:
			switch i.typ {
			case itemLeftDelim:
				state = stateSection
			case itemIdentifier:
				state = stateEqual
				varname = i.val
			case itemEOF:
				break Loop
			default:
				return nil, parseError("expected [ or varname]")
			}
		case stateEqual:
			switch i.typ {
			case itemEqual:
				state = stateValue
			default:
				return nil, parseError("expected =")
			}
		case stateValue:
			switch i.typ {
			case itemString, itemRawString:
				section[varname] = i.val
				state = stateStart
			default:
				return nil, parseError("expected string")
			}
		case stateSection:
			switch i.typ {
			case itemIdentifier:
				section = make(Section)
				c.Sections[i.val] = section
				state = stateRightDelim
			}
		case stateRightDelim:
			switch i.typ {
			case itemRightDelim:
				state = stateStart
			default:
				return nil, parseError("expected ]")
			}
		default:
			return nil, parseError("bad state")
		}
	}
	return &c, nil
}

const (
	stateStart = iota
	stateSection
	stateEqual
	stateRightDelim
	stateValue
)

func ParseFile(fname string) (*Conf, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	return Parse(fname, f)
}

type Conf struct {
	Global   Section
	Sections map[string]Section
}

func (c *Conf) String() string {
	r := c.Global.String()
	for k, v := range c.Sections {
		r += fmt.Sprintf("[%v]\n%v", k, v)
	}
	return r
}

type Section map[string]string

func (s Section) String() string {
	r := ""
	for k, v := range s {
		r += fmt.Sprintf("%v = %v\n", k, v)
	}
	return r
}

func (s Section) Get(key, fallback string) string {
	if v, ok := s[key]; ok {
		return v
	}
	return fallback
}
