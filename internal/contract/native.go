package contract

type NativeRequest struct {
	OutputRunes   int    `json:"outputRunes"`
	Action        string `json:"action"`
	Path          string `json:"path"`
	Root          string `json:"root"`
	SourceID      string `json:"sourceId"`
	SnapshotID    string `json:"snapshotId"`
	CurrentTask   string `json:"currentTask"`
	AfterPosition string `json:"afterPosition"`
}
type NativeFile struct {
	Path      string `json:"path"`
	NativeID  string `json:"nativeId"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	Problem   string `json:"problem"`
}
type NativeResult struct {
	Provenance     []NativeProvenance     `json:"provenance"`
	Files          []NativeFile           `json:"files"`
	SourceID       string                 `json:"sourceId"`
	ConversationID string                 `json:"conversationId"`
	SnapshotID     string                 `json:"snapshotId"`
	Parser         string                 `json:"parser"`
	Consumed       string                 `json:"consumed"`
	Pending        bool                   `json:"pending"`
	Gaps           []string               `json:"gaps"`
	Activities     ConversationActivities `json:"activities"`
}

type NativeRange struct {
	SourceID string `json:"sourceId"`
	Field    string `json:"field"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}
type NativeSegment struct {
	SourceID string        `json:"sourceId"`
	Text     string        `json:"text"`
	Role     string        `json:"role"`
	CallID   string        `json:"callId"`
	ToolName string        `json:"toolName"`
	Status   string        `json:"status"`
	Sources  []NativeRange `json:"sources"`
}
type NativeOmission struct {
	Range         NativeRange `json:"range"`
	Reason        string      `json:"reason"`
	OriginalRunes int         `json:"originalRunes"`
}
type NativePNG struct {
	PNG    []byte        `json:"png"`
	Ranges []NativeRange `json:"ranges"`
}
type NativePreview struct {
	ID                string           `json:"id"`
	Version           string           `json:"version"`
	CurrentTask       string           `json:"currentTask"`
	Segments          []NativeSegment  `json:"segments"`
	Omissions         []NativeOmission `json:"omissions"`
	Pages             []NativePNG      `json:"pages"`
	Uncovered         []NativeRange    `json:"uncovered"`
	MissingGlyphCount int              `json:"missingGlyphCount"`
	Problem           string           `json:"problem"`
}

type NativeProvenance struct {
	ActivityID string `json:"activityId"`
	SourceID   string `json:"sourceId"`
	Checksum   string `json:"checksum"`
	CallID     string `json:"callId"`
	ToolName   string `json:"toolName"`
	Kind       string `json:"kind"`
	Offset     string `json:"offset"`
	End        string `json:"end"`
}
