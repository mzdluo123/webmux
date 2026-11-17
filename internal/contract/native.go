package contract

type NativeRequest struct {
	SessionID     string `json:"sessionId"`
	Agent         string `json:"agent"`
	NativeID      string `json:"nativeId"`
	Branch        string `json:"branch"`
	TextOffset    string `json:"textOffset"`
	OutputRunes   int    `json:"outputRunes"`
	Action        string `json:"action"`
	Path          string `json:"path"`
	Root          string `json:"root"`
	SnapshotID    string `json:"snapshotId"`
	CurrentTask   string `json:"currentTask"`
	AfterPosition string `json:"afterPosition"`
}
type NativeFile struct {
	Agent    string `json:"agent"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	NativeID string `json:"nativeId"`
}
type NativeResult struct {
	ExpiresAt      string         `json:"expiresAt"`
	Agent          string         `json:"agent"`
	NativeID       string         `json:"nativeId"`
	Branch         string         `json:"branch"`
	Format         string         `json:"format"`
	Boundary       string         `json:"boundary"`
	ContentHash    string         `json:"contentHash"`
	NextTextOffset string         `json:"nextTextOffset"`
	NextPosition   string         `json:"nextPosition"`
	More           bool           `json:"more"`
	RecordCount    int            `json:"recordCount"`
	Truncated      bool           `json:"truncated"`
	Files          []NativeFile   `json:"files"`
	SnapshotID     string         `json:"snapshotId"`
	Parser         string         `json:"parser"`
	Pending        bool           `json:"pending"`
	Gaps           []string       `json:"gaps"`
	Records        []NativeRecord `json:"records"`
}

type NativeRecord struct {
	ID             string             `json:"id"`
	NativeID       string             `json:"nativeId"`
	ParentID       string             `json:"parentId"`
	Hash           string             `json:"hash"`
	Kind           string             `json:"kind"`
	Role           string             `json:"role"`
	Status         string             `json:"status"`
	Text           string             `json:"text"`
	CallID         string             `json:"callId"`
	NativeCallID   string             `json:"nativeCallId"`
	ToolName       string             `json:"toolName"`
	Position       string             `json:"position"`
	Offset         string             `json:"offset"`
	End            string             `json:"end"`
	ByteLength     string             `json:"byteLength"`
	NextTextOffset string             `json:"nextTextOffset"`
	Truncated      bool               `json:"truncated"`
	Attachments    []NativeAttachment `json:"attachments"`
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
	Gaps              []string         `json:"gaps"`
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

type NativeAttachment struct {
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Reference string `json:"reference"`
	Size      int64  `json:"size"`
}
