// Copyright 2010 The draw2d Authors. All rights reserved.
// created: 21/11/2010 by Laurent Le Goff

package draw2d

import (
	"image"
	"image/color"
)

// GraphicContext describes the interface for the various backends (images, pdf, opengl, ...)
type GraphicContext interface {
	PathBuilder
	// BeginPath creates a new path
	BeginPath()
	// GetMatrixTransform returns the current transformation matrix
	GetMatrixTransform() Matrix
	// SetMatrixTransform sets the current transformation matrix
	SetMatrixTransform(tr Matrix)
	// ComposeMatrixTransform composes the current transformation matrix with tr
	ComposeMatrixTransform(tr Matrix)
	// Rotate applies a rotation to the current transformation matrix. angle is in radian.
	Rotate(angle float64)
	// Translate applies a translation to the current transformation matrix.
	Translate(tx, ty float64)
	// Scale applies a scale to the current transformation matrix.
	Scale(sx, sy float64)
	// SetStrokeColor sets the current stroke color
	SetStrokeColor(c color.Color)
	// SetStrokeColor sets the current fill color
	SetFillColor(c color.Color)
	// SetFillRule sets the current fill rule
	SetFillRule(f FillRule)
	// SetLineWidth sets the current line width
	SetLineWidth(lineWidth float64)
	// SetLineCap sets the current line cap
	SetLineCap(cap LineCap)
	// SetLineJoin sets the current line join
	SetLineJoin(join LineJoin)
	// SetLineJoin sets the current dash
	SetLineDash(dash []float64, dashOffset float64)
	// SetFontSize
	SetFontSize(fontSize float64)
	GetFontSize() float64
	SetFontData(fontData FontData)
	GetFontData() FontData
	DrawImage(image image.Image)
	Save()
	Restore()
	Clear()
	ClearRect(x1, y1, x2, y2 int)
	SetDPI(dpi int)
	GetDPI() int
	GetStringBounds(s string) (left, top, right, bottom float64)
	CreateStringPath(text string, x, y float64) (cursor float64)
	FillString(text string) (cursor float64)
	FillStringAt(text string, x, y float64) (cursor float64)
	StrokeString(text string) (cursor float64)
	StrokeStringAt(text string, x, y float64) (cursor float64)
	Stroke(paths ...*Path)
	Fill(paths ...*Path)
	FillStroke(paths ...*Path)
}
