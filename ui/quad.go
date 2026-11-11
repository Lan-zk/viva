package ui

// corners is the winding order the four quads points go round in: top-left,
// top-right, bottom-right, bottom-left. Quad.Points puts the bottom two the
// other way round so that a quad reads like a rect (see Quad).
var quadWinding = [4]int{0, 1, 3, 2}

// area is the signed area of the quad, in square logical pixels.
func (quad Quad) area() float32 {
	points := quad.Points
	var sum float32
	for index, at := range quadWinding {
		next := points[quadWinding[(index+1)%len(quadWinding)]]
		point := points[at]
		sum += point.X*next.Y - next.X*point.Y
	}
	return sum / 2
}

// paintable reports whether the quad covers a visible area at all.
func (quad Quad) paintable() bool {
	if area := quad.area(); area > -0.5 && area < 0.5 {
		return false
	}
	for _, tint := range quad.Tint {
		if tint.Alpha > 0 {
			return true
		}
	}
	return false
}

// FillQuad fills a general quadrilateral: four corners in the order top-left,
// top-right, bottom-left, bottom-right, each with the colour that corner takes.
// The colours vary linearly across the shape, so one quad can paint a flat
// panel, a tilted sheet or a graded band. Unlike Fill it need not be
// axis-aligned; unlike a rectangle it is tessellated into triangles, so its
// outer edges are feathered like a path's instead of landing on pixel edges.
//
// The quad is clipped by Canvas.Clip and drawn on the current layer. A quad with
// no area or with every corner transparent paints nothing.
func (canvas *Canvas) FillQuad(quad Quad) {
	if !quad.paintable() {
		return
	}
	points, tints := quad.Points, quad.Tint
	clip, clipped := canvas.clipSnapshot()
	layer := canvas.currentLayer()
	// Rectangles and triangles are separate lists, so a triangle records how
	// many rectangles came before it to be interleaved back into paint order.
	before := uint32(len(canvas.rectangles))
	for _, half := range [2][3]int{{0, 1, 2}, {1, 3, 2}} {
		triangle := Triangle{
			Points:      [3]Point{points[half[0]], points[half[1]], points[half[2]]},
			Layer:       layer,
			ClipOf:      clip,
			HasClip:     clipped,
			RectsBefore: before,
		}
		colors := [3]Color{tints[half[0]], tints[half[1]], tints[half[2]]}
		triangle.Color = colors[0]
		triangle.Shade = [2]Color{colors[1], colors[2]}
		triangle.Shaded = colors[0] != colors[1] || colors[0] != colors[2]
		// Feather only the edges on the quad's outline; the diagonal the two
		// halves share stays fully covered so the seam does not show.
		if half[0] == 0 {
			triangle.EdgeMask = 1<<0 | 1<<2
		} else {
			triangle.EdgeMask = 1<<0 | 1<<1
		}
		if triangle.visible() {
			canvas.triangles = append(canvas.triangles, triangle)
		}
	}
}

// AppendTriangle appends a raw Triangle primitive on the current layer and clip,
// setting RectsBefore to the current rectangle count so paint order is preserved.
func (canvas *Canvas) AppendTriangle(triangle Triangle) {
	triangle.Layer = canvas.currentLayer()
	triangle.ClipOf, triangle.HasClip = canvas.clipSnapshot()
	triangle.RectsBefore = uint32(len(canvas.rectangles))
	if triangle.visible() {
		canvas.triangles = append(canvas.triangles, triangle)
	}
}

// LabelQuad appends a system-font line drawn into quad instead of its
// axis-aligned bounds, with the corner tints multiplied into the pixels at each
// corner. The platform rasterises the line at its Bounds size, so give Bounds
// the rectangle the line occupies untransformed, and let the quad place that
// picture. A quad is what tilts, foreshortens or shades a line drawn on a
// surface that is not facing the reader square on, such as a page being turned.
//
// One label is one straight line of text: a quad bends a line only as far as
// its four corners describe, so give a strongly curved surface one quad per
// line rather than a single quad for the page.
func (canvas *Canvas) LabelQuad(value string, bounds Rect, quad Quad, style TextStyle) {
	canvas.LabelQuadAligned(value, bounds, quad, style, AlignStart)
}

// LabelQuadAligned appends a system-font line drawn into quad with explicit
// horizontal placement inside bounds, like LabelAligned inside a quad.
func (canvas *Canvas) LabelQuadAligned(value string, bounds Rect, quad Quad, style TextStyle, alignment Alignment) {
	if !quad.paintable() {
		return
	}
	originalX := bounds.X
	originalW := bounds.Width
	value, bounds.X = canvas.placeLabel(value, bounds, style, alignment)
	if inset := bounds.X - originalX; inset > 0 && originalW > 0 {
		frac := min(inset/originalW, 1)
		quad.Points[0].X += (quad.Points[1].X - quad.Points[0].X) * frac
		quad.Points[0].Y += (quad.Points[1].Y - quad.Points[0].Y) * frac
		quad.Points[2].X += (quad.Points[3].X - quad.Points[2].X) * frac
		quad.Points[2].Y += (quad.Points[3].Y - quad.Points[2].Y) * frac
	}

	text := Text{
		Value:   value,
		Bounds:  bounds,
		Style:   style,
		Layer:   canvas.currentLayer(),
		Quad:    quad.Points,
		Tint:    quad.Tint,
		HasQuad: true,
	}
	text.ClipOf, text.HasClip = canvas.clipSnapshot()
	canvas.texts = append(canvas.texts, text)
}
