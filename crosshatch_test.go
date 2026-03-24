package canvas

import (
	"image/color"
	"testing"
	"time"
)

func TestCrossHatchPanic(t *testing.T) {
	// This should not panic
	clip := Rectangle(100, 100)
	pattern := NewCrossHatch(color.RGBA{A: 255}, 45, -45, 10, 10, 1)
	
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panicked: %v", r)
			}
		}()
		pattern.Tile(clip)
	}()
	
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out - likely infinite loop")
	}
}

func TestCrossHatchHang(t *testing.T) {
	// This should not hang
	clip := Rectangle(100, 100)
	pattern := NewCrossHatch(color.Black, 45.0, 135.0, 5.0, 5.0, 0.3)

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panicked: %v", r)
			}
		}()
		pattern.Tile(clip)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out - likely infinite loop")
	}
}
