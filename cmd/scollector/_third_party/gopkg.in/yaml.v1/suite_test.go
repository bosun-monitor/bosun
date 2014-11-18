package yaml_test

import (
	. "github.com/bosun-monitor/scollector/_third_party/gopkg.in/check.v1"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})
