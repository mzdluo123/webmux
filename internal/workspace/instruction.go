package workspace

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// artifactsDirPlaceholder stands for the session's interaction artifacts
// folder in an expanded instruction; it is replaced once the session exists.
const artifactsDirPlaceholder = "{{" + artifactsDirName + "}}"

// webmuxPlaceholder stands for the session's WebMux brief (the shell guide
// address, executable, socket and artifacts folder); it is replaced once
// the session exists.
const webmuxPlaceholder = "{{" + webmuxReferenceName + "}}"

// userReference is the built-in {{user}}: where the user's own message goes
// in an expanded instruction. {{dir}} is the artifacts folder and
// {{webmux}} the brief above. A quick command cannot be named any of them.
const (
	artifactsDirName    = "dir"
	userReferenceName   = "user"
	userReference       = "{{" + userReferenceName + "}}"
	webmuxReferenceName = "webmux"
)

// QuickCommand is a saved quick command: an initialization instruction
// inserts its Text where it writes {{Name}}.
type QuickCommand struct {
	Name string
	Text string
}

// instructionReference matches {{name}}; the name may not contain braces or
// line breaks.
var instructionReference = regexp.MustCompile(`\{\{([^{}\n]+)\}\}`)

// expandInstruction replaces each {{name}} of template with the text of the
// quick command called name. Replacement is one pass: the inserted text is
// not scanned again. {{user}}, {{dir}} and {{webmux}} stay in the text for the
// caller to fill. The names no command has are returned in order of first
// appearance and their references are left as written.
func expandInstruction(template string, commands []QuickCommand) (string, []string) {
	byName := make(map[string]string, len(commands))
	for _, c := range commands {
		byName[c.Name] = c.Text
	}
	var missing []string
	seen := map[string]bool{}
	text := instructionReference.ReplaceAllStringFunc(template, func(ref string) string {
		name := strings.TrimSpace(ref[2 : len(ref)-2])
		if name == userReferenceName {
			return userReference
		}
		if name == artifactsDirName {
			return artifactsDirPlaceholder
		}
		if name == webmuxReferenceName {
			return webmuxPlaceholder
		}
		if replacement, ok := byName[name]; ok {
			return replacement
		}
		if !seen[name] {
			seen[name] = true
			missing = append(missing, name)
		}
		return ref
	})
	return text, missing
}

// validInstruction normalizes an initialization instruction: blank is none,
// more than MaxInitInstruction characters is ErrInvalid.
func validInstruction(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	if n := utf8.RuneCountInString(text); n > MaxInitInstruction {
		return "", problemf(ErrInvalid, "初始化指令最多 %d 个字符，当前 %d 个", MaxInitInstruction, n)
	}
	return text, nil
}

// SetInitInstruction replaces repository id's initialization instruction;
// blank clears it. The registry is saved first: a failed save leaves the
// instruction as it was.
func (s *Service) SetInitInstruction(ctx context.Context, id, text string) (Repository, error) {
	instruction, err := validInstruction(text)
	if err != nil {
		return Repository{}, err
	}
	r, err := s.repository(id)
	if err != nil {
		return Repository{}, err
	}
	s.saveMu.Lock()
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil {
		s.mu.Unlock()
		s.saveMu.Unlock()
		return Repository{}, err
	}
	entries := append([]*repositoryState(nil), s.entries...)
	s.mu.Unlock()
	found := false
	for _, item := range entries {
		found = found || item == r
	}
	if !found {
		s.saveMu.Unlock()
		return Repository{}, problemf(ErrNotFound, "仓库不存在")
	}
	err = s.saveWith(entries, r, instruction)
	if err == nil {
		s.mu.Lock()
		r.instruction = instruction
		s.mu.Unlock()
	}
	s.saveMu.Unlock()
	if err != nil {
		return Repository{}, fmt.Errorf("保存仓库清单: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	snapshot, err := s.snapshot(ctx, r)
	if err != nil {
		// The instruction is saved; a failed read still answers the entry.
		snapshot = s.unavailableRepository(r, err)
	}
	snapshot.InitInstruction = instruction
	return snapshot, nil
}
