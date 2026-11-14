package contract

// Project is a session's durable directory context owned by workspace.
type Project struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	RepositoryID string `json:"repositoryId"`
	Availability string `json:"availability"`
	Error        string `json:"error"`
}
type OpenProjectRequest struct {
	Path string `json:"path"`
}
type CreateProjectRequest struct {
	Parent        string `json:"parent"`
	Name          string `json:"name"`
	InitializeGit bool   `json:"initializeGit"`
}

// ProjectCreationFailure identifies a created directory retained after failure.
type ProjectCreationFailure struct {
	Error string `json:"error"`
	Path  string `json:"path"`
}
