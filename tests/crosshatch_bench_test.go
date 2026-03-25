package tests

import (
	"image"
	"image/color"
	"testing"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
)

// Test polygons of varying complexity.
var (
	// Simple square zone ~100x100 pixels.
	smallSquare = [][2]float64{
		{50, 50}, {150, 50}, {150, 150}, {50, 150},
	}

	// Larger square ~400x400 pixels.
	largeSquare = [][2]float64{
		{50, 50}, {450, 50}, {450, 450}, {50, 450},
	}

	// Irregular polygon with 8 vertices.
	irregularPoly = [][2]float64{
		{100, 50}, {200, 30}, {300, 80}, {320, 200},
		{280, 350}, {150, 380}, {60, 300}, {40, 150},
	}

	// Complex polygon with 16 vertices (star-like).
	complexPoly = [][2]float64{
		{250, 50}, {280, 150}, {380, 150}, {300, 220},
		{330, 320}, {250, 260}, {170, 320}, {200, 220},
		{120, 150}, {220, 150}, {250, 50}, {260, 130},
		{340, 170}, {290, 240}, {310, 340}, {250, 280},
	}
)

func makeCtx(w, h int) (*canvas.Context, *image.RGBA) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rast := rasterizer.FromImage(img, canvas.DPMM(1.0), canvas.DefaultColorSpace)
	ctx := canvas.NewContext(rast)
	ctx.SetCoordSystem(canvas.CartesianIV)
	return ctx, img
}

// --- Canvas pattern benchmarks ---

func drawCrosshatchPattern(ctx *canvas.Context, polygon [][2]float64, spacing float64, c color.RGBA) {
	if len(polygon) < 3 {
		return
	}

	clip := &canvas.Path{}
	clip.MoveTo(polygon[0][0], polygon[0][1])
	for _, pt := range polygon[1:] {
		clip.LineTo(pt[0], pt[1])
	}
	clip.Close()

	hatch := canvas.NewCrossHatch(c, 45.0, 135.0, spacing, spacing, 1)
	ctx.SetFillColor(c)
	ctx.SetStrokeColor(canvas.Transparent)
	ctx.DrawPath(0, 0, hatch.Tile(clip))
}

func Benchmark_Crosshatch_Pattern_SmallSquare(b *testing.B) {
	ctx, _ := makeCtx(200, 200)
	c := color.RGBA{A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawCrosshatchPattern(ctx, smallSquare, 10, c)
	}
}

func Benchmark_Crosshatch_Pattern_LargeSquare(b *testing.B) {
	ctx, _ := makeCtx(500, 500)
	c := color.RGBA{A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawCrosshatchPattern(ctx, largeSquare, 10, c)
	}
}

func Benchmark_Crosshatch_Pattern_IrregularPoly(b *testing.B) {
	ctx, _ := makeCtx(400, 400)
	c := color.RGBA{A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawCrosshatchPattern(ctx, irregularPoly, 10, c)
	}
}

func Benchmark_Crosshatch_Pattern_ComplexPoly(b *testing.B) {
	ctx, _ := makeCtx(500, 500)
	c := color.RGBA{A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawCrosshatchPattern(ctx, complexPoly, 10, c)
	}
}
