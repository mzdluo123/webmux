package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/rainchan/webmux/internal/historysource"
)

func (s *Service) contextReferences(ctx context.Context, doc historysource.Document) ([]historysource.ContextReference, error) {
	refs, err := s.store.NativeContextReferences(ctx, doc.Source)
	if err != nil || len(refs) == 0 {
		return nil, err
	}
	byID := make(map[string][]historysource.ContextReference, len(refs))
	for _, ref := range refs {
		byID[ref.NativeID] = append(byID[ref.NativeID], ref)
	}
	out := make([]historysource.ContextReference, 0)
	for _, record := range doc.Records {
		candidates := byID[record.NativeID]
		if record.NativeID == "" || len(candidates) == 0 {
			continue
		}
		sum := sha256.Sum256([]byte(record.Text))
		digest := hex.EncodeToString(sum[:])
		role := record.Role
		if role == "agent" {
			role = "assistant"
		}
		for _, ref := range candidates {
			if ref.Role == role && ref.TextHash == digest {
				ref.RecordID = record.ID
				key, err := hashValue(ref)
				if err != nil {
					return nil, err
				}
				ref.Key = key
				out = append(out, ref)
				break
			}
		}
	}
	return out, nil
}
