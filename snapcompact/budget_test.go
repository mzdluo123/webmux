package snapcompact

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func known(n int64) Quantity { return Quantity{Value: n, Known: true, Basis: "test estimate"} }
func testLimits() BudgetLimits {
	return BudgetLimits{known(10000), known(10000), known(10), known(100000), known(20000)}
}
func testCapabilities() Capabilities {
	return Capabilities{VisionKnown: true, VisionSupported: true,
		TextTokens:  func(_ context.Context, s string) (Quantity, error) { return known(int64(len(s))), nil },
		ImageTokens: func(_ context.Context, w, h int) (Quantity, error) { return known(int64(w * h)), nil },
		RequestBytes: func(_ context.Context, d Delivery) (Quantity, error) {
			n := int64(len(d.Text) + len(d.CurrentTask) + 128)
			for _, p := range d.Pages {
				n += int64((len(p.PNG)+2)/3*4 + 64)
			}
			return known(n), nil
		}}
}
func testPlanInput() PlanInput {
	return PlanInput{Representation: Projected, Projection: Projection{CurrentTask: "  task\r\n", Segments: []Segment{{SourceID: "s", Role: "tool", Text: "历史"}}}, Rendered: RenderResult{Pages: []Page{{Width: 10, Height: 10, PNG: []byte("fake-png"), Ranges: []Range{{SourceID: "s", Field: "projection", End: 2}}}}}}
}
func TestPlanSeparateBudgets(t *testing.T) {
	tests := []struct {
		name   string
		change func(*BudgetLimits)
		reason string
	}{
		{"text", func(l *BudgetLimits) { l.TextTokens = known(0) }, "text tokens"},
		{"image", func(l *BudgetLimits) { l.ImageTokens = known(0) }, "image tokens"},
		{"count", func(l *BudgetLimits) { l.Images = known(0) }, "image count"},
		{"bytes", func(l *BudgetLimits) { l.RequestBytes = known(0) }, "request bytes"},
		{"context", func(l *BudgetLimits) { l.ContextTokens = known(0) }, "context tokens"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testPlanInput()
			if tt.name != "text" {
				in.Representation = Image
			}
			limits := testLimits()
			tt.change(&limits)
			got, err := Plan(context.Background(), in, testCapabilities(), limits)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "insufficient" || len(got.Uncovered) != 1 || !strings.Contains(strings.Join(got.Reasons, ";"), tt.reason) {
				t.Fatalf("unexpected plan: %+v", got)
			}
			if got.Candidate.CurrentTask != in.Projection.CurrentTask {
				t.Fatal("task mutated")
			}
		})
	}
}
func TestPlanUnknownAndIncomplete(t *testing.T) {
	in := testPlanInput()
	in.Representation = Image
	tests := []struct {
		name   string
		modify func(*PlanInput, *Capabilities, *BudgetLimits)
		want   string
	}{
		{"unknown vision", func(_ *PlanInput, c *Capabilities, _ *BudgetLimits) { c.VisionKnown = false }, "unknown"},
		{"unsupported", func(_ *PlanInput, c *Capabilities, _ *BudgetLimits) { c.VisionSupported = false }, "unavailable"},
		{"unknown image rate", func(_ *PlanInput, c *Capabilities, _ *BudgetLimits) { c.ImageTokens = nil }, "unknown"},
		{"unknown encoder", func(_ *PlanInput, c *Capabilities, _ *BudgetLimits) { c.RequestBytes = nil }, "unknown"},
		{"unknown context", func(_ *PlanInput, _ *Capabilities, l *BudgetLimits) { l.ContextTokens.Known = false }, "unknown"},
		{"missing glyph", func(i *PlanInput, _ *Capabilities, _ *BudgetLimits) {
			i.Rendered.Missing = []MissingGlyph{{Rune: '😀'}}
		}, "unavailable"},
		{"partial", func(i *PlanInput, _ *Capabilities, _ *BudgetLimits) {
			i.Rendered.Uncovered = []Range{{SourceID: "s", End: 2}}
		}, "unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			give := in
			caps := testCapabilities()
			limits := testLimits()
			tt.modify(&give, &caps, &limits)
			got, err := Plan(context.Background(), give, caps, limits)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.want || len(got.Uncovered) != 1 {
				t.Fatalf("unexpected plan %+v", got)
			}
		})
	}
}
func TestPlanRepresentations(t *testing.T) {
	for _, kind := range []Representation{Original, Projected, Image, Reference} {
		in := testPlanInput()
		in.Representation = kind
		in.OriginalText = "original"
		in.OriginalRanges = []Range{{SourceID: "s", Field: "text", End: 8}}
		in.ReferenceText = "source://s"
		in.ReferenceRanges = in.OriginalRanges
		got, err := Plan(context.Background(), in, testCapabilities(), testLimits())
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "ready" || got.Candidate.Representation != kind || len(got.Uncovered) != 0 {
			t.Fatalf("unexpected %+v", got)
		}
		if kind == Image && got.Usage.ImageTokens.Value != 100 {
			t.Fatal("image cost must use dimensions")
		}
	}
}
func TestPlanCancellationAndErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Plan(ctx, testPlanInput(), testCapabilities(), testLimits()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	caps := testCapabilities()
	caps.TextTokens = func(context.Context, string) (Quantity, error) { return known(-1), nil }
	if _, err := Plan(context.Background(), testPlanInput(), caps, testLimits()); err == nil {
		t.Fatal("negative accepted")
	}
	caps.TextTokens = func(context.Context, string) (Quantity, error) { return known(math.MaxInt64), nil }
	in := testPlanInput()
	in.Representation = Image
	if _, err := Plan(context.Background(), in, caps, testLimits()); err == nil {
		t.Fatal("overflow accepted")
	}
	caps = testCapabilities()
	sentinel := errors.New("estimator unavailable")
	caps.RequestBytes = func(context.Context, Delivery) (Quantity, error) { return Quantity{}, sentinel }
	if _, err := Plan(context.Background(), testPlanInput(), caps, testLimits()); !errors.Is(err, sentinel) {
		t.Fatalf("cause lost: %v", err)
	}
}

func TestPlanRejectsGapsAndOverlaps(t *testing.T) {
	tests := []struct {
		name   string
		ranges []Range
	}{
		{"gap", []Range{{SourceID: "s", Field: "projection", Start: 1, End: 2}}},
		{"overlap", []Range{{SourceID: "s", Field: "projection", End: 2}, {SourceID: "s", Field: "projection", End: 1}}},
		{"wrong field", []Range{{SourceID: "s", Field: "text", End: 2}}},
		{"unknown source", []Range{{SourceID: "unknown", Field: "projection", End: 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testPlanInput()
			in.Representation = Image
			in.Rendered.Pages[0].Ranges = tt.ranges
			got, err := Plan(context.Background(), in, testCapabilities(), testLimits())
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "unavailable" || len(got.Uncovered) != 1 {
				t.Fatalf("accepted bad coverage: %+v", got)
			}
		})
	}
}

func TestPlanImageRateIndependentOfPNGSize(t *testing.T) {
	in := testPlanInput()
	in.Representation = Image
	first, err := Plan(context.Background(), in, testCapabilities(), testLimits())
	if err != nil {
		t.Fatal(err)
	}
	in.Rendered.Pages[0].PNG = make([]byte, 300)
	second, err := Plan(context.Background(), in, testCapabilities(), testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if first.Usage.ImageTokens.Value != second.Usage.ImageTokens.Value {
		t.Fatal("PNG bytes affected token rate")
	}
	if first.Usage.RequestBytes.Value >= second.Usage.RequestBytes.Value {
		t.Fatal("PNG bytes did not affect transfer estimate")
	}
}
