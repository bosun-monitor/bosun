package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"bosun.org/Godeps/_workspace/src/github.com/ajstarks/svgo"
	"bosun.org/Godeps/_workspace/src/github.com/vdobler/chart"
	"bosun.org/Godeps/_workspace/src/github.com/vdobler/chart/imgg"
	"bosun.org/Godeps/_workspace/src/github.com/vdobler/chart/svgg"
	"bosun.org/Godeps/_workspace/src/github.com/vdobler/chart/txtg"
)

var (
	data1  = []float64{15e-7, 30e-7, 35e-7, 50e-7, 70e-7, 75e-7, 80e-7, 32e-7, 35e-7, 70e-7, 65e-7}
	data10 = []float64{34567, 35432, 37888, 39991, 40566, 42123, 44678}

	data2 = []float64{10e-7, 11e-7, 12e-7, 22e-7, 25e-7, 33e-7}
	data3 = []float64{50e-7, 55e-7, 55e-7, 60e-7, 50e-7, 65e-7, 60e-7, 65e-7, 55e-7, 50e-7}
)

var Background = color.RGBA{0xff, 0xff, 0xff, 0xff}

// -------------------------------------------------------------------------
// Dumper

// Dumper helps saving plots of size WxH in a NxM grid layout
// in several formats
type Dumper struct {
	N, M, W, H, Cnt           int
	S                         *svg.SVG
	I                         *image.RGBA
	svgFile, imgFile, txtFile *os.File
}

func NewDumper(name string, n, m, w, h int) *Dumper {
	var err error
	dumper := Dumper{N: n, M: m, W: w, H: h}

	dumper.svgFile, err = os.Create(name + ".svg")
	if err != nil {
		panic(err)
	}
	dumper.S = svg.New(dumper.svgFile)
	dumper.S.Start(n*w, m*h)
	dumper.S.Title(name)
	dumper.S.Rect(0, 0, n*w, m*h, "fill: #ffffff")

	dumper.imgFile, err = os.Create(name + ".png")
	if err != nil {
		panic(err)
	}
	dumper.I = image.NewRGBA(image.Rect(0, 0, n*w, m*h))
	bg := image.NewUniform(color.RGBA{0xff, 0xff, 0xff, 0xff})
	draw.Draw(dumper.I, dumper.I.Bounds(), bg, image.ZP, draw.Src)

	dumper.txtFile, err = os.Create(name + ".txt")
	if err != nil {
		panic(err)
	}

	return &dumper
}
func (d *Dumper) Close() {
	png.Encode(d.imgFile, d.I)
	d.imgFile.Close()

	d.S.End()
	d.svgFile.Close()

	d.txtFile.Close()
}

func (d *Dumper) Plot(c chart.Chart) {
	row, col := d.Cnt/d.N, d.Cnt%d.N

	igr := imgg.AddTo(d.I, col*d.W, row*d.H, d.W, d.H, color.RGBA{0xff, 0xff, 0xff, 0xff}, nil, nil)
	c.Plot(igr)

	sgr := svgg.AddTo(d.S, col*d.W, row*d.H, d.W, d.H, "", 12, color.RGBA{0xff, 0xff, 0xff, 0xff})
	c.Plot(sgr)

	tgr := txtg.New(100, 30)
	c.Plot(tgr)
	d.txtFile.Write([]byte(tgr.String() + "\n\n\n"))

	d.Cnt++

}

// -------------------------------------------------------------------------
// Individual Charts

//
// Some sample strip charts
//
func stripChart() {
	dumper := NewDumper("xstrip1", 2, 2, 400, 300)
	defer dumper.Close()

	c := chart.StripChart{}

	c.AddData("Sample A", data1, chart.Style{})
	c.AddData("Sample B", data2, chart.Style{})
	c.AddData("Sample C", data3, chart.Style{})
	c.Title = "Sample Strip Chart (no Jitter)"
	c.XRange.Label = "X - Axis"
	c.Key.Pos = "icr"
	dumper.Plot(&c)

	c.Jitter = true
	c.Title = "Sample Strip Chart (with Jitter)"
	dumper.Plot(&c)

	c.Key.Hide = true
	dumper.Plot(&c)

	c.Jitter = false
	c.Title = "Sample Strip Chart (no Jitter)"
	dumper.Plot(&c)
}

//
// All different key styles
//
func keyStyles() {
	dumper := NewDumper("xkey", 6, 6, 400, 300)
	defer dumper.Close()

	p := chart.ScatterChart{Title: "Key Placement"}
	p.XRange.TicSetting.Mirror, p.YRange.TicSetting.Mirror = 1, 1
	p.XRange.MinMode.Fixed, p.XRange.MaxMode.Fixed = true, true
	p.XRange.MinMode.Value, p.XRange.MaxMode.Value = -5, 5
	p.XRange.Min, p.XRange.Max = -5, 5
	p.XRange.TicSetting.Delta = 2

	p.YRange.MinMode.Fixed, p.YRange.MaxMode.Fixed = true, true
	p.YRange.MinMode.Value, p.YRange.MaxMode.Value = -5, 5
	p.YRange.Min, p.YRange.Max = -5, 5
	p.YRange.TicSetting.Delta = 3

	p.AddFunc("Sin", func(x float64) float64 { return math.Sin(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0xa0, 0x00, 0x00, 0xff}, LineWidth: 1, LineStyle: 1})
	p.AddFunc("Cos", func(x float64) float64 { return math.Cos(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0x00, 0xa0, 0x00, 0xff}, LineWidth: 1, LineStyle: 1})
	p.AddFunc("Tan", func(x float64) float64 { return math.Tan(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0x00, 0x00, 0xa0, 0xff}, LineWidth: 1, LineStyle: 1})

	for _, pos := range []string{"itl", "itc", "itr", "icl", "icc", "icr", "ibl", "ibc", "ibr",
		"otl", "otc", "otr", "olt", "olc", "olb", "obl", "obc", "obr", "ort", "orc", "orb"} {
		p.Key.Pos = pos
		p.Title = "Key Placement: " + pos
		dumper.Plot(&p)
	}

	p.Key.Pos = "itl"
	p.AddFunc("Log", func(x float64) float64 { return math.Log(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0xff, 0x60, 0x60, 0xff}, LineWidth: 1, LineStyle: 1})
	p.AddFunc("Exp", func(x float64) float64 { return math.Exp(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0x60, 0xff, 0x60, 0xff}, LineWidth: 1, LineStyle: 1})
	p.AddFunc("Atan", func(x float64) float64 { return math.Atan(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0x60, 0x60, 0xff, 0xff}, LineWidth: 1, LineStyle: 1})
	p.AddFunc("Y1", func(x float64) float64 { return math.Y1(x) }, chart.PlotStyleLines,
		chart.Style{LineColor: color.NRGBA{0xd0, 0xd0, 0x00, 0xff}, LineWidth: 1, LineStyle: 1})

	for _, cols := range []int{-4, -3, -2, -1, 0, 1, 2, 3, 4} {
		p.Key.Cols = cols
		p.Title = fmt.Sprintf("Key Cols: %d", cols)
		dumper.Plot(&p)
	}
}

//
// Scatter plots with different tic/grid settings
//
func scatterTics() {
	dumper := NewDumper("xstrip1", 3, 3, 400, 300)
	defer dumper.Close()

	p := chart.ScatterChart{Title: "Sample Scatter Chart"}
	p.AddDataPair("Sample A", data10, data1, chart.PlotStylePoints, chart.Style{})
	p.XRange.TicSetting.Delta = 5000
	p.XRange.Label = "X - Value"
	p.YRange.Label = "Y - Value"
	dumper.Plot(&p)

	p.XRange.TicSetting.Hide, p.YRange.TicSetting.Hide = true, true
	dumper.Plot(&p)

	p.YRange.TicSetting.Hide = false
	p.XRange.TicSetting.Grid, p.YRange.TicSetting.Grid = chart.GridLines, chart.GridLines
	dumper.Plot(&p)

	p.XRange.TicSetting.Hide, p.YRange.TicSetting.Hide = false, false
	p.XRange.TicSetting.Mirror, p.YRange.TicSetting.Mirror = 1, 2
	dumper.Plot(&p)

	c := chart.ScatterChart{Title: "Own tics"}
	c.XRange.Fixed(0, 4*math.Pi, math.Pi)
	c.YRange.Fixed(-1.25, 1.25, 0.5)
	c.XRange.TicSetting.Format = func(f float64) string {
		w := int(180*f/math.Pi + 0.5)
		return fmt.Sprintf("%d°", w)
	}
	c.AddFunc("Sin(x)", func(x float64) float64 { return math.Sin(x) }, chart.PlotStyleLines,
		chart.Style{Symbol: '@', LineWidth: 2, LineColor: color.NRGBA{0x00, 0x00, 0xcc, 0xff}, LineStyle: 0})
	c.AddFunc("Cos(x)", func(x float64) float64 { return math.Cos(x) }, chart.PlotStyleLines,
		chart.Style{Symbol: '%', LineWidth: 2, LineColor: color.NRGBA{0x00, 0xcc, 0x00, 0xff}, LineStyle: 0})
	dumper.Plot(&c)

	c.Title = "Tic Variants"
	c.XRange.TicSetting.Tics = 1
	c.YRange.TicSetting.Tics = 2
	dumper.Plot(&c)

	c.Title = "Blocked Grid"
	c.XRange.TicSetting.Tics = 1
	c.YRange.TicSetting.Tics = 1
	c.XRange.TicSetting.Mirror, c.YRange.TicSetting.Mirror = 1, 1
	c.XRange.TicSetting.Grid = chart.GridBlocks
	c.YRange.TicSetting.Grid = chart.GridBlocks
	dumper.Plot(&c)
}

//
// Full fletched scatter plots
//
func scatterChart() {
	dumper := NewDumper("xscatter2", 1, 1, 800, 600)
	defer dumper.Close()

	pl := chart.ScatterChart{Title: "Scatter + Lines"}
	pl.XRange.Label, pl.YRange.Label = "X - Value", "Y - Value"
	pl.Key.Pos = "itl"
	// pl.XRange.TicSetting.Delta = 5
	pl.XRange.TicSetting.Grid = 1
	x := []float64{-4, -3.3, -1.8, -1, 0.2, 0.8, 1.8, 3.1, 4, 5.3, 6, 7, 8, 9}
	y := []float64{22, 18, -3, 0, 0.5, 2, 45, 12, 16.5, 24, 30, 55, 60, 70}
	pl.AddDataPair("Data", x, y, chart.PlotStyleLinesPoints,
		chart.Style{Symbol: '#', SymbolColor: color.NRGBA{0x00, 0x00, 0xff, 0xff}, LineStyle: chart.SolidLine})
	last := len(pl.Data) - 1
	pl.Data[last].Samples[6].DeltaX = 2.5
	pl.Data[last].Samples[6].OffX = 0.5
	pl.Data[last].Samples[6].DeltaY = 16
	pl.Data[last].Samples[6].OffY = 2

	pl.AddData("Points", []chart.EPoint{chart.EPoint{-4, 40, 0, 0, 0, 0}, chart.EPoint{-3, 45, 0, 0, 0, 0},
		chart.EPoint{-2, 35, 0, 0, 0, 0}}, chart.PlotStylePoints,
		chart.Style{Symbol: '0', SymbolColor: color.NRGBA{0xff, 0x00, 0xff, 0xff}})
	pl.AddFunc("Theory", func(x float64) float64 {
		if x > 5.25 && x < 5.75 {
			return 75
		}
		if x > 7.25 && x < 7.75 {
			return 500
		}
		return x * x
	}, chart.PlotStyleLines,
		chart.Style{Symbol: '%', LineWidth: 2, LineColor: color.NRGBA{0xa0, 0x00, 0x00, 0xff}, LineStyle: chart.DashDotDotLine})
	pl.AddFunc("30", func(x float64) float64 { return 30 }, chart.PlotStyleLines,
		chart.Style{Symbol: '+', LineWidth: 1, LineColor: color.NRGBA{0x00, 0xa0, 0x00, 0xff}, LineStyle: 1})
	pl.AddFunc("", func(x float64) float64 { return 7 }, chart.PlotStyleLines,
		chart.Style{Symbol: '@', LineWidth: 1, LineColor: color.NRGBA{0x00, 0x00, 0xa0, 0xff}, LineStyle: 1})

	pl.XRange.ShowZero = true
	pl.XRange.TicSetting.Mirror = 1
	pl.YRange.TicSetting.Mirror = 1
	pl.XRange.TicSetting.Grid = 1
	pl.XRange.Label = "X-Range"
	pl.YRange.Label = "Y-Range"
	pl.Key.Cols = 2
	pl.Key.Pos = "orb"

	dumper.Plot(&pl)
}

//
// Function plots with fancy clippings
//
func functionPlots() {
	dumper := NewDumper("xscatter3", 2, 1, 500, 400)
	defer dumper.Close()

	p := chart.ScatterChart{Title: "Functions"}
	p.XRange.Label, p.YRange.Label = "X - Value", "Y - Value"
	p.Key.Pos = "ibl"
	p.XRange.MinMode.Fixed, p.XRange.MaxMode.Fixed = true, true
	p.XRange.MinMode.Value, p.XRange.MaxMode.Value = -10, 10
	p.YRange.MinMode.Fixed, p.YRange.MaxMode.Fixed = true, true
	p.YRange.MinMode.Value, p.YRange.MaxMode.Value = -10, 10

	p.XRange.TicSetting.Delta = 2
	p.YRange.TicSetting.Delta = 5
	p.XRange.TicSetting.Mirror = 1
	p.YRange.TicSetting.Mirror = 1

	p.AddFunc("i+n", func(x float64) float64 {
		if x > -7 && x < -5 {
			return math.Inf(-1)
		} else if x > -1.5 && x < 1.5 {
			return math.NaN()
		} else if x > 5 && x < 7 {
			return math.Inf(1)
		}
		return -0.75 * x
	},
		chart.PlotStyleLines, chart.Style{Symbol: 'o', LineWidth: 2, LineColor: color.NRGBA{0xa0, 0x00, 0x00, 0xff}, LineStyle: 1})
	p.AddFunc("sin", func(x float64) float64 { return 13 * math.Sin(x) }, chart.PlotStyleLines,
		chart.Style{Symbol: '#', LineWidth: 1, LineColor: color.NRGBA{0x00, 0x00, 0xa0, 0xff}, LineStyle: 1})
	p.AddFunc("2x", func(x float64) float64 { return 2 * x }, chart.PlotStyleLines,
		chart.Style{Symbol: 'X', LineWidth: 1, LineColor: color.NRGBA{0x00, 0xa0, 0x00, 0xff}, LineStyle: 1})

	dumper.Plot(&p)

	p = chart.ScatterChart{Title: "Functions"}
	p.Key.Hide = true
	p.XRange.MinMode.Fixed, p.XRange.MaxMode.Fixed = true, true
	p.XRange.MinMode.Value, p.XRange.MaxMode.Value = -2, 2
	p.YRange.MinMode.Fixed, p.YRange.MaxMode.Fixed = true, true
	p.YRange.MinMode.Value, p.YRange.MaxMode.Value = -2, 2
	p.XRange.TicSetting.Delta = 1
	p.YRange.TicSetting.Delta = 1
	p.XRange.TicSetting.Mirror = 1
	p.YRange.TicSetting.Mirror = 1
	p.NSamples = 5
	p.AddFunc("10x", func(x float64) float64 { return 10 * x }, chart.PlotStyleLines,
		chart.Style{Symbol: 'o', LineWidth: 2, LineColor: color.NRGBA{0x00, 0xa0, 0x00, 0xff}, LineStyle: 1})
	dumper.Plot(&p)
}

//
// Autoscaling
//
func autoscale() {
	dumper := NewDumper("xautoscale", 2, 2, 600, 400)
	defer dumper.Close()

	N := 200
	points := make([]chart.EPoint, N)
	for i := 0; i < N-1; i++ {
		points[i].X = rand.Float64()*10000 - 5000 // Full range is [-5000:5000]
		points[i].Y = rand.Float64()*10000 - 5000 // Full range is [-5000:5000]
		points[i].DeltaX = rand.Float64() * 400
		points[i].DeltaY = rand.Float64() * 400
	}
	points[N-1].X = -650
	points[N-1].Y = -2150
	points[N-1].DeltaX = 400
	points[N-1].DeltaY = 400
	points[N-1].OffX = 100
	points[N-1].OffY = -150

	s := chart.ScatterChart{Title: "Full Autoscaling"}
	s.Key.Hide = true
	s.XRange.TicSetting.Mirror = 1
	s.YRange.TicSetting.Mirror = 1
	s.AddData("Data", points, chart.PlotStylePoints, chart.Style{Symbol: 'o', SymbolColor: color.NRGBA{0x00, 0xee, 0x00, 0xff}})
	dumper.Plot(&s)

	s = chart.ScatterChart{Title: "Xmin: -1850, Xmax clipped to [500:900]"}
	s.Key.Hide = true
	s.XRange.TicSetting.Mirror = 1
	s.YRange.TicSetting.Mirror = 1
	s.XRange.MinMode.Fixed, s.XRange.MinMode.Value = true, -1850
	s.XRange.MaxMode.Constrained = true
	s.XRange.MaxMode.Lower, s.XRange.MaxMode.Upper = 500, 900

	s.AddData("Data", points, chart.PlotStylePoints, chart.Style{Symbol: '0', SymbolColor: color.NRGBA{0xee, 0x00, 0x00, 0xff}})
	dumper.Plot(&s)

	s = chart.ScatterChart{Title: "Xmin: -1850, Ymax clipped to [9000:11000]"}
	s.Key.Hide = true
	s.XRange.TicSetting.Mirror = 1
	s.YRange.TicSetting.Mirror = 1
	s.XRange.MinMode.Fixed, s.XRange.MinMode.Value = true, -1850
	s.YRange.MaxMode.Constrained = true
	s.YRange.MaxMode.Lower, s.YRange.MaxMode.Upper = 9000, 11000

	s.AddData("Data", points, chart.PlotStylePoints, chart.Style{Symbol: '0', SymbolColor: color.NRGBA{0x00, 0x00, 0xee, 0xff}})
	dumper.Plot(&s)

	s = chart.ScatterChart{Title: "Tiny fraction"}
	s.Key.Hide = true
	s.XRange.TicSetting.Mirror = 1
	s.YRange.TicSetting.Mirror = 1

	s.YRange.MinMode.Constrained = true
	s.YRange.MinMode.Lower, s.YRange.MinMode.Upper = -2250, -2050
	s.YRange.MaxMode.Constrained = true
	s.YRange.MaxMode.Lower, s.YRange.MaxMode.Upper = -1950, -1700

	s.XRange.MinMode.Constrained = true
	s.XRange.MinMode.Lower, s.XRange.MinMode.Upper = -900, -800
	s.XRange.MaxMode.Constrained = true
	s.XRange.MaxMode.Lower, s.XRange.MaxMode.Upper = -850, -650

	s.AddData("Data", points, chart.PlotStylePoints, chart.Style{Symbol: '0', SymbolColor: color.NRGBA{0xee, 0xcc, 0x00, 0xff}})
	dumper.Plot(&s)
}

//
// Box Charts
//
func boxChart() {
	dumper := NewDumper("xbox1", 2, 2, 400, 300)
	defer dumper.Close()

	p := chart.BoxChart{Title: "Box Chart"}
	p.XRange.Label, p.YRange.Label = "Value", "Count"
	for x := 10; x <= 50; x += 5 {
		points := make([]float64, 70)
		a := rand.Float64() * 10
		v := rand.Float64()*5 + 2
		for i := 0; i < len(points); i++ {
			x := rand.NormFloat64()*v + a
			points[i] = x
		}
		p.AddSet(float64(x), points, true)
	}

	p.NextDataSet("Sample B", chart.Style{Symbol: 'x', LineColor: color.NRGBA{0x00, 0xc0, 0x00, 0xff}, LineWidth: 1, LineStyle: chart.SolidLine})
	for x := 12; x <= 50; x += 10 {
		points := make([]float64, 60)
		a := rand.Float64()*15 + 30
		v := rand.Float64()*5 + 2
		for i := 0; i < len(points); i++ {
			x := rand.NormFloat64()*v + a
			points[i] = x
		}
		p.AddSet(float64(x), points, true)
	}
	dumper.Plot(&p)

	p = chart.BoxChart{Title: "Categorical Box Chart"}
	p.XRange.Label, p.YRange.Label = "Population", "Count"
	p.XRange.Fixed(-1, 3, 1)
	p.XRange.Category = []string{"Rural", "Urban", "Island"}

	p.NextDataSet("", chart.Style{Symbol: '%', LineColor: color.NRGBA{0x00, 0x00, 0xcc, 0xff}, LineWidth: 1, LineStyle: chart.SolidLine})
	p.AddSet(0, bigauss(100, 0, 5, 10, 0, 0, 0, 50), true)
	p.AddSet(1, bigauss(100, 25, 5, 5, 2, 25, 0, 50), true)
	p.AddSet(2, bigauss(50, 50, 4, 8, 4, 16, 0, 50), true)
	dumper.Plot(&p)

}

// gaussian distribution with n samples, stddev of s, offset of a, forced to [l,u]
func gauss(n int, s, a, l, u float64) []float64 {
	points := make([]float64, n)
	for i := 0; i < len(points); i++ {
		x := rand.NormFloat64()*s + a
		if x < l {
			x = l
		} else if x > u {
			x = u
		}
		points[i] = x
	}
	return points
}

// bigaussian distribution with n samples, stddev of s, offset of a, clipped to [l,u]
func bigauss(n1, n2 int, s1, a1, s2, a2, l, u float64) []float64 {
	points := make([]float64, n1+n2)
	for i := 0; i < n1; i++ {
		x := rand.NormFloat64()*s1 + a1
		for x < l || x > u {
			x = rand.NormFloat64()*s1 + a1
		}
		points[i] = x
	}
	for i := n1; i < n1+n2; i++ {
		x := rand.NormFloat64()*s2 + a2
		for x < l || x > u {
			x = rand.NormFloat64()*s2 + a2
		}
		points[i] = x
	}
	return points
}

func kernels() {
	dumper := NewDumper("xkernels", 1, 1, 600, 400)
	defer dumper.Close()

	p := chart.ScatterChart{Title: "Kernels"}
	p.XRange.Label, p.YRange.Label = "u", "K(u)"
	p.XRange.MinMode.Fixed, p.XRange.MaxMode.Fixed = true, true
	p.XRange.MinMode.Value, p.XRange.MaxMode.Value = -2, 2
	p.YRange.MinMode.Fixed, p.YRange.MaxMode.Fixed = true, true
	p.YRange.MinMode.Value, p.YRange.MaxMode.Value = -0.1, 1.1

	p.XRange.TicSetting.Delta = 1
	p.YRange.TicSetting.Delta = 0.2
	p.XRange.TicSetting.Mirror = 1
	p.YRange.TicSetting.Mirror = 1

	p.AddFunc("Bisquare", chart.BisquareKernel,
		chart.PlotStyleLines, chart.Style{Symbol: 'o', LineWidth: 1, LineColor: color.NRGBA{0xa0, 0x00, 0x00, 0xff}, LineStyle: 1})
	p.AddFunc("Epanechnikov", chart.EpanechnikovKernel,
		chart.PlotStyleLines, chart.Style{Symbol: 'X', LineWidth: 1, LineColor: color.NRGBA{0x00, 0xa0, 0x00, 0xff}, LineStyle: 1})
	p.AddFunc("Rectangular", chart.RectangularKernel,
		chart.PlotStyleLines, chart.Style{Symbol: '=', LineWidth: 1, LineColor: color.NRGBA{0x00, 0x00, 0xa0, 0xff}, LineStyle: 1})
	p.AddFunc("Gauss", chart.GaussKernel,
		chart.PlotStyleLines, chart.Style{Symbol: '*', LineWidth: 1, LineColor: color.NRGBA{0xa0, 0x00, 0xa0, 0xff}, LineStyle: 1})

	dumper.Plot(&p)
}

//
// Histograms Charts
//
func histChart(name, title string, stacked, counts, shifted bool) {
	dumper := NewDumper("xhist", 2, 2, 400, 300)
	defer dumper.Close()

	hc := chart.HistChart{Title: title, Stacked: stacked, Counts: counts, Shifted: shifted}
	hc.XRange.Label = "Sample Value"
	if counts {
		hc.YRange.Label = "Total Count"
	} else {
		hc.YRange.Label = "Rel. Frequency [%]"
	}
	hc.Key.Hide = true
	points := gauss(150, 10, 20, 0, 50)
	hc.AddData("Sample 1", points,
		chart.Style{ /*LineColor: color.NRGBA{0xff,0x00,0x00,0xff}, LineWidth: 1, LineStyle: 1, FillColor: color.NRGBA{0xff,0x80,0x80,0xff}*/ })
	hc.Kernel = chart.BisquareKernel //  chart.GaussKernel // chart.EpanechnikovKernel // chart.RectangularKernel // chart.BisquareKernel
	dumper.Plot(&hc)

	points2 := gauss(80, 4, 37, 0, 50)
	// hc.Kernel = nil
	hc.AddData("Sample 2", points2,
		chart.Style{ /*LineColor: color.NRGBA{0x00,0xff,0x00,0xff}, LineWidth: 1, LineStyle: 1, FillColor: color.NRGBA{0x80,0xff,0x80,0xff}*/ })
	hc.YRange.TicSetting.Delta = 0
	dumper.Plot(&hc)

	points3 := gauss(60, 15, 0, 0, 50)
	hc.AddData("Sample 3", points3,
		chart.Style{ /*LineColor: color.NRGBA{0x00,0x00,0xff,0xff}, LineWidth: 1, LineStyle: 1, FillColor: color.NRGBA{0x80,0x80,0xff,0xff}*/ })
	hc.YRange.TicSetting.Delta = 0
	dumper.Plot(&hc)

	points4 := gauss(40, 30, 15, 0, 50)
	hc.AddData("Sample 4", points4, chart.Style{ /*LineColor: color.NRGBA{0x00,0x00,0x00,0xff}, LineWidth: 1, LineStyle: 1*/ })
	hc.Kernel = nil
	hc.YRange.TicSetting.Delta = 0
	dumper.Plot(&hc)
}

//
// Bar Charts
//
func barChart() {
	dumper := NewDumper("xbar1", 3, 2, 400, 300)
	defer dumper.Close()

	red := chart.Style{Symbol: 'o', LineColor: color.NRGBA{0xcc, 0x00, 0x00, 0xff},
		FillColor: color.NRGBA{0xff, 0x80, 0x80, 0xff},
		LineStyle: chart.SolidLine, LineWidth: 2}
	green := chart.Style{Symbol: '#', LineColor: color.NRGBA{0x00, 0xcc, 0x00, 0xff},
		FillColor: color.NRGBA{0x80, 0xff, 0x80, 0xff},
		LineStyle: chart.SolidLine, LineWidth: 2}

	barc := chart.BarChart{Title: "Simple Bar Chart"}
	barc.Key.Hide = true
	barc.XRange.ShowZero = true
	barc.AddDataPair("Amount",
		[]float64{-10, 10, 20, 30, 35, 40, 50},
		[]float64{90, 120, 180, 205, 230, 150, 190}, red)
	dumper.Plot(&barc)
	barc.XRange.TicSetting.Delta = 0

	barc = chart.BarChart{Title: "Simple Bar Chart"}
	barc.Key.Hide = true
	barc.XRange.ShowZero = true
	barc.AddDataPair("Test", []float64{-5, 15, 25, 35, 45, 55}, []float64{110, 80, 95, 80, 120, 140}, green)
	dumper.Plot(&barc)
	barc.XRange.TicSetting.Delta = 0

	barc.YRange.TicSetting.Delta = 0
	barc.Title = "Combined (ugly as bar positions do not match)"
	barc.AddDataPair("Amount", []float64{-10, 10, 20, 30, 35, 40, 50}, []float64{90, 120, 180, 205, 230, 150, 190}, red)
	dumper.Plot(&barc)

	barc.Title = "Stacked (still ugly)"
	barc.Stacked = true
	dumper.Plot(&barc)

	barc = chart.BarChart{Title: "Nicely Stacked"}
	barc.Key.Hide = true
	barc.XRange.Fixed(0, 60, 10)
	barc.AddDataPair("A", []float64{10, 30, 40, 50}, []float64{110, 95, 60, 120}, red)
	barc.AddDataPair("B", []float64{10, 30, 40, 50}, []float64{40, 130, 15, 100}, green)
	dumper.Plot(&barc)

	barc.Stacked = true
	dumper.Plot(&barc)
}

//
// Categorical Bar Charts
//
func categoricalBarChart() {
	dumper := NewDumper("xbar2", 3, 2, 400, 300)
	defer dumper.Close()

	x := []float64{0, 1, 2, 3}
	europe := []float64{10, 15, 25, 20}
	asia := []float64{15, 30, 10, 20}
	africa := []float64{20, 5, 5, 5}
	blue := chart.Style{Symbol: '#', LineColor: color.NRGBA{0x00, 0x00, 0xff, 0xff}, LineWidth: 4, FillColor: color.NRGBA{0x40, 0x40, 0xff, 0xff}}
	green := chart.Style{Symbol: 'x', LineColor: color.NRGBA{0x00, 0xaa, 0x00, 0xff}, LineWidth: 4, FillColor: color.NRGBA{0x40, 0xff, 0x40, 0xff}}
	pink := chart.Style{Symbol: '0', LineColor: color.NRGBA{0x99, 0x00, 0x99, 0xff}, LineWidth: 4, FillColor: color.NRGBA{0xaa, 0x60, 0xaa, 0xff}}
	red := chart.Style{Symbol: '%', LineColor: color.NRGBA{0xcc, 0x00, 0x00, 0xff}, LineWidth: 4, FillColor: color.NRGBA{0xff, 0x40, 0x40, 0xff}}

	// Categorized Bar Chart
	c := chart.BarChart{Title: "Income"}
	c.XRange.Category = []string{"none", "low", "average", "high"}

	// Unstacked, different labelings
	c.ShowVal = 1
	c.AddDataPair("Europe", x, europe, blue)
	dumper.Plot(&c)

	c.ShowVal = 2
	c.AddDataPair("Asia", x, asia, pink)
	dumper.Plot(&c)

	c.ShowVal = 3
	c.AddDataPair("Africa", x, africa, green)
	dumper.Plot(&c)

	// Stacked with different labelings
	c.Stacked = true
	c.ShowVal = 1
	dumper.Plot(&c)

	c.ShowVal = 2
	dumper.Plot(&c)

	c.ShowVal = 3
	dumper.Plot(&c)

	// Including negative ones
	dumper2 := NewDumper("xbar3", 3, 2, 400, 300)
	defer dumper2.Close()

	c = chart.BarChart{Title: "Income"}
	c.XRange.Category = []string{"none", "low", "average", "high"}
	c.Key.Hide = true
	c.YRange.ShowZero = true
	c.ShowVal = 3

	c.AddDataPair("Europe", x, []float64{-10, -15, -20, -5}, blue)
	dumper2.Plot(&c)

	c.AddDataPair("Asia", x, []float64{-15, -10, -5, -20}, pink)
	dumper2.Plot(&c)

	c.Stacked = true
	dumper2.Plot(&c)

	// Mixed
	c = chart.BarChart{Title: "Income"}
	c.XRange.Category = []string{"none", "low", "average", "high"}
	c.Key.Hide = true
	c.YRange.ShowZero = true
	c.ShowVal = 3

	c.AddDataPair("Europe", x, []float64{-10, 15, -20, 5}, blue)
	dumper2.Plot(&c)

	c.AddDataPair("Asia", x, []float64{-15, 10, -5, 20}, pink)
	dumper2.Plot(&c)

	c.Stacked = true
	dumper2.Plot(&c)

	// Very Mixed
	c = chart.BarChart{Title: "Income"}
	c.XRange.Category = []string{"none", "low", "average", "high"}
	c.Key.Hide = true
	c.YRange.ShowZero = true
	c.ShowVal = 3

	c.AddDataPair("Europe", x, []float64{-10, 15, -20, 5}, blue)
	c.AddDataPair("Asia", x, []float64{-15, 10, 5, 20}, pink)
	c.AddDataPair("Africa", x, []float64{10, -10, 15, -5}, green)
	dumper2.Plot(&c)

	c.Stacked = true
	dumper2.Plot(&c)

	c.AddDataPair("America", x, []float64{15, -5, -10, -20}, red)
	c.YRange.TicSetting.Delta = 0
	dumper2.Plot(&c)
}

//
// Logarithmic axes
//
func logAxis() {
	dumper := NewDumper("xlog1", 2, 2, 400, 300)
	defer dumper.Close()

	lc := chart.ScatterChart{}
	lc.XRange.Label, lc.YRange.Label = "X-Value", "Y-Value"
	lx := []float64{4e-2, 3e-1, 2e0, 1e1, 8e1, 7e2, 5e3}
	ly := []float64{10, 30, 90, 270, 3 * 270, 9 * 270, 27 * 270}
	lc.AddDataPair("Measurement", lx, ly, chart.PlotStylePoints,
		chart.Style{Symbol: '#', SymbolColor: color.NRGBA{0x99, 0x66, 0xff, 0xff}, SymbolSize: 1.5})
	lc.Key.Hide = true
	lc.XRange.MinMode.Expand, lc.XRange.MaxMode.Expand = chart.ExpandToTic, chart.ExpandToTic
	lc.YRange.MinMode.Expand, lc.YRange.MaxMode.Expand = chart.ExpandToTic, chart.ExpandToTic
	lc.Title = "Lin / Lin"
	lc.XRange.Min, lc.XRange.Max = 0, 0
	lc.YRange.Min, lc.YRange.Max = 0, 0
	dumper.Plot(&lc)

	lc.Title = "Lin / Log"
	lc.XRange.Log, lc.YRange.Log = false, true
	lc.XRange.Min, lc.XRange.Max, lc.XRange.TicSetting.Delta = 0, 0, 0
	lc.YRange.Min, lc.YRange.Max, lc.YRange.TicSetting.Delta = 0, 0, 0
	dumper.Plot(&lc)

	lc.Title = "Log / Lin"
	lc.XRange.Log, lc.YRange.Log = true, false
	lc.XRange.Min, lc.XRange.Max, lc.XRange.TicSetting.Delta = 0, 0, 0
	lc.YRange.Min, lc.YRange.Max, lc.YRange.TicSetting.Delta = 0, 0, 0
	dumper.Plot(&lc)

	lc.Title = "Log / Log"
	lc.XRange.Log, lc.YRange.Log = true, true
	lc.XRange.Min, lc.XRange.Max, lc.XRange.TicSetting.Delta = 0, 0, 0
	lc.YRange.Min, lc.YRange.Max, lc.YRange.TicSetting.Delta = 0, 0, 0
	dumper.Plot(&lc)
}

//
// Pie Charts
//
func pieChart() {
	dumper := NewDumper("xpie1", 2, 2, 500, 250)
	defer dumper.Close()

	pc := chart.PieChart{Title: "Some Pies"}
	pc.AddDataPair("Data1", []string{"2009", "2010", "2011"}, []float64{10, 20, 30})
	pc.Inner = 0.75
	dumper.Plot(&pc)

	piec := chart.PieChart{Title: "Some Pies"}
	piec.AddIntDataPair("Europe",
		[]string{"D", "AT", "CH", "F", "E", "I"},
		[]int{10, 20, 30, 35, 15, 25})
	piec.Data[0].Samples[3].Flag = true
	dumper.Plot(&piec)

	piec.Inner = 0.5
	piec.FmtVal = chart.AbsoluteValue
	dumper.Plot(&piec)

	piec.Inner = 0.65
	piec.Key.Cols = 2
	piec.FmtVal = chart.PercentValue
	chart.PieChartShrinkage = 0.45
	dumper.Plot(&piec)

	dumper2 := NewDumper("xpie2", 2, 1, 500, 400)
	defer dumper2.Close()
	pie := chart.PieChart{Title: "Some Pies"}
	data := []chart.CatValue{{"D", 10, false}, {"GB", 20, true}, {"CH", 30, false}, {"F", 60, false}}
	lw := 4
	red := chart.Style{LineColor: color.NRGBA{0xcc, 0x00, 0x00, 0xff}, FillColor: color.NRGBA{0xff, 0x80, 0x80, 0xff},
		LineStyle: chart.SolidLine, LineWidth: lw}
	green := chart.Style{LineColor: color.NRGBA{0x00, 0xcc, 0x00, 0xff}, FillColor: color.NRGBA{0x80, 0xff, 0x80, 0xff},
		LineStyle: chart.SolidLine, LineWidth: lw}
	blue := chart.Style{LineColor: color.NRGBA{0x00, 0x00, 0xcc, 0xff}, LineWidth: lw, LineStyle: chart.SolidLine, FillColor: color.NRGBA{0x80, 0x80, 0xff, 0xff}}
	pink := chart.Style{LineColor: color.NRGBA{0x99, 0x00, 0x99, 0xff}, LineWidth: lw, LineStyle: chart.SolidLine, FillColor: color.NRGBA{0xaa, 0x60, 0xaa, 0xff}}

	styles := []chart.Style{red, green, blue, pink}
	pie.FmtKey = chart.IntegerValue
	pie.AddData("Data1", data, styles)
	pie.Inner = 0
	pie.Key.Cols = 2
	pie.Key.Pos = "ibr"
	dumper2.Plot(&pie)

	pie = chart.PieChart{Title: "Some Rings"}
	data2 := []chart.CatValue{{"D", 15, false}, {"GB", 25, false}, {"CH", 30, false}, {"F", 50, false}}
	data[1].Flag = false
	lw = 2
	lightred := chart.Style{LineColor: color.NRGBA{0xcc, 0x40, 0x40, 0xff}, FillColor: color.NRGBA{0xff, 0xc0, 0xc0, 0xff},
		LineStyle: chart.SolidLine, LineWidth: lw}
	lightgreen := chart.Style{LineColor: color.NRGBA{0x40, 0xcc, 0x40, 0xff}, FillColor: color.NRGBA{0xc0, 0xff, 0xc0, 0xff},
		LineStyle: chart.SolidLine, LineWidth: lw}
	lightblue := chart.Style{LineColor: color.NRGBA{0x40, 0x40, 0xcc, 0xff}, FillColor: color.NRGBA{0xc0, 0xc0, 0xff, 0xff},
		LineWidth: lw, LineStyle: chart.SolidLine}
	lightpink := chart.Style{LineColor: color.NRGBA{0xaa, 0x00, 0xaa, 0xff}, FillColor: color.NRGBA{0xff, 0x80, 0xff, 0xff},
		LineWidth: lw, LineStyle: chart.SolidLine}
	lightstyles := []chart.Style{lightred, lightgreen, lightblue, lightpink}

	pie.Inner = 0.3
	pie.Key.Cols = 2
	pie.Key.Pos = "ibr"
	pie.FmtVal = chart.PercentValue
	chart.PieChartShrinkage = 0.55
	pie.FmtKey = chart.IntegerValue

	pie.AddData("1980", data, styles)
	pie.AddData("2010", data2, lightstyles)
	dumper2.Plot(&pie)
}

func textlen() {
	s2f, _ := os.Create("text.svg")
	mysvg := svg.New(s2f)
	mysvg.Start(1600, 800)
	mysvg.Title("My Plot")
	mysvg.Rect(0, 0, 2000, 800, "fill: #ffffff")
	sgr := svgg.New(mysvg, 2000, 800, "Arial", 18, Background)
	sgr.Begin()

	texts := []string{"ill", "WWW", "Some normal text.", "Illi, is. illigalli: ill!", "OO WORKSHOOPS OMWWW BMWWMB"}
	fonts := []string{"Arial", "Helvetica", "Times", "Courier" /* "Calibri", "Palatino" */}
	sizes := []chart.FontSize{chart.TinyFontSize, chart.SmallFontSize, chart.NormalFontSize, chart.LargeFontSize, chart.HugeFontSize}
	font := chart.Font{Color: color.NRGBA{0x00, 0x00, 0x00, 0xff}}

	df := chart.Font{Name: "Arial", Color: color.NRGBA{0x20, 0x20, 0xff, 0xff}, Size: -3}
	x, y := 20, 40
	for _, t := range texts {
		for _, f := range fonts {
			for _, s := range sizes {
				font.Name, font.Size = f, s
				tvl := sgr.TextLen(t, font)
				sgr.Text(x+tvl/2, y-2, t, "cc", 0, font)
				sgr.Line(x, y, x+tvl, y, chart.Style{LineColor: color.NRGBA{0xff, 0x00, 0x00, 0xff}, LineWidth: 2, LineStyle: chart.SolidLine})
				r := fmt.Sprintf("%s (%d)", f, s)
				sgr.Text(x+tvl+10, y-2, r, "cl", 0, df)
				y += 30
				if y > 760 {
					y = 40
					x += 300
				}
			}
		}
	}

	sgr.End()
	mysvg.End()
	s2f.Close()

}

//
// Test of graphic primitives
//
func testGraphics() {
	dumper := NewDumper("xgraphics", 1, 1, 900, 416)
	defer dumper.Close()

	igr := imgg.AddTo(dumper.I, 0, 0, 900, 416, color.RGBA{0xff, 0xff, 0xff, 0xff}, nil, nil)
	sgr := svgg.AddTo(dumper.S, 0, 0, 900, 416, "", 14, color.RGBA{0xff, 0xff, 0xff, 0xff})

	style := chart.Style{LineWidth: 0, LineColor: color.NRGBA{0x00, 0x00, 0x00, 0xff}, LineStyle: chart.SolidLine}

	// Line Width
	x0, y0 := 10, 10
	for w := 1; w <= 10; w++ {
		style.LineWidth = w
		igr.Line(x0, y0, x0+160, y0, style)
		sgr.Line(x0, y0, x0+160, y0, style)
		y0 += w + 5
	}

	// Line Color
	style = chart.Style{LineWidth: 19, LineColor: color.NRGBA{0x80, 0x80, 0x80, 0xff}, LineStyle: chart.SolidLine}
	igr.Line(x0+40, y0-5, x0+40, y0+174, style)
	sgr.Line(x0+40, y0-5, x0+40, y0+174, style)
	igr.Line(x0+80, y0-5, x0+80, y0+174, style)
	sgr.Line(x0+80, y0-5, x0+80, y0+174, style)
	igr.Line(x0+120, y0-5, x0+120, y0+174, style)
	sgr.Line(x0+120, y0-5, x0+120, y0+174, style)

	style = chart.Style{LineWidth: 5, LineStyle: chart.SolidLine}
	for _, col := range []color.NRGBA{
		color.NRGBA{0x00, 0x00, 0x00, 0xff}, color.NRGBA{0xff, 0x00, 0x00, 0xff},
		color.NRGBA{0x00, 0xff, 0x00, 0xff}, color.NRGBA{0x00, 0x00, 0xff, 0xff},
		color.NRGBA{0xff, 0xff, 0x00, 0xff}, color.NRGBA{0xff, 0x00, 0xff, 0xff},
		color.NRGBA{0x00, 0xff, 0xff, 0xff},
		color.NRGBA{0x3f, 0x3f, 0x3f, 0xff}, color.NRGBA{0x7f, 0x7f, 0x7f, 0xff},
		color.NRGBA{0xbf, 0xbf, 0xbf, 0xff}, color.NRGBA{0xff, 0xff, 0xff, 0xff},
		color.NRGBA{0xcc, 0x00, 0x00, 0xff}, color.NRGBA{0x00, 0xbb, 0x00, 0xff},
		color.NRGBA{0x00, 0x00, 0xdd, 0xff}, color.NRGBA{0x99, 0x66, 0x00, 0xff},
		color.NRGBA{0xbb, 0x00, 0xbb, 0xff}, color.NRGBA{0x00, 0xaa, 0xaa, 0xff},
		color.NRGBA{0xaa, 0xaa, 0x00, 0xff},
	} {
		d := 0
		for _, a := range []uint8{0xff, 0xc0, 0x80, 0x40, 0x00} {
			c := col
			c.A = a
			style.LineColor = c
			igr.Line(x0+d, y0, x0+d+40, y0, style)
			sgr.Line(x0+d, y0, x0+d+40, y0, style)
			d += 40
		}
		y0 += 10
	}

	// Line Style
	style.LineColor = color.NRGBA{0x00, 0x00, 0x00, 0xff}
	style.LineWidth = 1
	for _, st := range []chart.LineStyle{
		chart.SolidLine, chart.DashedLine, chart.DottedLine, chart.DashDotDotLine,
		chart.LongDashLine, chart.LongDotLine,
	} {
		style.LineStyle = st
		igr.Line(x0, y0, x0+160, y0, style)
		sgr.Line(x0, y0, x0+160, y0, style)
		y0 += 5

	}
	style.LineWidth = 9
	y0 += 10
	for _, st := range []chart.LineStyle{
		chart.SolidLine, chart.DashedLine, chart.DottedLine, chart.DashDotDotLine,
		chart.LongDashLine, chart.LongDotLine,
	} {
		style.LineStyle = st
		igr.Line(x0, y0, x0+160, y0, style)
		sgr.Line(x0, y0, x0+160, y0, style)
		y0 += 12

	}

	// Text Alignment
	font := chart.Font{}
	rx, ry := 180, 10
	px, py := 450, 90
	text := "(JgbXÄj)"
	alignedText(igr, text, font, rx, ry, px, py, 0, 0)
	alignedText(sgr, text, font, rx, ry, px, py, 0, 0)

	font.Size = chart.HugeFontSize
	rx, ry = 180, 100
	px, py = 450, 180
	alignedText(igr, text, font, rx, ry, px, py, 0, 0)
	alignedText(sgr, text, font, rx, ry, px, py, 0, 0)

	font.Size = chart.TinyFontSize
	rx, ry = 180, 190
	px, py = 450, 270
	alignedText(igr, text, font, rx, ry, px, py, 0, 0)
	alignedText(sgr, text, font, rx, ry, px, py, 0, 0)

	// Rectangles
	x0, y0 = 180, 285
	dx, dy := 40, 30
	w, h := 30, 20
	n, m := 7, 4

	// background cross
	style = chart.Style{LineWidth: 19, LineColor: color.NRGBA{0x80, 0x80, 0x80, 0xff}, LineStyle: chart.SolidLine}
	igr.Line(x0, y0+2, x0+(n-1)*dx+w, y0+(m-1)*dy+h-2, style)
	sgr.Line(x0, y0+2, x0+(n-1)*dx+w, y0+(m-1)*dy+h-2, style)
	style = chart.Style{LineWidth: 19, LineColor: color.NRGBA{0x0, 0xd0, 0x0, 0xff}, LineStyle: chart.SolidLine}
	igr.Line(x0, y0+(m-1)*dy+h-2, x0+(n-1)*dx+w, y0+2, style)
	sgr.Line(x0, y0+(m-1)*dy+h-2, x0+(n-1)*dx+w, y0+2, style)

	for i := 1; i <= n*m; i++ {
		alpha := uint8(i * 256 / (n*m + 1))
		style := chart.Style{LineWidth: 3, LineColor: color.NRGBA{0xc0, 0x0, 0x0, alpha},
			LineStyle: chart.SolidLine, FillColor: color.NRGBA{0x0, 0x0, 0xc0, 0xff - alpha}}
		igr.Rect(x0, y0, w, h, style)
		sgr.Rect(x0, y0, w, h, style)
		if i%n == 0 {
			x0 = 180
			y0 += dy
		} else {
			x0 += dx
		}
	}

	// Symbols
	scols := []color.NRGBA{{0, 0, 0, 0xff}, {0xc0, 0x00, 0x00, 0xff}, {0x00, 0xc0, 0x00, 0xff},
		{0x00, 0x00, 0xc0, 0xff}, {0x80, 0x80, 0x80, 0xff}}

	dx, dy = 30, 27
	font = chart.Font{Color: color.NRGBA{0x00, 0x00, 0x00, 0xff}}
	for i, st := range []int{'o', '=', '%', '&', '+', 'X', '*', '0', '@', '#', 'A', 'W', 'V', 'Z', '.'} {
		igr.Text(470, 20+i*dy, fmt.Sprintf("%c", st), "cc", 0, font)
		sgr.Text(470, 20+i*dy, fmt.Sprintf("%c", st), "cc", 0, font)
	}
	for si, size := range []float64{0.5, 0.67, 1.0, 1.5, 2.0} {
		x0, y0 = 500+si*dx, 20
		for _, st := range []int{'o', '=', '%', '&', '+', 'X', '*', '0', '@', '#', 'A', 'W', 'V', 'Z', '.'} {
			style := chart.Style{Symbol: st, SymbolColor: scols[si], SymbolSize: size}
			igr.Symbol(x0, y0, style)
			sgr.Symbol(x0, y0, style)
			y0 += dy
		}
	}

	// Rotated text
	gray := color.NRGBA{0x80, 0x80, 0x80, 0xff}
	style = chart.Style{LineColor: gray, FillColor: gray}
	rx, ry = 675, 50
	px, py = 875, 200
	// igr.Rect(rx, ry, px-rx, py-ry, style)
	// sgr.Rect(rx, ry, px-rx, py-ry, style)
	text = "[##X##]"
	alignedText(igr, text, font, rx, ry, px, py, 20, 5)
	alignedText(sgr, text, font, rx, ry, px, py, 20, 5)

	x0, y0 = 775, 325
	black := color.NRGBA{0x00, 0x00, 0x00, 0xff}
	font.Color = black
	style.LineWidth, style.LineColor, style.LineStyle = 1, black, chart.SolidLine
	igr.Line(x0-100, y0, x0+100, y0, style)
	igr.Line(x0, y0-100, x0, y0+100, style)
	igr.Text(x0, y0, "abcABCxyz", "cc", 1, font)
	igr.Text(x0, y0, "abcABCxyz", "cc", 30, font)
	igr.Text(x0, y0, "abcABCxyz", "cc", 60, font)
	igr.Text(x0, y0, "abcABCxyz", "cc", 90, font)

	/*
		font.Color = color.NRGBA{0xee, 0x00, 0x00, 0xff}
		igr.Text(x0, y0, "abcABCxyz", "tl", 1, font)
		igr.Text(x0, y0, "abcABCxyz", "tl", 30, font)
		igr.Text(x0, y0, "abcABCxyz", "tl", 60, font)
		igr.Text(x0, y0, "abcABCxyz", "tl", 90, font)
		font.Color = color.NRGBA{0x11, 0xee, 0x11, 0xff}
		igr.Text(x0, y0, "abcABCxyz", "br", 1, font)
		igr.Text(x0, y0, "abcABCxyz", "br", 30, font)
		igr.Text(x0, y0, "abcABCxyz", "br", 60, font)
		igr.Text(x0, y0, "abcABCxyz", "br", 90, font)
	*/
}

func alignedText(g chart.Graphics, text string, font chart.Font, rx, ry, px, py int, rot int, drot int) {
	mx, my := (rx+px)/2, (ry+py)/2
	var style chart.Style
	style.LineWidth, style.LineColor, style.LineStyle = 1, color.NRGBA{0xff, 0x00, 0x00, 0xff}, chart.SolidLine
	g.Line(rx, ry, px, ry, style)
	g.Line(px, ry, px, py, style)
	g.Line(px, py, rx, py, style)
	g.Line(rx, py, rx, ry, style)
	g.Line(mx, ry, mx, py, style)
	g.Line(rx, my, px, my, style)

	font.Color = color.NRGBA{0x00, 0x00, 0x00, 0xff}
	g.Text(rx, ry, text, "tl", rot, font)
	font.Color = color.NRGBA{0xff, 0x00, 0x00, 0xff}
	rot += drot
	g.Text(mx, ry, text, "tc", rot, font)
	font.Color = color.NRGBA{0x00, 0xff, 0x00, 0xff}
	rot += drot
	g.Text(px, ry, text, "tr", rot, font)
	font.Color = color.NRGBA{0x00, 0x00, 0xff, 0xff}
	rot += drot
	g.Text(rx, my, text, "cl", rot, font)
	font.Color = color.NRGBA{0xbb, 0xbb, 0x00, 0xff}
	rot += drot
	g.Text(mx, my, text, "cc", rot, font)
	font.Color = color.NRGBA{0xff, 0x00, 0xff, 0xff}
	rot += drot
	g.Text(px, my, text, "cr", rot, font)
	font.Color = color.NRGBA{0x00, 0xff, 0xff, 0xff}
	rot += drot
	g.Text(rx, py, text, "bl", rot, font)
	font.Color = color.NRGBA{0x60, 0x60, 0x60, 0xff}
	rot += drot
	g.Text(mx, py, text, "bc", rot, font)
	font.Color = color.NRGBA{0x00, 0x00, 0x00, 0xff}
	rot += drot
	g.Text(px, py, text, "br", rot, font)
}

func bestOf() {
	const (
		width  = 500
		height = 300
		N      = 3
		M      = 3
	)

	charts := make([]chart.Chart, 0, N*M)

	// Strip Chart
	strip := chart.StripChart{Jitter: true}
	strip.Title = "Filament Length in NaCl"
	strip.AddData("Sample A", data1, chart.Style{})
	strip.AddData("Sample B", data2, chart.Style{})
	strip.AddData("Sample C", data3, chart.Style{})
	strip.XRange.Label = "Filament Length"
	strip.Key.Pos = "icr"
	charts = append(charts, &strip)

	// Pie Chart
	piec := chart.PieChart{Title: "Distribution of Foo Bars"}
	piec.AddDataPair("Europe", []string{"D", "AT", "CH", "F", "E", "I"}, []float64{10, 20, 30, 35, 15, 25})
	piec.Data[0].Samples[3].Flag = true
	piec.Inner = 0.5
	piec.Key.Border = -1
	piec.FmtVal = chart.AbsoluteValue
	charts = append(charts, &piec)

	// Fancy tics
	trigc := chart.ScatterChart{Title: ""}
	trigc.XRange.Fixed(0, 4*math.Pi, math.Pi)
	trigc.YRange.Fixed(-1.25, 1.25, 0.5)
	trigc.XRange.TicSetting.Format = func(f float64) string {
		w := int(180*f/math.Pi + 0.5)
		return fmt.Sprintf("%d°", w)
	}
	trigc.AddFunc("Sin(x)", func(x float64) float64 { return math.Sin(x) }, chart.PlotStyleLines,
		chart.Style{Symbol: '@', LineWidth: 2, LineColor: color.NRGBA{0x00, 0x00, 0xcc, 0xff}, LineStyle: 0})
	trigc.AddFunc("Cos(x)", func(x float64) float64 { return math.Cos(x) }, chart.PlotStyleLines,
		chart.Style{Symbol: '%', LineWidth: 2, LineColor: color.NRGBA{0x00, 0xcc, 0x00, 0xff}, LineStyle: 0})
	trigc.XRange.TicSetting.Tics, trigc.YRange.TicSetting.Tics = 1, 1
	trigc.XRange.TicSetting.Mirror, trigc.YRange.TicSetting.Mirror = 2, 2
	trigc.XRange.TicSetting.Grid, trigc.YRange.TicSetting.Grid = 2, 1
	trigc.XRange.ShowZero = true
	charts = append(charts, &trigc)

	// Log axis
	log := chart.ScatterChart{Title: "A Log / Log - Plot"}
	log.XRange.Label, log.YRange.Label = "Energy [mJ]", "Depth [cm]"
	lx := []float64{4e-2, 3e-1, 2e0, 1e1, 8e1, 7e2, 5e3}
	ly := []float64{10, 30, 90, 270, 3 * 270, 9 * 270, 27 * 270}
	log.AddDataPair("Electrons", lx, ly, chart.PlotStylePoints,
		chart.Style{Symbol: '#', SymbolColor: color.NRGBA{0x99, 0x66, 0xff, 0xff}, SymbolSize: 1.5})
	log.Data[0].Samples[1].DeltaX = 0.3
	log.Data[0].Samples[1].DeltaY = 25
	log.Data[0].Samples[3].DeltaX = 9
	log.Data[0].Samples[3].DeltaY = 210
	log.Data[0].Samples[5].DeltaX = 500
	log.Data[0].Samples[5].DeltaY = 1900
	log.Key.Hide = true
	log.XRange.MinMode.Expand, log.XRange.MaxMode.Expand = chart.ExpandToTic, chart.ExpandToTic
	log.YRange.MinMode.Expand, log.YRange.MaxMode.Expand = chart.ExpandToTic, chart.ExpandToTic
	log.XRange.Log, log.YRange.Log = true, true
	charts = append(charts, &log)

	// Stacked Histograms
	hist := chart.HistChart{Title: "Stacked Histograms", Stacked: true, Counts: false}
	hist.XRange.Label = "Sample Value"
	hist.YRange.Label = "Rel. Frequency [%]"
	points := gauss(150, 10, 20, 0, 50)
	hist.AddData("Sample 1", points,
		chart.Style{LineColor: color.NRGBA{0xff, 0x00, 0x00, 0xff}, LineWidth: 1, FillColor: color.NRGBA{0xff, 0x80, 0x80, 0xff}})
	points2 := gauss(80, 4, 37, 0, 50)
	hist.AddData("Sample 2", points2,
		chart.Style{LineColor: color.NRGBA{0x00, 0xff, 0x00, 0xff}, LineWidth: 1, FillColor: color.NRGBA{0x80, 0xff, 0x80, 0xff}})
	points3 := gauss(60, 15, 0, 0, 50)
	hist.AddData("Sample 3", points3,
		chart.Style{LineColor: color.NRGBA{0x00, 0x00, 0xff, 0xff}, LineWidth: 1, FillColor: color.NRGBA{0x80, 0x80, 0xff, 0xff}})
	charts = append(charts, &hist)

	// Box Plots
	box := chart.BoxChart{Title: "Influence of doses on effect"}
	box.XRange.Label, box.YRange.Label = "Number of unit doses applied", "Effect [a.u.]"
	box.Key.Pos = "orc"
	box.NextDataSet("Male",
		chart.Style{Symbol: '#', LineColor: color.NRGBA{0x00, 0x00, 0xcc, 0xff}, LineWidth: 1, LineStyle: chart.SolidLine})
	for x := 10; x < 50; x += 5 {
		points := make([]float64, 70)
		a := rand.Float64() * 10
		v := rand.Float64()*5 + 2
		for i := 0; i < len(points); i++ {
			x := rand.NormFloat64()*v + a
			points[i] = x
		}
		box.AddSet(float64(x), points, true)
	}
	box.NextDataSet("Female",
		chart.Style{Symbol: '%', LineColor: color.NRGBA{0xcc, 0x00, 0x00, 0xff}, LineWidth: 1, LineStyle: chart.SolidLine})
	for x := 12; x <= 50; x += 10 {
		points := make([]float64, 60)
		a := rand.Float64()*15 + 30
		v := rand.Float64()*5 + 2
		for i := 0; i < len(points); i++ {
			x := rand.NormFloat64()*v + a
			points[i] = x
		}
		box.AddSet(float64(x), points, true)
	}
	charts = append(charts, &box)

	// Bar Chart
	x := []float64{0, 1, 2, 3}
	europe := []float64{10, 15, 25, 20}
	asia := []float64{15, 30, 10, 20}
	africa := []float64{20, 5, 5, 5}
	blue := chart.Style{Symbol: '#', LineColor: color.NRGBA{0x00, 0x00, 0xff, 0xff},
		LineWidth: 3, FillColor: color.NRGBA{0x40, 0x40, 0xff, 0xff}}
	green := chart.Style{Symbol: 'x', LineColor: color.NRGBA{0x00, 0xaa, 0x00, 0xff},
		LineWidth: 3, FillColor: color.NRGBA{0x40, 0xff, 0x40, 0xff}}
	pink := chart.Style{Symbol: '0', LineColor: color.NRGBA{0x99, 0x00, 0x99, 0xff},
		LineWidth: 3, FillColor: color.NRGBA{0xaa, 0x60, 0xaa, 0xff}}

	bar := chart.BarChart{Title: "Income Distribution"}
	bar.XRange.Category = []string{"none", "low", "average", "high"}
	bar.XRange.Label = "Income category"
	bar.YRange.Label = "Adult population"
	bar.YRange.TicSetting.Format = func(f float64) string {
		return fmt.Sprintf("%d%%", int(f+0.5))
	}
	bar.Stacked = true
	bar.Key.Pos, bar.Key.Cols = "obc", 3
	bar.AddDataPair("Europe", x, europe, blue)
	bar.AddDataPair("Asia", x, asia, pink)
	bar.AddDataPair("Africa", x, africa, green)
	bar.ShowVal = 3
	charts = append(charts, &bar)
	// Time axis
	tdc := chart.ScatterChart{Title: "Crop Growth"}
	tdc.XRange.Time, tdc.YRange.Time = true, true
	tdc.XRange.Label, tdc.YRange.Label = "Seeding", "Harvesting"
	tdc.XRange.TicSetting.Mirror, tdc.YRange.TicSetting.Mirror = 1, 1
	tdc.XRange.TicSetting.Grid, tdc.YRange.TicSetting.Grid = 1, 1
	dt := make([]chart.EPoint, 0, 20)
	for _, q := range [][2]string{
		[2]string{"12.03.2008", "15.09.2008"},
		[2]string{"22.03.2008", "17.09.2008"},
		[2]string{"29.03.2008", "16.09.2008"},
		[2]string{"04.04.2008", "20.09.2008"},
		[2]string{"14.04.2008", "22.09.2008"},
		[2]string{"25.04.2008", "27.09.2008"},
		[2]string{"10.05.2008", "26.09.2008"},
		[2]string{"20.05.2008", "29.09.2008"},
		[2]string{"30.05.2008", "02.10.2008"},
		[2]string{"05.06.2008", "04.10.2008"},
		[2]string{"19.06.2008", "05.10.2008"},
		[2]string{"27.06.2008", "04.10.2008"},
		[2]string{"07.07.2008", "07.10.2008"},
		[2]string{"17.07.2008", "08.10.2008"},
		[2]string{"25.07.2008", "06.10.2008"},
		[2]string{"05.08.2008", "10.10.2008"},
		[2]string{"19.08.2008", "12.10.2008"},
		[2]string{"28.08.2008", "13.10.2008"},
		[2]string{"10.09.2008", "15.10.2008"},
		[2]string{"20.09.2008", "17.10.2008"},
		[2]string{"29.09.2008", "18.10.2008"},
	} {
		t0, _ := time.Parse("02.01.2006", q[0])
		t1, _ := time.Parse("02.01.2006", q[1])
		ep := chart.EPoint{X: float64(t0.Unix()), Y: float64(t1.Unix()),
			DeltaX: math.NaN(), DeltaY: math.NaN()}
		dt = append(dt, ep)
	}
	tdc.Key.Pos = "ibr"
	tdc.AddData("Data", dt, chart.PlotStylePoints, chart.Style{Symbol: 'o', SymbolColor: color.NRGBA{0xcc, 0x00, 0x00, 0xff}})
	tdc.AddFunc("Low", func(x float64) float64 { return 0.15*(x-1.20525e9) + 1.221e9 },
		chart.PlotStyleLines,
		chart.Style{Symbol: 'x', LineColor: color.NRGBA{0x00, 0xcc, 0x00, 0xff}, LineWidth: 2, LineStyle: chart.DashedLine})
	tdc.AddFunc("High", func(x float64) float64 { return 0.18*(x-1.20525e9) + 1.2222e9 },
		chart.PlotStyleLines,
		chart.Style{Symbol: '+', LineColor: color.NRGBA{0x00, 0x00, 0xcc, 0xff}, LineWidth: 2, LineStyle: chart.LongDotLine})
	charts = append(charts, &tdc)

	// Bar charts
	ebit := chart.BarChart{Title: "EBIT"}
	ebit.XRange.Category = []string{"2007", "2008", "2009", "2010"}
	ebit.XRange.Label, ebit.YRange.Label = "Fiscal Year", "in 1000 US$"
	ebit.Key.Pos, ebit.Key.Cols, ebit.Key.Border = "otc", 2, -1
	ebit.YRange.ShowZero = true
	ebit.ShowVal = 0
	ebit.AddDataPair("Transport", []float64{0, 1, 2, 3}, []float64{1450, 1750, -870, 900},
		chart.Style{Symbol: '#', LineColor: color.NRGBA{0x30, 0x30, 0xff, 0xff}, LineWidth: 2, FillColor: color.NRGBA{0xcb, 0xcb, 0xff, 0xff}})
	ebit.AddDataPair("Energy", []float64{0, 1, 2, 3}, []float64{960, -490, 450, 1230},
		chart.Style{Symbol: 'O', LineColor: color.NRGBA{0xe0, 0x44, 0x44, 0xff}, LineWidth: 2, FillColor: color.NRGBA{0xf6, 0xb5, 0xcc, 0xff}})
	charts = append(charts, &ebit)

	dumper := NewDumper("bestof", N, M, width, height)
	defer dumper.Close()
	for _, c := range charts {
		dumper.Plot(c)
		c.Reset()
	}
}

func timeRange() {
	factors := []int64{1, 2, 3, 5, 7, 9, 11, 15}
	magnitudes := []int64{1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9}
	steps := make([]float64, 0, len(factors)*len(magnitudes))
	for _, f := range factors {
		for _, m := range magnitudes {
			steps = append(steps, float64(f)*float64(m))
		}
	}
	sort.Float64s(steps)

	now := float64(time.Now().Unix())
	for _, step := range steps {
		fmt.Printf("\nStep %d seconds (approx %d hours or %d weeks or %d years)\n",
			int64(step), int64(step/3600+0.5), int64(step/(3600*24*7)+0.5),
			int(step/(3600*24*365.25)+0.5))
		rng := chart.Range{Time: true}
		rng.Init()
		rng.DataMin, rng.DataMax = now, now+step
		rng.Setup(7, 9, 90, 5, false)
		tg := txtg.New(120, 4)
		tg.XAxis(rng, 1, 0, nil)
		fmt.Printf("%s\n", tg.String())
	}
}

//
// Nettomieten
//
func mietenChart() {
	dumper := NewDumper("xmieten", 2, 3, 600, 250)
	defer dumper.Close()

	hc := chart.HistChart{Title: "Nettomieten", Stacked: false, Counts: false}
	hc.XRange.Label, hc.YRange.Label = "Nettomiete in Euro", "Count"
	hc.Key.Hide = true
	hc.XRange.MinMode.Fixed, hc.XRange.MaxMode.Fixed = true, true
	hc.XRange.MinMode.Value, hc.XRange.MaxMode.Value = 0, 1700
	hc.XRange.TicSetting.Delta = 250

	hc.YRange.MinMode.Fixed, hc.YRange.MaxMode.Fixed = true, true
	hc.YRange.MinMode.Value, hc.YRange.MaxMode.Value = 0, 30
	hc.YRange.TicSetting.Delta = 10

	hc.XRange.TicSetting.Mirror = 1
	hc.YRange.TicSetting.Mirror = 1

	points := []float64{
		741.39, 715.82, 528.25, 553.99, 698.21, 935.65, 204.85, 426.93, 446.33, 381.45, 337.26,
		756.73, 945.9, 264.93,
		540.58, 757.74, 538.7, 796.07, 461.19, 752.27, 446.55, 548.17, 421.03, 759.57, 390, 436.89, 174.76, 354.91, 491.62,
		695.36, 540.22, 325.12, 598.88, 466.38, 374.13, 488.3, 662.55, 357.05, 1061.81, 546.1, 360.48, 422.43, 433.25, 521.53,
		264.99, 480.1, 383.72, 636.13, 415.91, 1355.28, 575.9, 935.9, 624.8, 606.91, 347.68, 373.98, 1063.5, 894.78, 1077.23,
		331.67, 849.24, 1012.86, 664.69, 1022.6, 640.32, 649.35, 699.46, 479.46, 400.34, 1151.63, 593.4, 817.25, 584.38, 505.62,
		621.6, 706.61, 691.92, 566.21, 269.84, 526.7, 710.69, 689.49, 511.3, 418.3, 415.32, 445.33, 359.73, 890, 483.43,
		656.51, 695, 106.22, 132.24, 456.44, 489, 825.48, 831.28, 715.82, 637.38, 813.78, 339.5, 598.23, 361.97, 693.57,
		549.98, 621.79, 352.85, 969.36, 298.95, 456.94, 292.28, 649.82, 363.91, 1749.15, 679.01, 643.64, 727.96, 963.84, 338.34,
		718.07, 892.13, 1217.15, 338.15, 466.43, 570.1, 673.47, 562.58, 639.13, 619.94, 426.73, 294.82, 304.23, 587.99, 308.99,
		516.5, 998.83, 951.37, 679.38, 505.67, 292.61, 536.87, 613.55, 546.5, 246.48, 588, 756.34, 149.62, 237.75, 261.19,
		476.53, 379.86, 181.98, 907.53, 767.94, 740.71, 678.93, 705.29, 458.92, 560.76, 559.87, 690, 920.34, 469.08, 324.67,
		685.04, 410.74, 608.43, 383.48, 505.23, 586.11, 585.03, 605.78, 536.74, 690, 423.43, 352.5, 387.96, 428.93, 277.93,
		1150.42, 392.98, 1038.48, 490.85, 389.77, 618.17, 718.86, 377.08, 552.21, 871.77, 419.27, 383.47, 635.69, 527.76, 647.81,
		345.72, 680.36, 517.59, 414.65, 722.92, 524.14, 410.97, 175.81, 1113.78, 314.74, 551.55, 773.69, 487.61, 762.35, 486.4,
		400, 1119.75, 538.07, 1153.77, 300.42, 564.01, 637.03, 562.43, 496.15, 261.98, 926.93, 393.2, 565.09, 613.56, 592.96,
		458.82, 279.35, 800, 563.96, 389.77, 1288.48, 488.55, 766.95, 921.23, 373.25, 331.87, 344.06, 529.56, 617.65, 239.07,
		424.71, 377.2, 591.44, 521.52, 627.66, 426.91, 695.45, 184.11, 661.31, 869.21, 304.22, 997, 458.11, 163.17, 640,
		485.74, 559.76, 505.08, 603.34, 673.87, 594.4, 554.73, 768.15, 664.69, 764.5, 257.3, 1022.6, 1195.52, 890.59, 327.23,
		392.05, 544.93, 583.75, 501.08, 676.33, 658.42, 747.22, 450.97, 848.76, 203.75, 1094.18, 521.52, 592.84, 271.11, 855.03,
		272.96, 602.69, 700.47, 486.06, 777.19, 603.33, 448.42, 509.28, 789.96, 390.26, 956.13, 769.76, 753.29, 664.69, 923.67,
		556.89, 1236.38, 284.92, 974.84, 506.18, 882.24, 461.81, 516.77, 572.66, 654.97, 594.54, 425.36, 505.38, 653.44, 814.56,
		282.77, 471.66, 511.44, 593.11, 378.37, 1073.73, 283.9, 710.94, 408.89, 864.1, 363.35, 619.69, 862.57, 499.23, 938.61,
		611.96, 826.91, 442.9, 877, 238.62, 983.17, 107.74, 819.13, 417.47, 1095.49, 735.52, 710.24, 632.47, 870, 495.84,
		268.36, 637.85, 894.78, 444.83, 587.99, 485.73, 766.95, 720.93, 652.04, 536.86, 332.6, 584.6, 1344.72, 442.46, 602.6,
		562.43, 384, 334, 239.44, 910.11, 541.87, 372.34, 841.08, 261.53, 640.14, 688.31, 1789.55, 311.87, 263.21, 345.23,
		487.89, 479.87, 1089.39, 484.04, 619.98, 1216.99, 551.64, 81.28, 344.24, 674.26, 170.04, 591.56, 171.61, 840.22, 390.44,
		683.74, 778.34, 370.71, 447.83, 286.95, 359.17, 442.29, 77.31, 257.18, 635.94, 533.66, 240.31, 424.38, 260.41, 249.1,
		426.61, 287.36, 624.3, 352.79, 595.66, 355.59, 432.87, 321.77, 377.25, 288.88, 994.29, 945.91, 383.36, 587.99, 995,
		640, 732.87, 516.05, 647.81, 610.77, 281.21, 1073.73, 537.44, 367.78, 250.09, 404.72, 445.13, 564.27, 562.43, 613.33,
		542.86, 1165.75, 320.07, 824.21, 307.68, 498.79, 357.91, 1053.25, 761.94, 460.17, 368.59, 417.34, 584.25, 532.31, 733.6,
		745.62, 483.69, 574.25, 746.32, 480.88, 402.91, 640.39, 458.03, 283.36, 483.16, 457.2, 637.95, 634.76, 749.05, 741.39,
		434.24, 573.37, 659.58, 422.3, 379.29, 441.41, 805.33, 680.03, 677.46, 1012.86, 605.31, 689.2, 613.56, 628.29, 482.96,
		843.65, 488.08, 267.48, 366.55, 818.08, 383.47, 460, 244.5, 393.7, 818.07, 649.36, 971.47, 463.4, 541.8, 698.14,
		574.62, 716, 299.62, 501.07, 676.68, 642.88, 579.28, 498.5, 420.5, 329.01, 169.74, 324.72, 489.78, 676.46, 568.98,
		408.02, 336.29, 806.79, 623.02, 668.34, 230.15, 441.41, 489.58, 602.16, 285.7, 511.31, 306.78, 485.24, 591.67, 714.87,
		850.48, 1035.68, 386.65, 951.81, 579.81, 680.03, 450.96, 968.16, 369.05, 653.16, 479.95, 533.75, 200.84, 349.06, 899.73,
		843.64, 597.47, 1308.94, 676.77, 528.03, 856.24, 971.47, 369.84, 408.84, 334.41, 711.56, 563.79, 585.44, 770.34, 622.11,
		246.18, 1173.94, 537.38, 293.22, 562.43, 383.48, 688.31, 477.73, 1037.94, 639.12, 1091.6, 618.67, 741.39, 312.92, 408.32,
		826.26, 350.63, 383.47, 493.43, 666.51, 664.69, 551.7, 585.09, 450.96, 593.77, 598.22, 420.15, 618.84, 507.02, 531.75,
		734.54, 1017.49, 567.55, 500.06, 331.17, 593.11, 134.37, 195.72, 659.64, 773.08, 313.78, 279.21, 660.79, 474.4, 574.14,
		180.4, 736.42, 360.56, 736.9, 536.87, 682.79, 531.76, 372.6, 575, 282.93, 705.59, 513.26, 1004, 434.6, 448.55,
		564.65, 423.99, 684.73, 1043.73, 276.33, 310.91, 460.17, 363.77, 509, 643.26, 385.61, 700.68, 380.62, 399.49, 999.22,
		687.26, 394.21, 435.21, 370.72, 368.13, 1073.73, 620.34, 413.09, 724.37, 384.03, 1057.04, 494.07, 465.74, 265.22, 343.3,
		398.54, 787.2, 706.83, 220.81, 562.43, 392.8, 299.57, 550.53, 383.69, 224.61, 613.05, 583.2, 573.43, 807.86, 398.69,
		367.65, 372.63, 618.67, 484.02, 568.35, 381.87, 480.74, 1298.7, 392.95, 1232, 462.8, 460.17, 418.24, 423.51, 817.22,
		210.76, 1578.39, 950.01, 422.07, 418.73, 927.08, 404.79, 625.1, 827.69, 614.58, 452.16, 450.73, 135.21, 394.61, 444.41,
		598.82, 269.25, 399.5, 766.95, 875.24, 425.64, 419.46, 547.15, 601.59, 815.53, 196.85, 635, 1266.05, 337.5, 399.61,
		359.02, 390.57, 279.53, 908.72, 713.27, 966.36, 590.54, 652.49, 637.82, 494.53, 741.38, 623.65, 690.26, 1661.55, 583.32,
		570.1, 874.32, 1452.93, 270.71, 305.45, 337.35, 290.42, 541.39, 723.59, 797.51, 715.82, 1022.68, 559.54, 479.41, 327.52,
		410.57, 393.37, 229.06, 342.57, 560.58, 364.3, 484.01, 464.56, 270.07, 561.35, 520.49, 387.24, 328.39, 657.68, 815.71,
		319.21, 460.17, 603.34, 286, 989.17, 655.48, 345.75, 609.58, 782.19, 355.36, 644.34, 243.44, 392.18, 568.93, 535.92,
		254.75, 1003.38, 415.57, 713.26, 661.5, 352.68, 1385.12, 738.8, 306.61, 582.37, 920.33, 224.55, 524.09, 487.77, 501.96,
		640.83, 548.79, 523.33, 177.36, 564.02, 714.27, 377.59, 468.92, 686.32, 280.7, 504.4, 364.18, 345.79, 193.38, 193.18,
		883.53, 362.13, 306.4, 771.5, 490.85, 580.32, 667.25, 601.65, 711.45, 370, 347.74, 967.62, 627.58, 617.6, 375.3,
		639.21, 721.38, 330.55, 675.71, 1143.05, 354.63, 645.85, 316.62, 493.4, 456.33, 801.7, 421.68, 704.08, 649.02, 663.33,
		642.39, 783.23, 438.7, 525.37, 723.49, 342.57, 235.2, 510, 577.77, 157.51, 509.47, 444.21, 629.09, 604.17, 398.82,
		678.4, 317.43, 434.61, 717.71, 525.71, 920.34, 494.28, 460, 598, 378.36, 824.58, 1538.43, 716, 434.56, 766.95,
		356.41, 727.54, 525, 737.3, 439.2, 376.26, 712.79, 491.79, 449.27, 1467.69, 484.66, 359.8, 531.76, 419.04, 793.33,
		515.67, 384.31, 1023.31, 444.83, 733.72, 715.82, 562, 603.34, 821.61, 1102, 823.19, 905.01, 733.77, 569.69, 818.08,
		751.6, 712.33, 329.63, 384, 281.22, 786.38, 324.6, 509.9, 785.76, 765.24, 692.57, 368.13, 377.16, 938.6, 710.95,
		556.38, 894, 362.28, 472.49, 617.6, 359.47, 328.64, 423.55, 359.92, 1505.66, 382.96, 275.19, 245.42, 664.69, 715.82,
		681.05, 387.95, 403.91, 271.05, 710.71, 521.33, 624.75, 542.12, 616.6, 594.64, 305.73, 443.88, 531.52, 242.35, 715.82,
		499.23, 175.99, 876.44, 353.69, 415.6, 403.93, 539.93, 356.88, 586.97, 338.48, 833.42, 976.58, 715.82, 834.15, 1133.94,
		319.46, 650.05, 547.1, 450.5, 675.24, 371.57, 616.45, 715.82, 848.23, 674.92, 603.34, 371.91, 327.75, 457.96, 736.27,
		1237.35, 484.06, 1416.96, 527.12, 423.22, 268.53, 725.13, 609.03, 785.36, 621.52, 1632.03, 1041.13, 561.38, 599.75, 516.27,
		591.18, 567.55, 316.56, 796.99, 290.49, 735.42, 352.44, 611.63, 429.49, 376.25, 510.24, 330.96, 493.83, 542.94, 362.93,
		401.37, 603.27, 328.03, 1094.47, 491.63, 395.62, 301.06, 613.56, 425.38, 478.23, 639.11, 640.99, 1198, 327.84, 299.45,
		565.49, 487.16, 321.56, 359.5, 447.23, 707.39, 526.98, 532.96, 652.9, 335.55, 444.23, 787.4, 124.47, 621.73, 843.65,
		637.59, 922.52, 582.38, 572.36, 960.64, 311.05, 376.65, 448.16, 438.92, 567.55, 415.18, 600.26, 728.19, 823.17, 554.44,
		392.68, 379.46, 615.88, 428.71, 983.99, 478.27, 553.66, 692.33, 526.01, 513.4, 826.45, 640.89, 399.54, 854.18, 576.51,
		743.37, 400.23, 366.06, 1018.38, 411.23, 262.41, 428.66, 286.14, 986.97, 702.64, 413.49, 299.62, 796.18, 542.08, 371.03,
		577.83, 717.4, 306.57, 393.97, 263.85, 268.39, 312, 458.74, 324.28, 506.19, 1018.22, 880.49, 409.04, 473.18, 439.08,
		500.24, 649.34, 484.03, 335.21, 706.73, 807.86, 664.69, 666.91, 340.01, 298.42, 486.37, 204.52, 788.91, 529.13, 210.05,
		299, 690.52, 340.98, 213.01, 480.63, 812.96, 449.94, 1457.2, 1000.41, 461.54, 287.14, 1027.79, 1002.8, 357.91, 479.63,
		478.07, 432.14, 531.2, 505.87, 409.05, 565.8, 685.78, 321.1, 897.43, 787.38, 352.8, 355.15, 789.25, 894.6, 478.9,
		444.67, 215.95, 554.76, 393.59, 646.8, 747.83, 427.86, 771.21, 376, 269.63, 654.46, 323.87, 493.85, 780.91, 951.02,
		618.67, 658.87, 553.35, 214.14, 389.49, 386.22, 328.63, 812.97, 810.55, 498.06, 559.67, 515.77, 812, 971.47, 360.57,
		269.7, 356.89, 256.67, 313.84, 485.43, 283.07, 869.85, 446.07, 130.35, 388.6, 696.09, 413.66, 444.83, 485.72, 580.28,
		1533.9, 713.39, 570.34, 516.55, 920.34, 606.9, 352.12, 570.67, 787.41, 302.84, 936.01, 869.57, 200.42, 409.96, 208.56,
		703.12, 971.47, 530.17, 396.23, 855.78, 151, 294, 389.23, 398.81, 515.58, 392.14, 611.66, 306.53, 327.05, 421.31,
		455.22, 414.58, 373.9, 389.95, 454.32, 155.59, 553.4, 629.18, 398.55, 109.32, 590.19, 536.86, 543.41, 583.53, 322.13,
		512.58, 688.51, 321.21, 896.82, 281.21, 758.37, 1252.69, 997.03, 345.54, 284.55, 361.7, 614.58, 779.73, 720.11, 591,
		404.18, 492.94, 611.56, 853.32, 588.74, 529.62, 518.61, 379.11, 1109.52, 368.13, 476.79, 485.93, 279.81, 605.6, 166.48,
		210.55, 670.34, 419.22, 701.46, 666.22, 383.47, 1135.96, 323.92, 620.44, 463.64, 807.75, 343.22, 275.52, 613.56, 594.93,
		820.88, 634.09, 470.52, 359.91, 355.5, 1227.12, 407.23, 388.59, 504.13, 695.37, 575.23, 677.98, 539.86, 414.97, 336.27,
		848.11, 752.08, 697.15, 422.46, 296.46, 899.95, 884.16, 463.97, 608.72, 342.57, 983.13, 347.98, 256.94, 251.43, 678.48,
		604.19, 724, 228.87, 1175.97, 554.99, 664.69, 370.61, 561.12, 840, 725.94, 517.97, 652.93, 250.4, 794.02, 514.35,
		438.69, 925.45, 299.52, 358.18, 1147.85, 900, 1575.71, 651.03, 582.89, 660.18, 562.42, 432.05, 802.93, 340.67, 538.1,
		720.59, 408.75, 654.78, 380.76, 1030.03, 536.07, 1015.18, 794.31, 172.23, 163.41, 235.96, 651.93, 631.62, 654.68, 303.45,
		847.8, 529.46, 897.98, 634.21, 401.89, 549.2, 111.97, 348, 746.5, 446.26, 585.03, 538.17, 556.43, 571.6, 1155.54,
		963.1, 319.56, 538.62, 502.31, 444.65, 603.34, 777.17, 640.86, 466.58, 271.17, 463, 753.02, 602.61, 373.34, 1101.85,
		317.62, 900.65, 377.87, 924.55, 562.43, 485.74, 314.09, 493.41, 222.41, 1532.99, 493.13, 351.86, 485.74, 511.3, 552.2,
		455.05, 271.9, 385.6, 265.88, 899.89, 473.64, 128.9, 388.92, 1136.95, 747.73, 248.34, 789.56, 893.25, 405.6, 458.85,
		536.86, 373.47, 388.81, 916.25, 345.29, 549.68, 290.71, 426.46, 434.68, 335.21, 309, 346.67, 423.45, 765.77, 403.92,
		474.76, 602.23, 323.3, 511.6, 98.85, 591.24, 518.51, 522.03, 373.37, 781.05, 296.75, 1068.13, 266.26, 483, 569,
		555.34, 307.53, 294.05, 301.09, 741.39, 625.69, 521.52, 408.21, 620.26, 449.97, 284.94, 608.45, 567.25, 356.16, 621.23,
		611.62, 689.78, 339.62, 617.27, 644.84, 383.26, 297.76, 290.2, 569.47, 454.09, 265.02, 670.99, 444.29, 710.71, 282.95,
		588, 1380.51, 1068.64, 383.47, 221.94, 706.24, 612.21, 613.56, 470.4, 869.21, 713.01, 475.5, 388.59, 346.69, 765,
		741.39, 392.67, 444.84, 791.86, 554, 770.34, 489.89, 104.14, 611.87, 444.84, 730.72, 412.21, 1482.77, 646.63, 273.92,
		325.85, 894.2, 649.55, 895, 431.54, 508.85, 603.33, 1088.36, 516.29, 383.48, 434.61, 307.81, 175.25, 547.92, 313.06,
		424, 747.68, 672.21, 118.09, 336, 522.08, 577.78, 495.96, 506.19, 261.38, 1342.17, 227.91, 501.7, 401.86, 480.61,
		1138.17, 301.15, 901.6, 879.66, 398.27, 383.48, 321.48, 501.08, 611.48, 284.37, 471.99, 373.34, 249.93, 545.73, 400.44,
		613.56, 524.85, 1062.18, 746.5, 430.06, 541.98, 546.4, 431.15, 945.91, 710.87, 518.87, 390.74, 440.85, 396.67, 322.12,
		588, 398.64, 562.43, 577.86, 798.42, 919.3, 567.54, 509.39, 443.6, 767.47, 465.28, 1459.49, 197.44, 375.7, 717.13,
		255.57, 282.24, 776.99, 828.19, 457.57, 1196.44, 588, 253.92, 1048.17, 396.7, 596.53, 769.51, 250.54, 296.78, 423.26,
		685.91, 319.84, 412.61, 513.12, 638.89, 869.21, 496.99, 475.5, 649.5, 295.72, 495.97, 461.54, 435.61, 875.73, 449.73,
		373.11, 661.61, 261.28, 644.23, 470, 499.75, 570.9, 818.08, 387.69, 426.06, 564.6, 801.09, 509.26, 731.16, 839.89,
		445.41, 273.75, 314.45, 935.55, 356.08, 472.74, 280.21, 357.91, 644.79, 501.07, 612.15, 541.98, 165.26, 242.36, 654.46,
		920.34, 285.73, 292.7, 510.79, 997.04, 738.29, 305.81, 794.11, 398.81, 349.05, 392.87, 322.88, 1150.42, 741.39, 748.55,
		752.8, 1149.09, 622.05, 788.16, 456.25, 719.74, 292.95, 429.49, 345.83, 801.96, 414.7, 548.39, 607.42, 559.52, 684.21,
		784.32, 613.56, 419.26, 470.4, 588, 233.58, 958.5, 330.25, 642.55, 480, 664.69, 352.71, 654.45, 1314.37, 272.24,
		381.5, 502.57, 481.34, 672.36, 674.74, 438.45, 766.95, 557.32, 512.88, 555.5, 418.23, 667.65, 712.76, 641.68, 432.71,
		642.51, 392.06, 632.38, 855.52, 695.2, 564.46, 967, 876.88, 482.77, 625.31, 528.68, 507.95, 664.69, 749.7, 520.33,
		675, 429.69, 598.41, 377.98, 446.85, 644.03, 710.7, 398.17, 869.21, 724.94, 567.88, 402.9, 329.68, 326.95, 356.1,
		503.72, 1184.64, 419.26, 848.98, 698.41, 490.34, 411.6, 479.01, 826.3, 368.13, 869.21, 630.83, 1000.12, 1135.07, 515.39,
		922.14, 582.89, 306.78, 552.19, 1173.33, 373.18, 855.85, 145.31, 120.8, 775.49, 446.44, 546.58, 425.63, 490.31, 313.69,
		706.84, 475.51, 1210.4, 706.19, 360, 1519.2, 598.21, 797.11, 422.53, 632.45, 287.2, 804.79, 554.76, 484.6, 428.07,
		1002.15, 452.02, 937.49, 920.46, 847.38, 416.91, 294.37, 265.88, 587.99, 413.1, 336.45, 814.93, 702, 523.8, 279.04,
		510.8, 486.76, 894.76, 588.77, 777.17, 637.65, 296.4, 542, 222.5, 271.77, 194.92, 382.42, 285.07, 286.8, 706.65,
		558.06, 797.62, 838.53, 417.73, 884.55, 680.03, 340.52, 461.19, 938.73, 560.9, 807.29, 291.33, 1034.33, 567.55, 696.39,
		588.82, 764.61, 690, 655.76, 380.25, 498.51, 959.56, 400.05, 1140, 522.96, 306.78, 112.08, 700.38, 491.12, 521.48,
		466.81, 649.35, 383.48, 766.95, 559.51, 396.26, 613.53, 685.14, 574.96, 838.53, 912.72, 301.97, 706.89, 613.56, 353.41,
		295.19, 335.48, 519.82, 772.69, 419.01, 448.34, 325.84, 339.09, 819.74, 779.37, 574.65, 775.54, 627.11, 650.89, 961.23,
		605.58, 592.67, 691.01, 970.05, 383.94, 434.37, 534.3, 359.75, 530.63, 664.69, 183.09, 374.15, 568.5, 516.41, 719.5,
		362.86, 766.95, 499.95, 657.51, 591.62, 724.65, 781.8, 700.48, 927.59, 1046.87, 433.04, 346.48, 584.79, 357.83, 766.95,
		367.01, 530.91, 667.92, 327.23, 817.97, 702, 957.26, 281.21, 178.95, 337.46, 390.75, 493.41, 423.01, 1278.25, 521.53,
		361.91, 992.72, 945.91, 583.91, 105.9, 522.75, 1329.38, 1022.6, 674.63, 727.66, 601.71, 601.02, 1527.3, 524.76, 489.44,
		732.8, 949.65, 482.83, 375.62, 621.72, 700.99, 771.76, 693.59, 874.32, 362, 808.25, 922.64, 704.44, 486.1, 608.44,
		692.81, 512.37, 639.13, 1022.16, 421.73, 398.88, 399.43, 390.52, 611.82, 787.4, 629.7, 811.93, 765.87, 391.14, 772.48,
		403.04, 569.08, 606.35, 591.71, 917.02, 483.18, 421.61, 590.96, 458.35, 505.72, 613.56, 559.37, 338.95, 455.06, 251.15,
		673.96, 764.94, 914.73, 464.6, 455.28, 409.42, 483.19, 663.85, 656.83, 269.93, 768.54, 613.55, 314.38, 222.42, 330.12,
		492.41, 629.49, 613.56, 537.9, 530.62, 812.96, 352.8, 373.19, 741.38, 777.18, 745.82, 418.63, 317, 209.3, 505.7,
		1214.8, 1278.25, 388.59, 664.69, 386.91, 272.62, 674.92, 441.29, 322.51, 603.56, 396.73, 292.64, 500.18, 1159.36, 373.56,
		586.29, 633.65, 313.66, 839.77, 1069.68, 326.86, 783.44, 466.36, 342.57, 697.71, 668.24, 480.63, 511.93, 796.61, 623.79,
		534.05, 396.87, 843.64, 624.96, 671.34, 846.58, 420.26, 1088.74, 447.1, 310.4, 997.04, 613.12, 486.26, 380.13, 383.48,
		398.81, 466.79, 1012.58, 521.75, 483.18, 855.03, 541.98, 218.11, 406.93, 276.2, 813.56, 320.08, 489.83, 626.5, 305.31,
		555.55, 289.08, 392.94, 532.56, 988, 397.98, 470.39, 505.99, 260.15, 802.99, 271.16, 567.54, 323.42, 506.19,
	}
	sort.Float64s(points)
	hc.AddData("Nettomiete", points, chart.Style{})
	dumper.Plot(&hc)

	hc.Kernel = chart.BisquareKernel
	hc.Title = "Bisquare Kernel"
	dumper.Plot(&hc)

	hc.Title = "Gauss Kernel"
	hc.Kernel = chart.GaussKernel
	dumper.Plot(&hc)

	hc.Title = "Epanechnikov Kernel"
	hc.Kernel = chart.EpanechnikovKernel
	dumper.Plot(&hc)

	hc.Title = "Rectangular Kernel"
	hc.Kernel = chart.RectangularKernel
	dumper.Plot(&hc)

}

//
//  Main
//
func main() {
	var debugging = flag.Bool("debug", false, "output debug information to stderr")

	var All *bool = flag.Bool("All", false, "show everything")
	var all *bool = flag.Bool("all", false, "show all basic chart types")
	var catBar *bool = flag.Bool("cat", false, "show categorical bar charts")
	var bar *bool = flag.Bool("bar", false, "show bar charts")
	var box *bool = flag.Bool("box", false, "show box charts")
	var strip *bool = flag.Bool("strip", false, "show strip charts")
	var pie *bool = flag.Bool("pie", false, "show pie charts")
	var scatter *bool = flag.Bool("scatter", false, "show scatter charts")
	var hist *bool = flag.Bool("hist", false, "show hist charts")
	var shist *bool = flag.Bool("shist", false, "show stacked hist charts")
	var ohist *bool = flag.Bool("ohist", false, "show offsetted hist charts")
	var mieten *bool = flag.Bool("mieten", false, "show Nettomieten histogram")

	var special *bool = flag.Bool("special", false, "show all special stuff")
	var logc *bool = flag.Bool("log", false, "show logarithmic axis")
	var tics *bool = flag.Bool("tics", false, "show tics")
	var auto *bool = flag.Bool("auto", false, "show autoscaling")
	var key *bool = flag.Bool("key", false, "show key placement")
	var funcs *bool = flag.Bool("func", false, "show function plots")
	var best *bool = flag.Bool("best", false, "show best of plots")
	var zeit *bool = flag.Bool("time", false, "show time plots")
	var test *bool = flag.Bool("test", false, "produce graphic test")

	flag.Parse()
	if *debugging {
		chart.DebugLogger = log.New(os.Stdout, "", log.LstdFlags)
	}

	if *All {
		*all = true
		*special = true
		*best = true
		*zeit = true
	}

	// Basic chart types
	if *all || *catBar {
		categoricalBarChart()
	}
	if *all || *bar {
		barChart()
	}
	if *all || *box {
		boxChart()
	}

	if *all || *strip {
		stripChart()
	}

	if *all || *pie {
		pieChart()
	}

	if *all || *scatter {
		scatterChart()
	}
	if *all || *hist {
		histChart("xhistn1.svg", "Normal Histogram", false, false, false)
		histChart("xhistn2.svg", "Normal Histogram", false, true, false)
	}
	if *all || *shist {
		histChart("xhists1.svg", "Stacked Histogram", true, false, false)
		histChart("xhists2.svg", "Stacked Histogram", true, true, false)
	}
	if *all || *ohist {
		histChart("xhisto1.svg", "Offseted + Stacked Histogram", true, true, true)
		histChart("xhisto2.svg", "Offseted Histogram", false, true, true)
	}
	if *all || *mieten {
		mietenChart()
	}

	// Some specialities
	if *special || *logc {
		logAxis()
	}
	if *special || *tics {
		scatterTics()
	}
	if *special || *auto {
		autoscale()
	}
	if *special || *key {
		keyStyles()
	}
	if *special || *funcs {
		functionPlots()
	}
	if *special || *test {
		testGraphics()
	}

	if *best {
		bestOf()
	}

	if *zeit {
		timeRange()
	}

	/*
			// Helper to determine parameters of fonts
			textlen()
		        kernels()
	*/
}
