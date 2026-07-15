package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v89/github"
	"github.com/goravel/framework/support/color"
	"github.com/goravel/framework/support/convert"

	"goravel/app/facades"
)

type Github interface {
	// CheckBranchExists checks if a branch exists in a repository
	CheckBranchExists(owner, repo, branch string) (bool, error)
	// CreateBranch creates a new branch from master in a repository
	CreateBranch(owner, repo, branch string) error
	// CreatePullRequest creates a new pull request
	CreatePullRequest(owner, repo string, pr *github.NewPullRequest) (*github.PullRequest, error)
	// CreateRelease creates a new release
	CreateRelease(owner, repo string, release *github.CreateReleaseRequest) (*github.RepositoryRelease, error)
	// GenerateReleaseNotes generates release notes for a repository
	GenerateReleaseNotes(owner, repo string, opts *github.GenerateNotesRequest) (*github.RepositoryReleaseNotes, error)
	// GetLatestRelease gets the latest release for a repository.
	// If tag is provided, it will return the latest release with the same major and minor version as the tag.
	// For example, if tag is v1.16.2, it will return the latest release with tag starting with v1.16.
	// If no such release is found, it will return the latest release.
	GetLatestRelease(owner, repo, tag string) (*github.RepositoryRelease, error)
	// GetPullRequest gets a specific pull request by number
	GetPullRequest(owner, repo string, number int) (*github.PullRequest, error)
	// GetPullRequests lists pull requests for a repository
	GetPullRequests(owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error)
	// GetCombinedStatus gets the combined CI status for a git ref
	GetCombinedStatus(owner, repo, ref string) (*github.CombinedStatus, error)
	// GetCheckRunsForRef gets check runs for a git ref
	GetCheckRunsForRef(owner, repo, ref string) ([]*github.CheckRun, error)
	// GetReleases lists releases for a repository
	GetReleases(owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, error)
	// SetDefaultBranch sets the default branch for a repository
	SetDefaultBranch(owner, repo, branch string) error
	// CreateWorkflowDispatchEvent triggers a workflow_dispatch event for a workflow file on a ref
	CreateWorkflowDispatchEvent(owner, repo, workflowFileName, ref string) error
}

type GithubImpl struct {
	ctx    context.Context
	client *github.Client
	real   bool
}

func NewGithubImpl(real bool) *GithubImpl {
	token := facades.Config().GetString("GITHUB_TOKEN")
	if token == "" {
		panic("github token is not set")
	}

	client, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		panic(fmt.Sprintf("failed to create github client: %s", err))
	}

	return &GithubImpl{ctx: context.Background(), client: client, real: real}
}

func (r *GithubImpl) CheckBranchExists(owner, repo, branch string) (bool, error) {
	_, response, err := r.client.Repositories.GetBranch(r.ctx, owner, repo, branch, 0)
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return false, nil
		}

		return false, fmt.Errorf("failed to check branch %s for %s/%s: %w", branch, owner, repo, err)
	}
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check branch %s for %s/%s: %s", branch, owner, repo, response.Status)
	}

	return true, nil
}

func (r *GithubImpl) CreateBranch(owner, repo, branch string) error {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip creating branch %s for %s/%s", branch, owner, repo))
		return nil
	}

	masterRef, response, err := r.client.Git.GetRef(r.ctx, owner, repo, "heads/master")
	if err != nil {
		return fmt.Errorf("failed to get master ref for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get master ref for %s/%s: %s", owner, repo, response.Status)
	}

	ref := github.CreateRef{
		Ref: "refs/heads/" + branch,
		SHA: masterRef.Object.GetSHA(),
	}
	_, response, err = r.client.Git.CreateRef(r.ctx, owner, repo, ref)
	if err != nil {
		return fmt.Errorf("failed to create branch %s for %s/%s: %w", branch, owner, repo, err)
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create branch %s for %s/%s: %s", branch, owner, repo, response.Status)
	}

	return nil
}

func (r *GithubImpl) CreatePullRequest(owner, repo string, pr *github.NewPullRequest) (*github.PullRequest, error) {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip creating pull request for %s/%s", owner, repo))
		return &github.PullRequest{
			Title:   pr.Title,
			HTMLURL: convert.Pointer(fmt.Sprintf("https://github.com/%s/%s/pull/fake", owner, repo)),
		}, nil
	}

	pullRequest, response, err := r.client.PullRequests.Create(r.ctx, owner, repo, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create pull request for %s/%s: %s", owner, repo, response.Status)
	}
	return pullRequest, nil
}

func (r *GithubImpl) CreateRelease(owner, repo string, release *github.CreateReleaseRequest) (*github.RepositoryRelease, error) {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip creating release for %s/%s", owner, repo))
		return nil, nil
	}

	createdRelease, response, err := r.client.Repositories.CreateRelease(r.ctx, owner, repo, *release)
	if err != nil {
		return nil, fmt.Errorf("failed to create release for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create release for %s/%s: %s", owner, repo, response.Status)
	}
	return createdRelease, nil
}

func (r *GithubImpl) GenerateReleaseNotes(owner, repo string, opts *github.GenerateNotesRequest) (*github.RepositoryReleaseNotes, error) {
	notes, response, err := r.client.Repositories.GenerateReleaseNotes(r.ctx, owner, repo, *opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate release notes for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to generate release notes for %s/%s: %s", owner, repo, response.Status)
	}
	if notes == nil {
		return nil, fmt.Errorf("failed to generate release notes for %s/%s, notes is nil", owner, repo)
	}
	return notes, nil
}

func (r *GithubImpl) GetLatestRelease(owner, repo, tag string) (*github.RepositoryRelease, error) {
	releases, response, err := r.client.Repositories.ListReleases(r.ctx, owner, repo, &github.ListOptions{Page: 1, PerPage: 50})
	if err != nil {
		var apiErr *github.ErrorResponse
		if errors.As(err, &apiErr) {
			if apiErr.Response.StatusCode == http.StatusNotFound {
				return nil, nil
			}
		}

		return nil, fmt.Errorf("failed to get latest release for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get latest release for %s/%s: %s", owner, repo, response.Status)
	}

	if len(releases) == 0 {
		return nil, nil
	}

	// v1.16.2 -> v1.16.
	tagPrefix := strings.Join(strings.Split(tag, ".")[:2], ".") + "."
	for _, release := range releases {
		if strings.HasPrefix(release.GetTagName(), tagPrefix) {
			return release, nil
		}
	}

	return releases[0], nil
}

func (r *GithubImpl) GetPullRequest(owner, repo string, number int) (*github.PullRequest, error) {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip getting pull request %d for %s/%s", number, owner, repo))
		return &github.PullRequest{
			Merged: convert.Pointer(true),
		}, nil
	}

	pr, response, err := r.client.PullRequests.Get(r.ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request %d for %s/%s: %w", number, owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get pull request %d for %s/%s: %s", number, owner, repo, response.Status)
	}
	return pr, nil
}

func (r *GithubImpl) GetPullRequests(owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip listing pull requests for %s/%s", owner, repo))
		return nil, nil
	}

	prs, response, err := r.client.PullRequests.List(r.ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %s", owner, repo, response.Status)
	}
	return prs, nil
}

func (r *GithubImpl) GetCombinedStatus(owner, repo, ref string) (*github.CombinedStatus, error) {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip getting combined status for %s/%s@%s", owner, repo, ref))
		return &github.CombinedStatus{
			State: convert.Pointer("success"),
		}, nil
	}

	status, response, err := r.client.Repositories.GetCombinedStatus(r.ctx, owner, repo, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get combined status for %s/%s@%s: %w", owner, repo, ref, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get combined status for %s/%s@%s: %s", owner, repo, ref, response.Status)
	}
	return status, nil
}

func (r *GithubImpl) GetCheckRunsForRef(owner, repo, ref string) ([]*github.CheckRun, error) {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip getting check runs for %s/%s@%s", owner, repo, ref))
		return []*github.CheckRun{
			{
				Status:     convert.Pointer("completed"),
				Conclusion: convert.Pointer("success"),
			},
		}, nil
	}

	opts := &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allCheckRuns []*github.CheckRun
	for {
		results, response, err := r.client.Checks.ListCheckRunsForRef(r.ctx, owner, repo, ref, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list check runs for %s/%s@%s: %w", owner, repo, ref, err)
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to list check runs for %s/%s@%s: %s", owner, repo, ref, response.Status)
		}
		allCheckRuns = append(allCheckRuns, results.CheckRuns...)
		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	return allCheckRuns, nil
}

func (r *GithubImpl) GetReleases(owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, error) {
	releases, response, err := r.client.Repositories.ListReleases(r.ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list releases for %s/%s: %s", owner, repo, response.Status)
	}
	return releases, nil
}

func (r *GithubImpl) SetDefaultBranch(owner, repo, branch string) error {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip setting default branch for %s/%s to %s", owner, repo, branch))
		return nil
	}

	_, response, err := r.client.Repositories.Edit(r.ctx, owner, repo, &github.Repository{
		DefaultBranch: convert.Pointer(branch),
	})
	if err != nil {
		return fmt.Errorf("failed to set default branch for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set default branch for %s/%s: %s", owner, repo, response.Status)
	}
	return nil
}

func (r *GithubImpl) CreateWorkflowDispatchEvent(owner, repo, workflowFileName, ref string) error {
	if !r.real {
		color.Yellow().Println(fmt.Sprintf(
			"Preview mode, skip triggering workflow %s for %s/%s@%s",
			workflowFileName, owner, repo, ref,
		))
		return nil
	}

	_, _, err := r.client.Actions.CreateWorkflowDispatchEventByFileName(
		r.ctx, owner, repo, workflowFileName,
		github.CreateWorkflowDispatchEventRequest{Ref: ref},
	)
	if err != nil {
		return fmt.Errorf("failed to trigger workflow %s for %s/%s@%s: %w",
			workflowFileName, owner, repo, ref, err)
	}

	return nil
}
