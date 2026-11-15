package historyimport

import (
	"context"

	"github.com/rainchan/webmux/internal/historysource"
)

func (s *Service) Bind(ctx context.Context, thread string, source historysource.Source) error {
	doc, err := s.read(ctx, historysource.Request{Source: source})
	if err != nil {
		return err
	}
	return s.store.BindNativeSource(ctx, thread, doc.Source)
}

func (s *Service) resolve(ctx context.Context, binding historysource.Binding) (historysource.Source, error) {
	source := binding.Source
	if source.Path != "" {
		return source, nil
	}
	root := s.roots[source.Agent]
	if root == "" {
		return source, historysource.ErrUnavailable
	}
	found, err := s.Discover(ctx, source.Agent, root)
	if err != nil {
		return source, err
	}
	matches := 0
	for _, candidate := range found {
		if candidate.Agent == source.Agent && candidate.NativeID == source.NativeID {
			source = candidate
			matches++
		}
	}
	if matches == 0 {
		return source, historysource.ErrUnavailable
	}
	if matches != 1 {
		return source, historysource.ErrIdentity
	}
	return source, nil
}

// FreezeConversation fixes inherited prefixes and each exact native identity.
// Fork callers request retain=true before creating any child or target process.
func (s *Service) FreezeConversation(ctx context.Context, conversation, owner string, retain bool) ([]historysource.Snapshot, error) {
	if !s.enabled() || owner == "" {
		return nil, historysource.ErrUnavailable
	}
	prefixes, err := s.store.PrefixSnapshots(ctx, conversation)
	if err != nil {
		return nil, err
	}
	out := make([]historysource.Snapshot, 0, len(prefixes))
	for _, id := range prefixes {
		snapshot, err := s.Snapshot(ctx, id)
		if err != nil {
			return nil, err
		}
		if retain && !snapshot.Retained {
			doc, err := s.Document(ctx, id)
			if err != nil {
				return nil, err
			}
			snapshot, err = s.freezeDocument(ctx, doc, owner, true)
			if err != nil {
				return nil, err
			}
		} else if err := s.Retain(ctx, snapshot.ID, owner); err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	bindings, err := s.store.NativeBindings(ctx, conversation)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, binding := range bindings {
		if binding.Source.NativeID == "" {
			continue // Dormant targets have not produced a native session.
		}
		key := binding.Source.Agent + ":" + binding.Source.NativeID
		if seen[key] {
			continue
		}
		seen[key] = true
		source, err := s.resolve(ctx, binding)
		if err != nil {
			return nil, err
		}
		snapshot, err := s.Freeze(ctx, source, owner, retain)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	if len(out) == 0 {
		return nil, historysource.ErrUnavailable
	}
	return out, nil
}

// Documents composes fixed source segments, never the live state of a parent.
// Only identical source IDs/versions deduplicate; equal text is not evidence.
func (s *Service) Documents(ctx context.Context, ids []string) (historysource.Document, error) {
	out := historysource.Document{Source: historysource.Source{Agent: "composition"}, ParserVersion: "composition/v1", FormatVersion: "composition/v1"}
	seen := make(map[string]string)
	total := 0
	for _, id := range ids {
		doc, err := s.Document(ctx, id)
		if err != nil {
			return out, err
		}
		out.Gaps = append(out.Gaps, doc.Gaps...)
		for _, record := range doc.Records {
			hash, err := hashValue(record)
			if err != nil {
				return out, err
			}
			key := doc.Source.Agent + ":" + doc.Source.NativeID + ":" + record.ID
			if previous, ok := seen[key]; ok {
				if previous != hash {
					return out, historysource.ErrChanged
				}
				continue
			}
			seen[key] = hash
			total += len(record.Text)
			if total > historysource.MaxBytes || len(out.Records) >= historysource.MaxRecords {
				return out, historysource.ErrLimit
			}
			out.Records = append(out.Records, record)
		}
	}
	identity, err := hashValue(ids)
	out.Fingerprint, out.Cursor = identity, identity
	return out, err
}
