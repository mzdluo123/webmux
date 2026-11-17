package historysource

// Exportable excludes runtime/control records from historical instructions.
// Readers already exclude reasoning, secrets and private provider state.
func (r Record) Exportable() bool { return contentKind(r.Kind) }

func (r RecordRef) Exportable() bool { return contentKind(r.Kind) }

func contentKind(kind string) bool {
	switch kind {
	case "message", "prose", "code", "patch", "structured", "command", "tool", "tool_arguments", "tool_result", "tool-call", "tool-result", "error", "interrupt", "file_change", "plan", "compaction":
		return true
	default:
		return false
	}
}
