package components

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// markdownCanvas measures every rune as half its size wide, and every cell of
// fixed-pitch text as 6 wide at size 12, so widths are counts of characters.
func markdownCanvas() *ui.Canvas {
	canvas := &ui.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face ui.MonoFace) (float32, float32) { return size / 2, size * 1.25 })
	return canvas
}

func layoutMarkdown(t *testing.T, src string, width float32, style MarkdownStyle) (*MarkdownView, *markdown.Document) {
	t.Helper()
	var doc markdown.Document
	doc.Parse(src)
	view := &MarkdownView{}
	view.Layout(markdownCanvas(), style, &doc, width)
	return view, &doc
}

// words returns the non-blank characters of the pieces, in order.
func (v *MarkdownView) words() string {
	var out strings.Builder
	for _, p := range v.pieces {
		out.WriteString(strings.Join(strings.Fields(p.text), ""))
	}
	return out.String()
}

func TestMarkdownSelectionRangeClampsAndOrders(t *testing.T) {
	tests := []struct {
		name    string
		sel     MarkdownSelection
		textLen int
		start   int
		end     int
	}{
		{name: "empty", sel: MarkdownSelection{Anchor: 4, Focus: 4}, textLen: 10, start: 4, end: 4},
		{name: "forward", sel: MarkdownSelection{Anchor: 2, Focus: 7}, textLen: 10, start: 2, end: 7},
		{name: "reversed", sel: MarkdownSelection{Anchor: 7, Focus: 2}, textLen: 10, start: 2, end: 7},
		{name: "negative", sel: MarkdownSelection{Anchor: -3, Focus: 4}, textLen: 10, start: 0, end: 4},
		{name: "past the end", sel: MarkdownSelection{Anchor: 4, Focus: 99}, textLen: 10, start: 4, end: 10},
		{name: "outside both ways", sel: MarkdownSelection{Anchor: -8, Focus: 99}, textLen: 10, start: 0, end: 10},
		{name: "empty text", sel: MarkdownSelection{Anchor: -1, Focus: 3}, textLen: 0, start: 0, end: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end := test.sel.Range(test.textLen)
			if start != test.start || end != test.end {
				t.Fatalf("Range(%d) = %d..%d, want %d..%d", test.textLen, start, end, test.start, test.end)
			}
		})
	}
}

func TestEmptyMarkdownViewHasNoTextToSelect(t *testing.T) {
	var view MarkdownView
	if got := view.TextLen(); got != 0 {
		t.Fatalf("TextLen = %d, want 0", got)
	}
	if got := view.SelectedText(MarkdownSelection{Anchor: -2, Focus: 5}); got != "" {
		t.Fatalf("SelectedText = %q, want empty", got)
	}
}

func TestMarkdownSelectionUsesRenderedTextWithoutSoftWraps(t *testing.T) {
	view, _ := layoutMarkdown(t, "alpha beta gamma delta epsilon", 70, MarkdownStyle{Size: 12})
	if len(view.pieces) < 2 {
		t.Fatalf("the paragraph did not wrap: %+v", view.pieces)
	}
	const want = "alpha beta gamma delta epsilon"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	if got := view.SelectedText(MarkdownSelection{Anchor: 11, Focus: 16}); got != "gamma" {
		t.Fatalf("selected %q, want gamma", got)
	}
}

func TestMarkdownSelectionHasOneBreakBetweenBlocks(t *testing.T) {
	view, _ := layoutMarkdown(t, "first paragraph\n\nsecond paragraph", 200, MarkdownStyle{Size: 12})
	const want = "first paragraph\nsecond paragraph"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestMarkdownSelectionUsesVisibleInlineTextAndSpacing(t *testing.T) {
	src := "before **bold** and `code` and a [link](https://example.com/x) after"
	view, _ := layoutMarkdown(t, src, 200, MarkdownStyle{Size: 12})
	const want = "before bold and code and a link after"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestMarkdownSelectionReadsTableCellsInRenderedOrder(t *testing.T) {
	src := "| Name | Value |\n|:--|--:|\n| ssh | shell |\n| ftp | old |\n"
	view, _ := layoutMarkdown(t, src, 300, MarkdownStyle{Size: 12})
	const want = "Name Value\nssh shell\nftp old"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestInlinePiecesMapToOneRangeOfTheRenderedText(t *testing.T) {
	view, _ := layoutMarkdown(t, "before **bold** and `code` and a [link](https://example.com/x) after", 90, MarkdownStyle{Size: 12})
	var last int32 = -1
	for _, piece := range view.pieces {
		if piece.plainLo < 0 {
			continue
		}
		if piece.plainLo < last || piece.plainHi < piece.plainLo || int(piece.plainHi) > view.TextLen() {
			t.Fatalf("piece %q has range %d..%d after %d", piece.text, piece.plainLo, piece.plainHi, last)
		}
		if got := string(view.plain[piece.plainLo:piece.plainHi]); got != piece.text {
			t.Errorf("piece %q maps to %q", piece.text, got)
		}
		last = piece.plainHi
	}
}

func TestFencedCodeCopiesSourceLinesWithoutVisualWraps(t *testing.T) {
	const line = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	view, _ := layoutMarkdown(t, "```\n"+line+"\nsecond line\n```\n", 200, MarkdownStyle{Size: 12})
	var mono int
	for _, piece := range view.pieces {
		if piece.mono {
			mono++
		}
	}
	if mono < 3 {
		t.Fatalf("the long code line did not wrap: %d pieces", mono)
	}
	want := line + "\nsecond line"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestFencedCodeKeepsTabsAndCopiesThem(t *testing.T) {
	view, _ := layoutMarkdown(t, "```\n\tfoo\tbar\n```\n", 300, MarkdownStyle{Size: 12})
	const want = "\tfoo\tbar"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	tests := []struct {
		start, end int
		want       string
	}{
		{start: 0, end: 1, want: "\t"},
		{start: 1, end: 4, want: "foo"},
		{start: 0, end: 4, want: "\tfoo"},
		{start: 4, end: 5, want: "\t"},
		{start: 5, end: 8, want: "bar"},
	}
	for _, test := range tests {
		if got := view.SelectedText(MarkdownSelection{Anchor: test.start, Focus: test.end}); got != test.want {
			t.Errorf("select %d..%d = %q, want %q", test.start, test.end, got, test.want)
		}
	}
}

func TestCodeChunkBoundariesDoNotSplitATab(t *testing.T) {
	// Five cells put the tab across the six-cell boundary: the first chunk has
	// to end before it, and the next one owns the whole tab.
	view, _ := layoutMarkdown(t, "```\nabcde\tX\n```\n", 56, MarkdownStyle{Size: 12})
	const want = "abcde\tX"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	last := int32(0)
	for _, piece := range view.pieces {
		if !piece.mono {
			continue
		}
		if piece.plainLo != last || piece.plainHi <= piece.plainLo {
			t.Fatalf("piece %q has range %d..%d, want it to start at %d", piece.text, piece.plainLo, piece.plainHi, last)
		}
		last = piece.plainHi
	}
	if int(last) != view.TextLen() {
		t.Fatalf("code pieces cover %d bytes of %d", last, view.TextLen())
	}
}

func TestWrappedCodePiecesCoverTheSourceLineInOrder(t *testing.T) {
	const line = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	view, _ := layoutMarkdown(t, "```\n"+line+"\n```\n", 200, MarkdownStyle{Size: 12})
	last := int32(0)
	for _, piece := range view.pieces {
		if !piece.mono {
			continue
		}
		if piece.plainLo != last || piece.plainHi <= piece.plainLo {
			t.Fatalf("piece %q has range %d..%d, want it to start at %d", piece.text, piece.plainLo, piece.plainHi, last)
		}
		last = piece.plainHi
	}
	if int(last) != view.TextLen() {
		t.Fatalf("code pieces cover %d bytes of %d", last, view.TextLen())
	}
}

func paintedMarkdown(t *testing.T, src string, width float32) (*MarkdownView, *ui.Canvas) {
	t.Helper()
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse(src)
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, width)
	view.Paint(canvas, MarkdownPaint{X: 10, Y: 20})
	return &view, canvas
}

func TestMarkdownOffsetAtFollowsThePaintedText(t *testing.T) {
	view, _ := paintedMarkdown(t, "abc def", 300)
	tests := []struct {
		point  ui.Point
		offset int
	}{
		{point: ui.Point{X: 10, Y: 20}, offset: 0},
		{point: ui.Point{X: 52, Y: 20}, offset: 7},
		{point: ui.Point{X: 0, Y: 20}, offset: 0},
		{point: ui.Point{X: 300, Y: 20}, offset: 7},
		{point: ui.Point{X: 10, Y: -100}, offset: 0},
		{point: ui.Point{X: 60, Y: 200}, offset: 7},
	}
	for _, test := range tests {
		offset, ok := view.OffsetAt(test.point)
		if !ok || offset != test.offset {
			t.Errorf("OffsetAt(%v) = %d, %v, want %d", test.point, offset, ok, test.offset)
		}
	}
}

func TestMarkdownOffsetAtSeparatesVisualLines(t *testing.T) {
	view, _ := paintedMarkdown(t, "first\nsecond", 300)
	if len(view.pieces) < 2 {
		t.Fatalf("the line break made %d pieces", len(view.pieces))
	}
	first, second := view.pieces[0], view.pieces[1]
	endOfFirst, ok := view.OffsetAt(ui.Point{X: 10 + first.x + first.w, Y: 20 + first.y})
	if !ok || endOfFirst != 5 {
		t.Fatalf("end of the first line = %d, %v", endOfFirst, ok)
	}
	startOfSecond, ok := view.OffsetAt(ui.Point{X: 10 + second.x, Y: 20 + second.y + 2})
	if !ok || startOfSecond != 6 {
		t.Fatalf("start of the second line = %d, %v", startOfSecond, ok)
	}
}

func TestMarkdownOffsetAtStaysOnCharacterBoundaries(t *testing.T) {
	view, _ := paintedMarkdown(t, "中文", 300)
	allowed := map[int]bool{0: true, 3: true, 6: true}
	for x := float32(0); x <= 12; x++ {
		offset, ok := view.OffsetAt(ui.Point{X: 10 + x, Y: 20})
		if !ok || !allowed[offset] {
			t.Fatalf("x=%v gives offset %d, not a character boundary", x, offset)
		}
	}
	if offset, _ := view.OffsetAt(ui.Point{X: 16, Y: 20}); offset != 3 {
		t.Errorf("the middle of the text gives %d, want 3", offset)
	}
	if offset, _ := view.OffsetAt(ui.Point{X: 22, Y: 20}); offset != 6 {
		t.Errorf("the end of the text gives %d, want 6", offset)
	}
}

func TestMarkdownOffsetAtSnapsInsideAnExpandedCodeTab(t *testing.T) {
	view, _ := paintedMarkdown(t, "```\nabcde\tX\n```\n", 56)
	var tabPiece *mdPiece
	for index := range view.pieces {
		if view.pieces[index].mono && view.pieces[index].plainLo == 5 {
			tabPiece = &view.pieces[index]
		}
	}
	if tabPiece == nil {
		t.Fatalf("no piece starts at the tab: %+v", view.pieces)
	}
	originX, originY := float32(10+tabPiece.x), float32(20+tabPiece.y+2)
	left, ok := view.OffsetAt(ui.Point{X: originX + 3, Y: originY})
	if !ok || left != 5 {
		t.Fatalf("the left half of the tab gives %d, %v, want 5", left, ok)
	}
	right, ok := view.OffsetAt(ui.Point{X: originX + 15, Y: originY})
	if !ok || right != 6 {
		t.Fatalf("the right half of the tab gives %d, %v, want 6", right, ok)
	}
}

func TestMarkdownCaretBoundsReportsNoCaret(t *testing.T) {
	view, _ := paintedMarkdown(t, "text", 300)
	if bounds, ok := view.CaretBounds(); ok || bounds != (ui.Rect{}) {
		t.Fatalf("CaretBounds = %+v, %v; a painted Markdown view has no caret", bounds, ok)
	}
}

func markdownSelectionRects(canvas *ui.Canvas, color ui.Color) []ui.Rectangle {
	var found []ui.Rectangle
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color {
			found = append(found, rectangle)
		}
	}
	return found
}

func TestMarkdownPaintsPaintedSelection(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha beta gamma")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	selected := ui.Color{Red: 1, Alpha: 1}
	var painted ui.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected}, X: 10, Y: 20, Selection: MarkdownSelection{Anchor: 0, Focus: 5},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), painted.Rectangles())
	}
	if rects[0].Bounds.Width <= 0 || rects[0].Bounds.Height <= 0 || rects[0].Bounds.Y < 20 {
		t.Errorf("selection rectangle = %+v", rects[0].Bounds)
	}
	if len(painted.Texts()) == 0 {
		t.Fatal("the text was not painted with the selection")
	}
}

func TestMarkdownPaintsNoSelectionForAnEmptyRange(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha beta gamma")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	selected := ui.Color{Red: 1, Alpha: 1}
	var painted ui.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected}, Selection: MarkdownSelection{Anchor: 4, Focus: 4},
	})
	if rects := markdownSelectionRects(&painted, selected); len(rects) != 0 {
		t.Fatalf("empty selection painted %+v", rects)
	}
}

func TestMarkdownSelectionIsClippedToVisibleLines(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("first\nsecond")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	if len(view.pieces) < 2 {
		t.Fatalf("the document has %d pieces", len(view.pieces))
	}
	second := view.pieces[1]
	selected := ui.Color{Red: 1, Alpha: 1}
	paint := func(selection MarkdownSelection) *ui.Canvas {
		var painted ui.Canvas
		view.Paint(&painted, MarkdownPaint{
			Look: MarkdownLook{Selection: selected}, Selection: selection,
			Clip: ui.Rect{Y: second.y + 2, Width: 300, Height: second.h - 2},
		})
		return &painted
	}
	if rects := markdownSelectionRects(paint(MarkdownSelection{Anchor: 0, Focus: 5}), selected); len(rects) != 0 {
		t.Fatalf("a selection above the clip painted %+v", rects)
	}
	if rects := markdownSelectionRects(paint(MarkdownSelection{Anchor: 0, Focus: 12}), selected); len(rects) != 1 {
		t.Fatalf("the visible line painted %d selection rectangles, want 1", len(rects))
	}
}

func TestMarkdownCodeSelectionCoversATabAsCells(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("```\nabcde\tX\n```\n")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 56)
	selected := ui.Color{Red: 1, Alpha: 1}
	behind := ui.Color{Green: 1, Alpha: 1}
	var painted ui.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected, CodeFill: behind}, Selection: MarkdownSelection{Anchor: 5, Focus: 6},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	selectionAt, fillAt := -1, -1
	for index, rectangle := range painted.Rectangles() {
		if rectangle.Color == selected {
			selectionAt = index
		}
		if rectangle.Color == behind {
			fillAt = index
		}
	}
	if fillAt < 0 || selectionAt < fillAt {
		t.Errorf("selection at %d and code fill at %d: selection must be painted over the fill", selectionAt, fillAt)
	}
	cell := float32(0)
	for _, piece := range view.pieces {
		if piece.mono {
			cell = piece.cell
			break
		}
	}
	if want := 3 * cell; cell <= 0 || rects[0].Bounds.Width != want {
		t.Errorf("the tab covers %v, want %v (cell %v)", rects[0].Bounds.Width, want, cell)
	}
}

func TestMarkdownWrapsEveryParagraphIntoItsWidthWithoutLosingText(t *testing.T) {
	src := "A long paragraph with **bold words**, `inline code` and a [link to somewhere](https://example.com/x), 中文也要能换行，而且标点不能出现在行首。\n" +
		"- an item that is long enough to need a second line to be shown\n" +
		"> a quote that is long enough to wrap as well, inside its bar\n" +
		"Averyveryveryverylongwordthatdoesnotfitanywhereatall"
	const width = 200
	view, _ := layoutMarkdown(t, src, width, MarkdownStyle{Size: 12})
	want := strings.Join(strings.Fields(strings.NewReplacer("**", "", "`", "", "[link to somewhere](https://example.com/x)", "linktosomewhere", "- ", "", "> ", "").Replace(src)), "")
	if got := view.words(); strings.ReplaceAll(got, "•", "") != want {
		t.Errorf("text changed while wrapping:\n got  %q\n want %q", got, want)
	}
	for _, p := range view.pieces {
		if p.x < 0 || p.x+p.w > width+12 { // closing punctuation may hang a glyph past the edge
			t.Errorf("piece %q at %v..%v leaves the %d-wide column", p.text, p.x, p.x+p.w, width)
		}
	}
	// Pieces on one line never overlap.
	for i, a := range view.pieces {
		for _, b := range view.pieces[i+1:] {
			if a.y == b.y && a.x < b.x+b.w-0.5 && b.x < a.x+a.w-0.5 {
				t.Errorf("%q and %q overlap on one line", a.text, b.text)
			}
		}
	}
}

func TestMarkdownLinkRegionsFollowTheWrappedText(t *testing.T) {
	view, _ := layoutMarkdown(t, "see [the very long link text here](https://example.com/a) now", 150, MarkdownStyle{Size: 12})
	if len(view.links) < 2 {
		t.Fatalf("a link that wraps should have a region on each line, got %d", len(view.links))
	}
	var canvas ui.Canvas
	scope := ui.Scope(7)
	view.Paint(&canvas, MarkdownPaint{Look: MarkdownLook{}, X: 10, Y: 100, Links: scope})
	if len(canvas.Hits()) != len(view.links) {
		t.Fatalf("%d hit regions for %d link fragments", len(canvas.Hits()), len(view.links))
	}
	for _, hit := range canvas.Hits() {
		index, ok := hit.ID.In(scope)
		if !ok || view.Link(index) != "https://example.com/a" || hit.Bounds.X < 10 || hit.Bounds.Y < 100 {
			t.Errorf("hit %+v does not lead to the link", hit)
		}
	}
	// Scrolled out of view, a link has no region.
	canvas.Reset()
	view.Paint(&canvas, MarkdownPaint{X: 10, Y: 100, Links: scope, Clip: ui.Rect{Y: 1000, Width: 100, Height: 100}})
	if len(canvas.Hits()) != 0 || len(canvas.Texts()) != 0 {
		t.Errorf("painted %d texts and %d hits outside the clip", len(canvas.Texts()), len(canvas.Hits()))
	}
}

func TestMarkdownCodeBlocksAreReportedAndKeepRoomForAButton(t *testing.T) {
	src := "text\n```sh\nls -la /var/log/some/very/long/path/that/must/wrap\necho 你好\n```\n"
	view, _ := layoutMarkdown(t, src, 200, MarkdownStyle{Size: 12, CodeInset: 30})
	codes := view.Codes()
	if len(codes) != 1 || !strings.HasPrefix(codes[0].Text, "ls -la") {
		t.Fatalf("codes = %+v", codes)
	}
	box := codes[0].Bounds
	if box.Width != 200 || box.Y <= 0 {
		t.Errorf("block is at %+v", box)
	}
	var lines int
	for _, p := range view.pieces {
		if !p.mono {
			continue
		}
		lines++
		if p.x+p.w > box.X+box.Width-30 || p.y < box.Y || p.y+p.h > box.Y+box.Height {
			t.Errorf("code line %q at %v..%v is outside the text area of %+v", p.text, p.x, p.x+p.w, box)
		}
	}
	if lines < 3 {
		t.Errorf("the long line should have wrapped: %d lines", lines)
	}
}

func TestMarkdownTableFitsItsWidthAndWrapsItsCells(t *testing.T) {
	src := "| Name | Description |\n|:--|--:|\n| ssh | the secure shell protocol and the program that speaks it to remote hosts |\n| ftp | old |\n"
	view, doc := layoutMarkdown(t, src, 180, MarkdownStyle{Size: 12})
	var frame mdBox // the header band spans the table
	for _, box := range view.boxes {
		if box.kind == mdBoxHead {
			frame = box
		}
	}
	if frame.w == 0 || frame.w > 180.5 {
		t.Fatalf("table is %v wide in a 180 column", frame.w)
	}
	for _, p := range view.pieces {
		if p.x+p.w > frame.x+frame.w+0.5 {
			t.Errorf("%q sticks out of the table", p.text)
		}
	}
	if !strings.Contains(view.words(), "secureshellprotocol"[:6]) {
		t.Errorf("cell text is missing: %q", view.words())
	}
	_ = doc
	// The right-aligned column ends at its cell's right edge.
	var oldRight float32
	for _, p := range view.pieces {
		if p.text == "old" {
			oldRight = p.x + p.w
		}
	}
	if oldRight < frame.x+frame.w-markdownCellPadX-1 {
		t.Errorf("right-aligned cell ends at %v, the table's edge is %v", oldRight, frame.x+frame.w)
	}
}

func TestMarkdownQuoteBarRunsThroughConsecutiveQuotedBlocks(t *testing.T) {
	view, _ := layoutMarkdown(t, "> first\n> - second\n\nplain\n\n> again", 200, MarkdownStyle{Size: 12})
	var bars []mdBox
	for _, box := range view.boxes {
		if box.kind == mdBoxBar {
			bars = append(bars, box)
		}
	}
	if len(bars) != 2 {
		t.Fatalf("want one bar for the first quote and one for the second, got %d", len(bars))
	}
	if bars[0].y+bars[0].h > bars[1].y {
		t.Errorf("bars overlap: %+v %+v", bars[0], bars[1])
	}
}

func TestMarkdownLayoutAndPaintDoNotAllocateOnceWarm(t *testing.T) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com), 中文。\n1. one\n2. two\n> quote\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\ncode\n```\n", 8)
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	look := MarkdownLookFor(ui.DarkTheme())
	selection := MarkdownSelection{Anchor: 2, Focus: 40}
	run := func() {
		canvas.Reset()
		view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
		view.Paint(canvas, MarkdownPaint{Look: look, Links: ui.Scope(3), Selection: selection})
	}
	run()
	if allocs := testing.AllocsPerRun(10, run); allocs != 0 {
		t.Errorf("a warm layout and paint allocates %v times", allocs)
	}
}

func BenchmarkMarkdownLayout(b *testing.B) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com), 中文。\n- item one\n- item two\n> quote\n\n```go\ncode\n```\n", 20)
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	b.ReportAllocs()
	for b.Loop() {
		view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
	}
}

func TestHighlightedCodeIsDrawnInColoredRunsThatKeepTheText(t *testing.T) {
	// A toy highlighter: every "#" to the end of the line is a comment, and the
	// quoted words are strings.
	highlight := func(language, line string, dst []MarkdownCodeRun) []MarkdownCodeRun {
		if language != "sh" {
			return dst
		}
		if at := strings.Index(line, "#"); at >= 0 {
			dst = append(dst, MarkdownCodeRun{Start: at, End: len(line), Class: CodeComment})
		}
		return dst
	}
	src := "```sh\necho 你好 # 注释 and a very long comment that has to wrap somewhere\n```\n```text\nplain\n```\n"
	var doc markdown.Document
	doc.Parse(src)
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12, Highlight: highlight}, &doc, 220)
	var canvas ui.Canvas
	look := MarkdownLookFor(ui.DarkTheme())
	look.Code[CodeComment] = ui.Color{Red: 1, Alpha: 1}
	view.Paint(&canvas, MarkdownPaint{Look: look})

	var joined strings.Builder
	coloured := 0
	for _, text := range canvas.Texts() {
		runs := canvas.TextRuns(text)
		if len(runs) == 0 {
			continue
		}
		joined.WriteString(text.Value)
		bytes := 0
		for _, run := range runs {
			bytes += int(run.Bytes)
			if run.Color == look.Code[CodeComment] {
				coloured++
			}
		}
		if bytes != len(text.Value) {
			t.Errorf("runs cover %d of %d bytes of %q", bytes, len(text.Value), text.Value)
		}
	}
	if coloured == 0 || !strings.Contains(joined.String(), "echo") {
		t.Fatalf("no colored comment runs: coloured=%d text=%q", coloured, joined.String())
	}
	// A block the highlighter ignores is drawn as it was: one plain label.
	plain := 0
	for _, text := range canvas.Texts() {
		if text.Value == "plain" && len(canvas.TextRuns(text)) == 0 {
			plain++
		}
	}
	if plain != 1 {
		t.Errorf("the unhighlighted block is not drawn as plain text")
	}
}

func TestTabsInCodeBecomeBlanksSoColumnsAreTrue(t *testing.T) {
	var doc markdown.Document
	doc.Parse("```go\n\tfmt.Println(1)\n\t\tx\tend\n```\n")
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, 300)
	var lines []string
	for _, piece := range view.pieces {
		if piece.mono {
			lines = append(lines, piece.text)
		}
	}
	want := []string{"    fmt.Println(1)", "        x   end"}
	if len(lines) != 2 || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("code lines = %q, want %q", lines, want)
	}
	if codes := view.Codes(); len(codes) != 1 || !strings.Contains(codes[0].Text, "\t") {
		t.Errorf("the copied text lost its tabs: %+v", codes)
	}
}

func TestFencedCodeBlankLinesLayOut(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{name: "blank line", src: "```\na\n\nb\n```\n", want: "a\n\nb"},
		{name: "empty block", src: "```\n```\n", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, _ := layoutMarkdown(t, test.src, 300, MarkdownStyle{Size: 12})
			if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != test.want {
				t.Fatalf("selected %q, want %q", got, test.want)
			}
		})
	}
}

func TestFencedCodeToleratesInvalidUTF8(t *testing.T) {
	view, _ := layoutMarkdown(t, "```\n\xff\n```\n", 300, MarkdownStyle{Size: 12})
	const want = "\xff"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestInlineCodeSelectionUsesItsOwnCellWidth(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha `code` beta")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	selected := ui.Color{Red: 1, Alpha: 1}
	var painted ui.Canvas
	// "alpha " is six bytes, then the four bytes of the inline code.
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected}, Selection: MarkdownSelection{Anchor: 6, Focus: 10},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	// Inline code is one point smaller than the body, so its cell is not a
	// fenced block's cell and there may be no fenced block at all.
	const cell = float32(11) / 2
	if want := 4 * cell; rects[0].Bounds.Width != want {
		t.Errorf("inline code selection is %v wide, want %v", rects[0].Bounds.Width, want)
	}
}

func TestMarkdownOffsetAtInsideInlineCode(t *testing.T) {
	view, _ := layoutMarkdown(t, "alpha `code` beta", 300, MarkdownStyle{Size: 12})
	var painted ui.Canvas
	view.Paint(&painted, MarkdownPaint{})
	index := -1
	for i := range view.pieces {
		if view.pieces[i].mono {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("no inline code piece")
	}
	piece := &view.pieces[index]
	const cell = float32(11) / 2
	offset, ok := view.OffsetAt(ui.Point{X: piece.x + cell*2.1, Y: piece.y + 1})
	if want := int(piece.plainLo) + 2; !ok || offset != want {
		t.Fatalf("offset in inline code = %d, %v; want %d", offset, ok, want)
	}
}

func TestHeadingInlineCodeSelectionUsesItsOwnCellWidth(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("# alpha `code` beta\n\n```\nx\n```\n")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	index := -1
	for i := range view.pieces {
		if piece := &view.pieces[i]; piece.mono && !piece.spread && piece.text == "code" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("no inline code piece in the heading")
	}
	piece := &view.pieces[index]
	selected := ui.Color{Red: 1, Alpha: 1}
	var painted ui.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look:      MarkdownLook{Selection: selected},
		Selection: MarkdownSelection{Anchor: int(piece.plainLo), Focus: int(piece.plainHi)},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	if want := float32(monoCells(piece.text)) * piece.cell; rects[0].Bounds.Width != want {
		t.Errorf("heading inline code selection is %v wide, want %v", rects[0].Bounds.Width, want)
	}
}
