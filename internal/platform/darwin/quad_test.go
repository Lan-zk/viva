//go:build darwin

package darwin

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/ui"
)

// TestMakeNativeTextLabelsCarriesQuadAndTint pins that a warped label crosses
// the boundary with its four corners and four colours, and that an ordinary one
// leaves the quad empty so the native side falls back to its box.
func TestMakeNativeTextLabelsCarriesQuadAndTint(t *testing.T) {
	warpped := TextLabel{
		Value: "In the beginning", X: 10, Y: 20, Width: 200, Height: 24, FontSize: 16,
		HasQuad: true,
		Quad:    [4]ui.Point{{X: 10, Y: 20}, {X: 210, Y: 24}, {X: 12, Y: 44}, {X: 208, Y: 40}},
		Tint: [4]Color{
			{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			{Red: .5, Green: .4, Blue: .3, Alpha: .9},
			{Red: .2, Green: .1, Blue: 0, Alpha: .5},
			{Red: 0, Green: 0, Blue: 0, Alpha: .25},
		},
	}
	plain := TextLabel{Value: "plain", X: 0, Y: 0, Width: 100, Height: 20, FontSize: 14}
	memory, strings, count, err := makeNativeTextLabels([]TextLabel{warpped, plain})
	if err != nil {
		t.Fatalf("makeNativeTextLabels failed: %v", err)
	}
	defer freeNativeTextLabels(memory, strings)
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	// The quad and its tints close the native structure: eight corner floats
	// followed by sixteen colour floats. Reading them back by offset pins where
	// they sit as well as what they hold.
	_, _, textSize, _, _ := compositeABI()
	const (
		quadFloats = 8
		tintFloats = 16
	)
	start := unsafe.Pointer(uintptr(memory) + textSize - (quadFloats+tintFloats)*4)
	quad := func(index int) [8]float32 {
		return *(*[8]float32)(unsafe.Add(start, uintptr(index)*textSize))
	}
	tint := func(index int) [16]float32 {
		return *(*[16]float32)(unsafe.Add(start, uintptr(index)*textSize+quadFloats*4))
	}
	packed := quad(0)
	for index, point := range warpped.Quad {
		if packed[index*2] != point.X || packed[index*2+1] != point.Y {
			t.Fatalf("corner %d = (%v, %v), want (%v, %v)", index, packed[index*2], packed[index*2+1], point.X, point.Y)
		}
	}
	packedTint := tint(0)
	for index, color := range warpped.Tint {
		want := [4]float32{float32(color.Red), float32(color.Green), float32(color.Blue), float32(color.Alpha)}
		got := [4]float32{packedTint[index*4], packedTint[index*4+1], packedTint[index*4+2], packedTint[index*4+3]}
		if got != want {
			t.Fatalf("tint %d = %v, want %v", index, got, want)
		}
	}
	if plainQuad := quad(1); plainQuad != [8]float32{} {
		t.Fatalf("a plain label carried quad %v, want zero", plainQuad)
	}
}
