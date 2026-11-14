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

// ProjectCloneRequest identifies one durable, repeatable remote clone.
type ProjectCloneRequest struct {
	RequestID string `json:"requestId"`
	Source    string `json:"source,omitempty"`
	URL       string `json:"url"`
	Path      string `json:"path"`
}

// ProjectCloneOperation is safe to poll after a browser reconnects. Errors do
// not include Git output or credentials from the service environment.
type ProjectCloneOperation struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Path      string `json:"path"`
	Error     string `json:"error"`
	ProjectID string `json:"projectId"`
}

// GitHubProjects describes the service user's explicit GitHub source query.
type GitHubProjects struct {
	Available    bool               `json:"available"`
	Reason       string             `json:"reason"`
	Login        string             `json:"login"`
	Repositories []GitHubRepository `json:"repositories"`
	Page         int                `json:"page"`
	HasMore      bool               `json:"hasMore"`
}
type GitHubRepository struct {
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	CloneURL    string `json:"cloneUrl"`
}
