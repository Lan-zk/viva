package reloadwire

import (
	"time"

	"github.com/vivaui/viva/ui"
)

func CaptureCanvas(canvas *ui.Canvas) CanvasSnapshot {
	var delayMs uint32
	if delay, ok := canvas.NextFrame(); ok && delay > 0 {
		delayMs = uint32(max((delay+time.Millisecond-1)/time.Millisecond, 1)) // never rounded down to "now"
	}
	return CanvasSnapshot{
		Rectangles:   append([]ui.Rectangle(nil), canvas.Rectangles()...),
		Triangles:    append([]ui.Triangle(nil), canvas.Triangles()...),
		Shadows:      append([]ui.Shadow(nil), canvas.Shadows()...),
		Images:       append([]ui.Image(nil), canvas.Images()...),
		Texts:        append([]ui.Text(nil), canvas.Texts()...),
		Runs:         append([]ui.TextRun(nil), canvas.AllTextRuns()...),
		Hits:         append([]ui.Hit(nil), canvas.Hits()...),
		TrappedScope: canvas.TrappedFocusScope(),
		EscapeTarget: canvas.EscapeTarget(),
		InitialFocus: canvas.InitialFocus(),
		Animating:    canvas.Animating(),
		FrameDelayMs: delayMs,
	}
}

func ApplyCanvas(canvas *ui.Canvas, snapshot CanvasSnapshot) {
	canvas.Reset()
	// Interleave triangles and shadows back among the rectangles in the exact
	// paint order recorded by RectsBefore and TrianglesBefore.
	triangles := snapshot.Triangles
	trianglesDone := 0
	shadows := snapshot.Shadows
	replayInterleaved := func(rectangles int) {
		for {
			if len(shadows) > 0 &&
				int(shadows[0].RectsBefore) <= rectangles &&
				int(shadows[0].TrianglesBefore) <= trianglesDone {
				shadow := shadows[0]
				shadows = shadows[1:]
				pushLayerAndClip(canvas, shadow.Layer, shadow.ClipOf, shadow.HasClip)
				canvas.Shadow(shadow.Bounds, shadow.Radius, shadow.Style, shadow.Color)
				popLayerAndClip(canvas, shadow.Layer, shadow.HasClip)
				continue
			}
			if len(triangles) > 0 && int(triangles[0].RectsBefore) <= rectangles {
				triangle := triangles[0]
				triangles = triangles[1:]
				trianglesDone++
				pushLayerAndClip(canvas, triangle.Layer, triangle.ClipOf, triangle.HasClip)
				canvas.AppendTriangle(triangle)
				popLayerAndClip(canvas, triangle.Layer, triangle.HasClip)
				continue
			}
			break
		}
	}
	for index, rectangle := range snapshot.Rectangles {
		replayInterleaved(index)
		pushLayerAndClip(canvas, rectangle.Layer, rectangle.ClipOf, rectangle.HasClip)
		canvas.Rounded(rectangle.Bounds, rectangle.Radius, rectangle.Color)
		popLayerAndClip(canvas, rectangle.Layer, rectangle.HasClip)
	}
	replayInterleaved(len(snapshot.Rectangles))
	for _, image := range snapshot.Images {
		pushLayerAndClip(canvas, image.Layer, image.ClipOf, image.HasClip)
		canvas.DrawBitmapRegion(image.Bitmap, image.Source, image.Bounds)
		popLayerAndClip(canvas, image.Layer, image.HasClip)
	}
	for _, text := range snapshot.Texts {
		pushLayerAndClip(canvas, text.Layer, text.ClipOf, text.HasClip)
		if text.RunCount > 0 || text.CellWidth > 0 {
			canvas.LabelRuns(text.Value, text.Bounds, text.Style, text.CellWidth, snapshot.Runs[text.RunFirst:text.RunFirst+text.RunCount])
		} else if text.HasQuad {
			canvas.LabelQuad(text.Value, text.Bounds, ui.Quad{Points: text.Quad, Tint: text.Tint}, text.Style)
		} else {
			canvas.Label(text.Value, text.Bounds, text.Style)
		}
		popLayerAndClip(canvas, text.Layer, text.HasClip)
	}
	for _, hit := range snapshot.Hits {
		if hit.Layer != 0 {
			canvas.PushLayer(hit.Layer)
		}
		if !hit.Scope.IsZero() {
			canvas.PushFocusScope(hit.Scope)
		}
		if hit.Block {
			canvas.Block(hit.Bounds)
		} else if hit.MovesWindow {
			canvas.MoveRegion(hit.Bounds)
		} else {
			canvas.HitWithOptions(hit.ID, hit.Bounds, hit.Label, ui.HitOptions{
				Role: hit.Role, Value: hit.Value, Hint: hit.Hint, Disabled: hit.Disabled,
				Checked: hit.Checked, Selected: hit.Selected, Expanded: hit.Expanded,
				Multiline: hit.Multiline, ReadOnly: hit.ReadOnly, Secure: hit.Secure, SkipFocus: !hit.Focusable, Autofocus: hit.Autofocus,
				TextBounds: hit.TextBounds, TextSize: hit.TextSize, TextColor: hit.TextColor, TextWeight: hit.TextWeight,
			})
		}
		if !hit.Scope.IsZero() {
			canvas.PopFocusScope()
		}
		if hit.Layer != 0 {
			canvas.PopLayer()
		}
	}
	canvas.TrapFocus(snapshot.TrappedScope)
	canvas.SetEscapeTarget(snapshot.EscapeTarget)
	canvas.SetInitialFocus(snapshot.InitialFocus)
	if snapshot.Animating {
		canvas.RequestFrame()
	} else if snapshot.FrameDelayMs > 0 {
		canvas.RequestFrameIn(time.Duration(snapshot.FrameDelayMs) * time.Millisecond)
	}
}

func pushLayerAndClip(canvas *ui.Canvas, layer uint16, clip ui.Rect, hasClip bool) {
	if layer != 0 {
		canvas.PushLayer(layer)
	}
	if hasClip {
		canvas.Clip(clip)
	}
}

func popLayerAndClip(canvas *ui.Canvas, layer uint16, hasClip bool) {
	if hasClip {
		canvas.Unclip()
	}
	if layer != 0 {
		canvas.PopLayer()
	}
}
