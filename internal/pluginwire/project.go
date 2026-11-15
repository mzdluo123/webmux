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
	return workspace.ProjectCloneRequest{RequestID: req.RequestID, Source: req.Source, URL: req.URL, Path: req.Path}
}

func GitHubProjects(p workspace.GitHubProjects) contract.GitHubProjects {
	return contract.GitHubProjects{Available: p.Available, Reason: p.Reason, Login: p.Login, Page: p.Page, HasMore: p.HasMore, Repositories: each(p.Repositories, func(r workspace.GitHubRepository) contract.GitHubRepository {
		return contract.GitHubRepository{FullName: r.FullName, Description: r.Description, Private: r.Private, CloneURL: r.CloneURL}
	})}
}
