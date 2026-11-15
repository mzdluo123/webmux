package snapcompact

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// Representation distinguishes source content from its lossy projection and references.
type Representation string

const (
	Original  Representation = "original"
	Projected Representation = "projection"
	Image     Representation = "image"
	Reference Representation = "reference"
)

// Quantity is unknown unless Known is true. Basis describes the caller's estimator,
// measured capability, or encoder; estimates are never presented as measurements.
type Quantity struct {
	Value int64
	Known bool
	Basis string
}

// BudgetLimits keeps transport, modality, and context constraints independent.
// Unknown constraints cause an unknown plan; zero is an actual zero budget.
type BudgetLimits struct{ TextTokens, ImageTokens, Images, RequestBytes, ContextTokens Quantity }
type BudgetUsage struct{ TextTokens, ImageTokens, Images, RequestBytes, ContextTokens Quantity }

// Delivery is a candidate only. The planner never sends requests or selects history.
// Text excludes CurrentTask; callers must encode both fields exactly once.
type Delivery struct {
	Representation    Representation
	CurrentTask, Text string
	Pages             []Page
	Ranges            []Range
}

// Capabilities supplies target-specific estimates and the actual request encoder's
// byte count, including base64, wrappers, task, and any other request fields.
// Nil callbacks explicitly mean unknown. No universal image rate is assumed.
type Capabilities struct {
	VisionKnown, VisionSupported bool
	TextTokens                   func(context.Context, string) (Quantity, error)
	ImageTokens                  func(context.Context, int, int) (Quantity, error)
	RequestBytes                 func(context.Context, Delivery) (Quantity, error)
}
type PlanInput struct {
	Representation Representation
	Projection     Projection
	Rendered       RenderResult
	// OriginalText and OriginalRanges must come from the frozen source, not projection.
	OriginalText    string
	OriginalRanges  []Range
	ReferenceText   string
	ReferenceRanges []Range
}
type PlanResult struct {
	Status    string // ready, insufficient, unknown, or unavailable
	Candidate Delivery
	Usage     BudgetUsage
	Uncovered []Range
	Reasons   []string
}

// Plan evaluates complete coverage. It never silently chooses a prefix, substitutes
// references for content, or claims that an estimate proves model readability.
func Plan(ctx context.Context, in PlanInput, caps Capabilities, limits BudgetLimits) (PlanResult, error) {
	result := PlanResult{Status: "ready", Reasons: []string{}, Uncovered: []Range{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	d := Delivery{Representation: in.Representation, CurrentTask: in.Projection.CurrentTask, Ranges: []Range{}}
	for _, s := range in.Projection.Segments {
		d.Ranges = append(d.Ranges, Range{SourceID: s.SourceID, Field: "projection", End: utf8.RuneCountInString(s.Text)})
	}
	switch in.Representation {
	case Original:
		d.Text = in.OriginalText
		d.Ranges = append([]Range{}, in.OriginalRanges...)
	case Projected:
		var b strings.Builder
		for _, s := range in.Projection.Segments {
			// Quoted labels keep source boundaries visible to the target.
			fmt.Fprintf(&b, "[%q role=%q tool=%q call=%q status=%q]\n%s\n", s.SourceID, s.Role, s.ToolName, s.CallID, s.Status, s.Text)
		}
		d.Text = b.String()
	case Image:
		d.Pages = append([]Page{}, in.Rendered.Pages...)
		if !caps.VisionKnown {
			result.Status = "unknown"
			result.Reasons = append(result.Reasons, "vision capability unknown")
		} else if !caps.VisionSupported {
			result.Status = "unavailable"
			result.Reasons = append(result.Reasons, "vision unsupported")
		}
		if len(in.Rendered.Missing) > 0 || len(in.Rendered.Uncovered) > 0 || !completePageCoverage(in.Projection, d.Pages) {
			result.Status = "unavailable"
			result.Reasons = append(result.Reasons, "rendering incomplete or missing glyphs")
		}
		if len(d.Ranges) > 0 && len(d.Pages) == 0 {
			result.Status = "unavailable"
			result.Reasons = append(result.Reasons, "no rendered pages")
		}
	case Reference:
		d.Text = in.ReferenceText
		d.Ranges = append([]Range{}, in.ReferenceRanges...)
	default:
		return result, fmt.Errorf("plan: unknown representation %q", in.Representation)
	}

	if (in.Representation == Original || in.Representation == Reference) && d.Text != "" && len(d.Ranges) == 0 {
		return result, fmt.Errorf("plan: original or reference text requires source ranges")
	}
	for _, r := range d.Ranges {
		if r.SourceID == "" || r.Field == "" || r.Start < 0 || r.End < r.Start {
			return result, fmt.Errorf("plan: invalid source range")
		}
	}
	result.Candidate = d
	u := BudgetUsage{Images: Quantity{Value: int64(len(d.Pages)), Known: true, Basis: "page count"}, ImageTokens: Quantity{Known: true, Basis: "no images"}}
	var err error
	if caps.TextTokens != nil {
		u.TextTokens, err = caps.TextTokens(ctx, d.CurrentTask+"\n"+d.Text)
		if err != nil {
			return result, fmt.Errorf("plan text tokens: %w", err)
		}
	}
	if len(d.Pages) > 0 {
		u.ImageTokens = Quantity{Known: true, Basis: "caller image estimator"}
		if caps.ImageTokens == nil {
			u.ImageTokens.Known = false
		}
		for _, page := range d.Pages {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if caps.ImageTokens == nil {
				break
			}
			q, e := caps.ImageTokens(ctx, page.Width, page.Height)
			if e != nil {
				return result, fmt.Errorf("plan image tokens: %w", e)
			}
			if e = validateQuantity(q); e != nil {
				return result, e
			}
			if !q.Known {
				u.ImageTokens.Known = false
				continue
			}
			if q.Value > math.MaxInt64-u.ImageTokens.Value {
				return result, fmt.Errorf("plan: image token overflow")
			}
			u.ImageTokens.Value += q.Value
		}
	}
	if caps.RequestBytes != nil {
		u.RequestBytes, err = caps.RequestBytes(ctx, d)
		if err != nil {
			return result, fmt.Errorf("plan request bytes: %w", err)
		}
	}
	if err := validateQuantity(u.TextTokens); err != nil {
		return result, err
	}
	if err := validateQuantity(u.RequestBytes); err != nil {
		return result, err
	}
	if u.TextTokens.Known && u.ImageTokens.Known {
		if u.ImageTokens.Value > math.MaxInt64-u.TextTokens.Value {
			return result, fmt.Errorf("plan: context token overflow")
		}
		u.ContextTokens = Quantity{Value: u.TextTokens.Value + u.ImageTokens.Value, Known: true, Basis: "text plus image estimate"}
	}
	result.Usage = u
	checks := []struct {
		name        string
		used, limit Quantity
	}{
		{"text tokens", u.TextTokens, limits.TextTokens}, {"image tokens", u.ImageTokens, limits.ImageTokens},
		{"image count", u.Images, limits.Images}, {"request bytes", u.RequestBytes, limits.RequestBytes}, {"context tokens", u.ContextTokens, limits.ContextTokens},
	}
	for _, check := range checks {
		if err := validateQuantity(check.limit); err != nil {
			return result, err
		}
		if !check.used.Known || !check.limit.Known {
			if result.Status == "ready" {
				result.Status = "unknown"
			}
			result.Reasons = append(result.Reasons, check.name+" unknown")
		} else if check.used.Value > check.limit.Value {
			if result.Status != "unavailable" {
				result.Status = "insufficient"
			}
			result.Reasons = append(result.Reasons, check.name+" exceeds budget")
		}
	}
	if result.Status != "ready" {
		result.Uncovered = append(result.Uncovered, d.Ranges...)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}
func validateQuantity(q Quantity) error {
	if q.Known && q.Value < 0 {
		return fmt.Errorf("plan: negative quantity")
	}
	return nil
}

// completePageCoverage rejects gaps, overlaps, invalid offsets, and unknown sources.
// Coverage describes the projection; original omissions remain independently visible.
func completePageCoverage(p Projection, pages []Page) bool {
	expected := make(map[string]int, len(p.Segments))
	ranges := make(map[string][]Range, len(p.Segments))
	for _, s := range p.Segments {
		if _, duplicate := expected[s.SourceID]; duplicate {
			return false
		}
		expected[s.SourceID] = utf8.RuneCountInString(s.Text)
	}
	for _, page := range pages {
		for _, r := range page.Ranges {
			size, ok := expected[r.SourceID]
			if !ok || r.Field != "projection" || r.Start < 0 || r.End < r.Start || r.End > size {
				return false
			}
			if r.End > r.Start {
				ranges[r.SourceID] = append(ranges[r.SourceID], r)
			}
		}
	}
	for id, size := range expected {
		source := ranges[id]
		sort.Slice(source, func(i, j int) bool { return source[i].Start < source[j].Start })
		position := 0
		for _, r := range source {
			if r.Start != position {
				return false
			}
			position = r.End
		}
		if position != size {
			return false
		}
	}
	return true
}
