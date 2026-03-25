package canvas

import (
	"image/color"
	"math"
	"sort"
)

type Pattern interface {
	Transform(Matrix) Pattern
	SetColorSpace(ColorSpace) Pattern
	RenderTo(Renderer, *Path)
}

//type CanvasPattern struct {
//	c    *Canvas
//	cell Matrix
//}
//
//func NewPattern(c *Canvas, cell Matrix) *CanvasPattern {
//	return &CanvasPattern{
//		c:    c,
//		cell: cell,
//	}
//}
//
//func (p *CanvasPattern) ClipTo(r Renderer, clip *Path) {
//	//fmt.Println("src", p.c.Size())
//	//fmt.Println("dst", r.Size())
//	//fmt.Println("matrix", p.m)
//	// TODO: tile
//	p.c.RenderViewTo(r, p.cell)
//}

//type ImagePattern struct {
//	img  *image.RGBA
//	cell Matrix
//}
//
//func NewImagePattern() *ImagePattern {
//	return &ImagePattern{}
//}
//
//func (p *ImagePattern) ClipTo(r Renderer, clip *Path) {
//}

// Hatch pattern is a filling hatch pattern.
type HatchPattern struct {
	Fill      Paint
	Thickness float64
	cell      Matrix
	hatch     Hatcher
}

// Hatcher is a hatch pattern along the cell's axes. The rectangle (x0,y0)-(x1,y1) is expressed in the unit cell's coordinate system, and the returned path should be transformed by the cell to obtain the final hatch pattern.
type Hatcher func(float64, float64, float64, float64) *Path

// NewHatchPattern returns a new hatch pattern.
func NewHatchPattern(ifill interface{}, thickness float64, cell Matrix, hatch Hatcher) *HatchPattern {
	var fill Paint
	if paint, ok := ifill.(Paint); ok {
		fill = paint
	} else if pattern, ok := ifill.(Pattern); ok {
		fill = Paint{Pattern: pattern}
	} else if gradient, ok := ifill.(Gradient); ok {
		fill = Paint{Gradient: gradient}
	} else if col, ok := ifill.(color.Color); ok {
		fill = Paint{Color: rgbaColor(col)}
	}
	if fill.IsPattern() {
		panic("hatch paint cannot be pattern")
	}
	return &HatchPattern{
		Fill:      fill,
		Thickness: thickness,
		cell:      cell,
		hatch:     hatch,
	}
}

// Transform sets the view. Automatically called by Canvas for coordinate system transformations.
func (p *HatchPattern) Transform(view Matrix) Pattern {
	return p
}

// SetColorSpace sets the color space. Automatically called by the rasterizer.
func (p *HatchPattern) SetColorSpace(colorSpace ColorSpace) Pattern {
	if _, ok := colorSpace.(LinearColorSpace); ok {
		return p
	}

	if p.Fill.IsGradient() {
		p.Fill.Gradient.SetColorSpace(colorSpace)
	} else if p.Fill.IsColor() {
		p.Fill.Color = colorSpace.ToLinear(p.Fill.Color)
	}
	return p
}

// Tile tiles the hatch pattern within the clipping path.
func (p *HatchPattern) Tile(clip *Path) *Path {
	dst := clip.FastBounds()

	// find extremes along cell axes
	invCell := p.cell.Inv()
	points := []Point{
		invCell.Dot(Point{dst.X0 - p.Thickness, dst.Y0 - p.Thickness}),
		invCell.Dot(Point{dst.X1 + p.Thickness, dst.Y0 - p.Thickness}),
		invCell.Dot(Point{dst.X1 + p.Thickness, dst.Y1 + p.Thickness}),
		invCell.Dot(Point{dst.X0 - p.Thickness, dst.Y1 + p.Thickness}),
	}
	x0, x1 := points[0].X, points[0].X
	y0, y1 := points[0].Y, points[0].Y
	for _, point := range points[1:] {
		x0 = math.Min(x0, point.X)
		x1 = math.Max(x1, point.X)
		y0 = math.Min(y0, point.Y)
		y1 = math.Max(y1, point.Y)
	}

	hatch := p.hatch(x0, y0, x1, y1)
	hatch = hatch.Transform(p.cell)

	// clip hatch lines directly against the polygon and stroke without
	// settling; this avoids the expensive Bentley-Ottmann boolean operations
	hatch = clipHatchLines(hatch, clip)
	if p.Thickness != 0.0 {
		origFastStroke := FastStroke
		FastStroke = true
		hatch = hatch.Stroke(p.Thickness, ButtCap, MiterJoin, 0.01)
		FastStroke = origFastStroke
	}
	return hatch
}

// clipHatchLines clips straight line segments in hatch against the filled
// region of clip using simple line-edge intersection. This is O(lines*edges)
// and much faster than the general-purpose And boolean operation for the
// typical case of many short lines against a modest polygon.
func clipHatchLines(hatch, clip *Path) *Path {
	// flatten clip path to line segments and extract edges
	flatClip := clip.Flatten(Tolerance)
	type edge struct {
		x0, y0, x1, y1 float64
	}
	var edges []edge
	var sx, sy, cx, cy float64
	scanner := flatClip.Scanner()
	for scanner.Scan() {
		end := scanner.End()
		switch scanner.Cmd() {
		case MoveToCmd:
			sx, sy = end.X, end.Y
			cx, cy = end.X, end.Y
		case LineToCmd:
			edges = append(edges, edge{cx, cy, end.X, end.Y})
			cx, cy = end.X, end.Y
		case CloseCmd:
			if cx != sx || cy != sy {
				edges = append(edges, edge{cx, cy, sx, sy})
			}
			cx, cy = sx, sy
		}
	}
	if len(edges) == 0 {
		return &Path{}
	}

	// point-in-polygon test using horizontal ray casting (even-odd rule)
	inside := func(px, py float64) bool {
		crossings := 0
		for _, e := range edges {
			if (e.y0 <= py && e.y1 > py) || (e.y1 <= py && e.y0 > py) {
				t := (py - e.y0) / (e.y1 - e.y0)
				if px < e.x0+t*(e.x1-e.x0) {
					crossings++
				}
			}
		}
		return crossings%2 == 1
	}

	result := &Path{}
	var ts []float64
	scanner = hatch.Scanner()
	var hx0, hy0 float64
	for scanner.Scan() {
		end := scanner.End()
		switch scanner.Cmd() {
		case MoveToCmd:
			hx0, hy0 = end.X, end.Y
		case LineToCmd:
			hx1, hy1 := end.X, end.Y
			dx, dy := hx1-hx0, hy1-hy0

			// find intersections with clip polygon edges
			ts = ts[:0]
			for _, e := range edges {
				edx, edy := e.x1-e.x0, e.y1-e.y0
				denom := dx*edy - dy*edx
				if math.Abs(denom) < 1e-12 {
					continue
				}
				t := ((e.x0-hx0)*edy - (e.y0-hy0)*edx) / denom
				u := ((e.x0-hx0)*dy - (e.y0-hy0)*dx) / denom
				if t > 0 && t < 1 && u >= 0 && u <= 1 {
					ts = append(ts, t)
				}
			}
			sort.Float64s(ts)

			// walk segments between intersections, keeping those inside
			prev := 0.0
			isInside := inside(hx0+1e-6*dx, hy0+1e-6*dy)
			for _, t := range ts {
				if isInside {
					result.MoveTo(hx0+prev*dx, hy0+prev*dy)
					result.LineTo(hx0+t*dx, hy0+t*dy)
				}
				prev = t
				isInside = !isInside
			}
			if isInside {
				result.MoveTo(hx0+prev*dx, hy0+prev*dy)
				result.LineTo(hx1, hy1)
			}

			hx0, hy0 = hx1, hy1
		}
	}
	return result
}

// RenderTo tiles the hatch pattern to the clipping path and renders it to the renderer.
func (p *HatchPattern) RenderTo(r Renderer, clip *Path) {
	hatch := p.Tile(clip)
	r.RenderPath(hatch, Style{Fill: p.Fill}, Identity)
}

// NewLineHatch returns a new line hatch pattern with lines at an angle with a spacing of distance. Thickness is the stroke thickness applied to the shape; stroking is ignored with thickness is zero.
func NewLineHatch(ifill interface{}, angle, distance, thickness float64) *HatchPattern {
	cell := Identity.Rotate(angle).Scale(distance, distance)
	return NewHatchPattern(ifill, thickness, cell, func(x0, y0, x1, y1 float64) *Path {
		p := &Path{}
		for y := math.Floor(y0); y <= y1; y += 1.0 {
			p.MoveTo(x0, y)
			p.LineTo(x1, y)
		}
		return p
	})
}

// NewCrossHatch returns a new cross hatch pattern of two regular line hatches at different angles and with different distance intervals. Thickness is the stroke thickness applied to the shape; stroking is ignored with thickness is zero.
func NewCrossHatch(ifill interface{}, angle0, angle1, distance0, distance1, thickness float64) *HatchPattern {
	cell := PrimitiveCell(
		Point{distance0, 0.0}.Rot(angle0*math.Pi/180.0, Origin),
		Point{distance1, 0.0}.Rot(angle1*math.Pi/180.0, Origin),
	)
	return NewHatchPattern(ifill, thickness, cell, func(x0, y0, x1, y1 float64) *Path {
		p := &Path{}
		for y := math.Floor(y0); y <= y1; y += 1.0 {
			p.MoveTo(x0, y)
			p.LineTo(x1, y)
		}
		for x := math.Floor(x0); x <= x1; x += 1.0 {
			p.MoveTo(x, y0)
			p.LineTo(x, y1)
		}
		return p
	})
}

// NewShapeHatch returns a new shape hatch that repeats the given shape over a rhombus primitive cell with sides of length distance. Thickness is the stroke thickness applied to the shape; stroking is ignored with thickness is zero.
func NewShapeHatch(ifill interface{}, shape *Path, distance, thickness float64) *HatchPattern {
	d := distance * math.Sin(60.0*math.Pi/180.0)
	cell := SquareCell(1.0)
	return NewHatchPattern(ifill, thickness, cell, func(x0, y0, x1, y1 float64) *Path {
		p := &Path{}
		for y := math.Floor(y0/distance) * distance; y <= y1; y += 2.0 * d {
			for x := math.Floor(x0/distance) * distance; x <= x1; x += distance {
				p = p.Append(shape.Copy().Translate(x, y))
			}
			for x := (math.Floor(x0/distance) + 0.5) * distance; x <= x1; x += distance {
				p = p.Append(shape.Copy().Translate(x, y+d))
			}
		}
		return p
	})
}

// ImagePattern is an image tiling pattern of an image drawn from an origin with a certain resolution. Higher resolution will give smaller tilings.
//type ImagePattern struct {
//	img    *image.RGBA
//	res    Resolution
//	origin Point
//}
//
//// NewImagePattern returns a new image pattern.
//func NewImagePattern(iimg image.Image, res Resolution, origin Point) *ImagePattern {
//	img, ok := iimg.(*image.RGBA)
//	if !ok {
//		bounds := iimg.Bounds()
//		img = image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
//		draw.Draw(img, img.Bounds(), iimg, bounds.Min, draw.Src)
//	}
//	return &ImagePattern{
//		img:    img,
//		res:    res,
//		origin: origin,
//	}
//}
//
//// SetColorSpace returns the linear gradient with the given color space. Automatically called by the rasterizer.
//func (p *ImagePattern) SetColorSpace(colorSpace ColorSpace) Pattern {
//	if _, ok := colorSpace.(LinearColorSpace); ok {
//		return p
//	}
//	// TODO: optimize
//	pattern := *p
//	bounds := p.img.Bounds()
//	pattern.img = image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
//	draw.Draw(pattern.img, pattern.img.Bounds(), p.img, bounds.Min, draw.Src)
//	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
//		for x := bounds.Min.X; x < bounds.Max.X; x++ {
//			col := pattern.img.RGBAAt(x, y)
//			col = colorSpace.ToLinear(col)
//			pattern.img.SetRGBA(x, y, col)
//		}
//	}
//	return &pattern
//}
//
//// At returns the color at position (x,y).
//func (p *ImagePattern) At(x, y float64) color.RGBA {
//	x = (x - p.origin.X) * p.res.DPMM()
//	y = (y - p.origin.Y) * p.res.DPMM()
//
//	var s [4]uint8
//	ix0, iy0 := int(x), int(y)
//	fx, fy := x-float64(ix0), y-float64(iy0)
//	ix0 = ix0 % p.img.Bounds().Dx()
//	iy0 = iy0 % p.img.Bounds().Dy()
//	ix1 := (ix0 + 1) % p.img.Bounds().Dx()
//	iy1 := (iy0 + 1) % p.img.Bounds().Dy()
//	d00 := p.img.PixOffset(ix0, iy0)
//	d10 := p.img.PixOffset(ix1, iy0)
//	d01 := p.img.PixOffset(ix0, iy1)
//	d11 := p.img.PixOffset(ix1, iy1)
//	for i := 0; i < 4; i++ {
//		s[i] = uint8((1.0-fy)*((1.0-fx)*float64(p.img.Pix[d00+i])+fx*float64(p.img.Pix[d10+i])) + fy*((1.0-fx)*float64(p.img.Pix[d01+i])+fx*float64(p.img.Pix[d11+i])) + 0.5)
//	}
//	return color.RGBA{s[0], s[1], s[2], s[3]}
//}
