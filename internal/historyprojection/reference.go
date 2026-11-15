package historyprojection

import "github.com/rainchan/webmux/internal/historysource"

// View separates exact native originals from the presentation used for handoff.
// Frozen identity-backed reference facts can replace a generated inline envelope
// with its separate user request, but never rewrite the original source records.
type View struct {
	Document      historysource.Document
	SkippedIDs    []string
	ReferenceKeys map[string]string
}

func PrepareDocument(doc historysource.Document) (View, error) {
	out := View{Document: doc}
	if len(doc.References) == 0 {
		return out, nil
	}
	if len(doc.RecordRefs) != len(doc.Records) {
		return out, historysource.ErrFormat
	}
	versions := make(map[string]string, len(doc.RecordRefs))
	for _, ref := range doc.RecordRefs {
		versions[ref.ID] = ref.Hash
	}
	references := make(map[string]historysource.ContextReference, len(doc.References))
	for _, ref := range doc.References {
		references[ref.RecordID] = ref
	}
	out.ReferenceKeys = make(map[string]string)
	out.Document.Records = make([]historysource.Record, 0, len(doc.Records))
	out.Document.RecordRefs = make([]historysource.RecordRef, 0, len(doc.RecordRefs))
	out.Document.Gaps = append([]historysource.Gap(nil), doc.Gaps...)
	out.Document.References = nil
	for index, record := range doc.Records {
		if ref, exists := references[record.ID]; exists {
			linked := ref.Policy != "" && ref.Key != ""
			for _, original := range ref.Originals {
				linked = linked && versions[original.ID] == original.Hash
			}
			if linked && ref.UserText == nil {
				out.SkippedIDs = append(out.SkippedIDs, record.ID)
				continue
			}
			key := ref.Key
			if linked {
				record.Text = *ref.UserText
				key += "/linked"
			} else {
				record.Kind = "structured"
				record.Text = "[Unreconciled historical projection reference; not original source text]\n" + record.Text
				key += "/unreconciled"
				out.Document.Gaps = append(out.Document.Gaps, historysource.Gap{RecordID: record.ID, Category: "unreconciled_projection_reference", Offset: record.Offset})
			}
			out.ReferenceKeys[record.ID] = key
		}
		out.Document.Records = append(out.Document.Records, record)
		out.Document.RecordRefs = append(out.Document.RecordRefs, doc.RecordRefs[index])
	}
	return out, nil
}
