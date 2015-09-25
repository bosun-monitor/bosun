// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package font defines an interface for font faces, for drawing text on an
// image.
//
// Other packages provide font face implementations. For example, a truetype
// package would provide one based on .ttf font files.
package font // import "bosun.org/_third_party/golang.org/x/image/font"

import (
	"image"
	"image/draw"
	"io"

	"bosun.org/_third_party/golang.org/x/image/math/fixed"
)

// TODO: who is responsible for caches (glyph images, glyph indices, kerns)?
// The Drawer or the Face?

// Face is a font face. Its glyphs are often derived from a font file, such as
// "Comic_Sans_MS.ttf", but a face has a specific size, style, weight and
// hinting. For example, the 12pt and 18pt versions of Comic Sans are two
// different faces, even if derived from the same font file.
//
// A Face is not safe for concurrent use by multiple goroutines, as its methods
// may re-use implementation-specific caches and mask image buffers.
//
// To create a Face, look to other packages that implement specific font file
// formats.
type Face interface {
	io.Closer

	// Glyph returns the draw.DrawMask parameters (dr, mask, maskp) to draw r's
	// glyph at the sub-pixel destination location dot, and that glyph's
	// advance width.
	//
	// It returns !ok if the face does not contain a glyph for r.
	//
	// The contents of the mask image returned by one Glyph call may change
	// after the next Glyph call. Callers that want to cache the mask must make
	// a copy.
	Glyph(dot fixed.Point26_6, r rune) (
		dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool)

	// GlyphBounds returns the bounding box of r's glyph, drawn at a dot equal
	// to the origin, and that glyph's advance width.
	//
	// It returns !ok if the face does not contain a glyph for r.
	//
	// The glyph's ascent and descent equal -bounds.Min.Y and +bounds.Max.Y. A
	// visual depiction of what these metrics are is at
	// https://developer.apple.com/library/mac/documentation/TextFonts/Conceptual/CocoaTextArchitecture/Art/glyph_metrics_2x.png
	GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool)

	// GlyphAdvance returns the advance width of r's glyph.
	//
	// It returns !ok if the face does not contain a glyph for r.
	GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool)

	// Kern returns the horizontal adjustment for the kerning pair (r0, r1). A
	// positive kern means to move the glyphs further apart.
	Kern(r0, r1 rune) fixed.Int26_6

	// TODO: per-font Metrics.
	// TODO: ColoredGlyph for various emoji?
	// TODO: Ligatures? Shaping?
}

// TODO: Drawer.Layout or Drawer.Measure methods to measure text without
// drawing?

// Drawer draws text on a destination image.
//
// A Drawer is not safe for concurrent use by multiple goroutines, since its
// Face is not.
type Drawer struct {
	// Dst is the destination image.
	Dst draw.Image
	// Src is the source image.
	Src image.Image
	// Face provides the glyph mask images.
	Face Face
	// Dot is the baseline location to draw the next glyph. The majority of the
	// affected pixels will be above and to the right of the dot, but some may
	// be below or to the left. For example, drawing a 'j' in an italic face
	// may affect pixels below and to the left of the dot.
	Dot fixed.Point26_6

	// TODO: Clip image.Image?
	// TODO: SrcP image.Point for Src images other than *image.Uniform? How
	// does it get updated during DrawString?
}

// TODO: should DrawString return the last rune drawn, so the next DrawString
// call can kern beforehand? Or should that be the responsibility of the caller
// if they really want to do that, since they have to explicitly shift d.Dot
// anyway?
//
// In general, we'd have a DrawBytes([]byte) and DrawRuneReader(io.RuneReader)
// and the last case can't assume that you can rewind the stream.
//
// TODO: how does this work with line breaking: drawing text up until a
// vertical line? Should DrawString return the number of runes drawn?

// DrawString draws s at the dot and advances the dot's location.
func (d *Drawer) DrawString(s string) {
	var prevC rune
	for i, c := range s {
		if i != 0 {
			d.Dot.X += d.Face.Kern(prevC, c)
		}
		dr, mask, maskp, advance, ok := d.Face.Glyph(d.Dot, c)
		if !ok {
			// TODO: is falling back on the U+FFFD glyph the responsibility of
			// the Drawer or the Face?
			// TODO: set prevC = '\ufffd'?
			continue
		}
		draw.DrawMask(d.Dst, dr, d.Src, image.Point{}, mask, maskp, draw.Over)
		d.Dot.X += advance
		prevC = c
	}
}

// MeasureString returns how far dot would advance by drawing s.
func (d *Drawer) MeasureString(s string) (advance fixed.Int26_6) {
	var prevC rune
	for i, c := range s {
		if i != 0 {
			advance += d.Face.Kern(prevC, c)
		}
		a, ok := d.Face.GlyphAdvance(c)
		if !ok {
			// TODO: is falling back on the U+FFFD glyph the responsibility of
			// the Drawer or the Face?
			// TODO: set prevC = '\ufffd'?
			continue
		}
		advance += a
		prevC = c
	}
	return advance
}

// Hinting selects how to quantize a vector font's glyph nodes.
//
// Not all fonts support hinting.
type Hinting int

const (
	HintingNone Hinting = iota
	HintingVertical
	HintingFull
)

// Stretch selects a normal, condensed, or expanded face.
//
// Not all fonts support stretches.
type Stretch int

const (
	StretchUltraCondensed Stretch = -4
	StretchExtraCondensed Stretch = -3
	StretchCondensed      Stretch = -2
	StretchSemiCondensed  Stretch = -1
	StretchNormal         Stretch = +0
	StretchSemiExpanded   Stretch = +1
	StretchExpanded       Stretch = +2
	StretchExtraExpanded  Stretch = +3
	StretchUltraExpanded  Stretch = +4
)

// Style selects a normal, italic, or oblique face.
//
// Not all fonts support styles.
type Style int

const (
	StyleNormal Style = iota
	StyleItalic
	StyleOblique
)

// Weight selects a normal, light or bold face.
//
// Not all fonts support weights.
type Weight int

const (
	WeightThin       Weight = 100
	WeightExtraLight Weight = 200
	WeightLight      Weight = 300
	WeightNormal     Weight = 400
	WeightMedium     Weight = 500
	WeightSemiBold   Weight = 600
	WeightBold       Weight = 700
	WeightExtraBold  Weight = 800
	WeightBlack      Weight = 900
)
