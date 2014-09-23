package sched

import (
	"fmt"
	"image/color"
	"io"
	"strconv"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	svg "github.com/StackExchange/bosun/_third_party/github.com/ajstarks/svgo"
	"github.com/StackExchange/bosun/_third_party/github.com/bradfitz/slice"
	"github.com/StackExchange/bosun/_third_party/github.com/vdobler/chart"
	"github.com/StackExchange/bosun/_third_party/github.com/vdobler/chart/svgg"
	"github.com/StackExchange/bosun/expr"
)

var chartColors = []color.Color{
	color.NRGBA{0xe4, 0x1a, 0x1c, 0xff},
	color.NRGBA{0x37, 0x7e, 0xb8, 0xff},
	color.NRGBA{0x4d, 0xaf, 0x4a, 0xff},
	color.NRGBA{0x98, 0x4e, 0xa3, 0xff},
	color.NRGBA{0xff, 0x7f, 0x00, 0xff},
	color.NRGBA{0xa6, 0x56, 0x28, 0xff},
	color.NRGBA{0xf7, 0x81, 0xbf, 0xff},
	color.NRGBA{0x99, 0x99, 0x99, 0xff},
}

// Autostyle styles a chart series.
func Autostyle(i int) chart.Style {
	c := chartColors[i%len(chartColors)]
	return chart.Style{
		// 0 uses a default
		SymbolSize: 0.00001,
		LineStyle:  chart.SolidLine,
		LineWidth:  1,
		LineColor:  c,
	}
}

func (s *Schedule) ExprGraph(t miniprofiler.Timer, w io.Writer, res []*expr.Result, q string, now time.Time) error {
	c := chart.ScatterChart{
		Title: fmt.Sprintf("%s - %s", q, now.Format(time.RFC1123)),
	}
	c.XRange.Time = true
	for ri, r := range res {
		rv := r.Value.(expr.Series)
		pts := make([]chart.EPoint, len(rv))
		idx := 0
		for k, v := range rv {
			i, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return err
			}
			//names[idx] = time.Unix(i, 0).Format("02 Jan 15:04")
			pts[idx].X = float64(i)
			pts[idx].Y = float64(v)
			idx++
		}
		slice.Sort(pts, func(i, j int) bool {
			return pts[i].X < pts[j].X
		})
		c.AddData(r.Group.String(), pts, chart.PlotStyleLinesPoints, Autostyle(ri))
	}
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	const width = 800
	const height = 600
	g := svg.New(w)
	g.StartviewUnit(100, 100, "%", 0, 0, width, height)
	g.Rect(0, 0, width, height, "fill: #ffffff")
	sgr := svgg.AddTo(g, 0, 0, width, height, "", 12, white)
	c.Plot(sgr)
	g.End()
	return nil
}
