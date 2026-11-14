package pluginwire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func Project(p workspace.Project) contract.Project {
	return contract.Project{ID: p.ID, Name: p.Name, Path: p.Path, RepositoryID: p.RepositoryID, Availability: p.Availability, Error: p.Error}
}
func Projects(projects []workspace.Project) []contract.Project { return each(projects, Project) }

func ProjectClone(op workspace.ProjectCloneOperation) contract.ProjectCloneOperation {
	return contract.ProjectCloneOperation{ID: op.ID, State: op.State, Path: op.Path, Error: op.Error, ProjectID: op.ProjectID}
}
func ProjectCloneRequest(req contract.ProjectCloneRequest) workspace.ProjectCloneRequest {
	return workspace.ProjectCloneRequest{RequestID: req.RequestID, URL: req.URL, Path: req.Path}
}
