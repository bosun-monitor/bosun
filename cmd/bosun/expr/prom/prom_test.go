package prom

import (
	"strings"
	"testing"

	"github.com/prometheus/prometheus/promql"
)

type queryTemplateTest struct {
	title string
	queryTemplateData
	expect string
}

var queryTemplateTests = []queryTemplateTest{
	{
		"minimal query",
		queryTemplateData{
			Metric: "up",
			AgFunc: "sum",
		},
		"sum( up) by (  )",
	},
	{
		"query with tags",
		queryTemplateData{
			Metric: "up",
			AgFunc: "sum",
			Tags:   "namespace,pod",
		},
		"sum( up) by ( namespace,pod )",
	},
	{
		"query with tags and filter",
		queryTemplateData{
			Metric: "up",
			AgFunc: "sum",
			Tags:   "namespace,pod",
			Filter: `service !~ "kubl.*"`,
		},
		`sum( up {service !~ "kubl.*"} ) by ( namespace,pod )`,
	},
	{
		"minimal rate query",
		queryTemplateData{
			Metric:       "up",
			AgFunc:       "sum",
			RateDuration: "5m",
		},
		`sum(rate( up [5m] )) by (  )`,
	},
	{
		"rate query with tags and filter",
		queryTemplateData{
			Metric:       "up",
			AgFunc:       "sum",
			Tags:         "namespace,pod",
			Filter:       ` service !~ "kubl.*" `,
			RateDuration: "5m",
		},
		`sum(rate( up { service !~ "kubl.*" }  [5m] )) by ( namespace,pod )`,
	},
}

func TestPromQueryTemplate(t *testing.T) {
	for _, qTest := range queryTemplateTests {
		out, err := qTest.RenderString()
		if err != nil {
			t.Errorf("error rendering template for test query %v: %v", qTest.title, err)
			continue
		}
		trimmedOut := strings.TrimSpace(out)
		if trimmedOut != qTest.expect {
			t.Errorf("unexpected output for test query %v: got ```%v``` want ```%v```", qTest.title, trimmedOut, qTest.expect)
		}
		_, err = promql.ParseExpr(trimmedOut)
		if err != nil {
			t.Errorf("failed to parse output of for test query %v as valid promql: %v", qTest, err)
		}
	}
}
