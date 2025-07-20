package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v73/github"
	"github.com/goravel/framework/facades"
)

type Github interface {
	CreatePullRequest(owner, repo string, pr *github.NewPullRequest) (*github.PullRequest, error)
	CreateRelease(owner, repo string, release *github.RepositoryRelease) (*github.RepositoryRelease, error)
	GenerateReleaseNotes(owner, repo string, opts *github.GenerateNotesOptions) (*github.RepositoryReleaseNotes, error)
	GetLatestRelease(owner, repo string) (*github.RepositoryRelease, error)
	GetPullRequest(owner, repo string, number int) (*github.PullRequest, error)
	GetPullRequests(owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error)
	GetReleases(owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, error)
}

type GithubImpl struct {
	ctx    context.Context
	client *github.Client
}

func NewGithubImpl(ctx context.Context) *GithubImpl {
	token := facades.Config().GetString("GITHUB_TOKEN")
	if token == "" {
		panic("github token is not set")
	}

	client := github.NewTokenClient(ctx, token)

	return &GithubImpl{ctx: ctx, client: client}
}

// CreatePullRequest creates a new pull request
func (r *GithubImpl) CreatePullRequest(owner, repo string, pr *github.NewPullRequest) (*github.PullRequest, error) {
	pullRequest, response, err := r.client.PullRequests.Create(r.ctx, owner, repo, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create pull request for %s/%s: %s", owner, repo, response.Status)
	}
	return pullRequest, nil
}

// CreateRelease creates a new release
func (r *GithubImpl) CreateRelease(owner, repo string, release *github.RepositoryRelease) (*github.RepositoryRelease, error) {
	createdRelease, response, err := r.client.Repositories.CreateRelease(r.ctx, owner, repo, release)
	if err != nil {
		return nil, fmt.Errorf("failed to create release for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create release for %s/%s: %s", owner, repo, response.Status)
	}
	return createdRelease, nil
}

// GenerateReleaseNotes generates release notes for a repository
func (r *GithubImpl) GenerateReleaseNotes(owner, repo string, opts *github.GenerateNotesOptions) (*github.RepositoryReleaseNotes, error) {
	notes, response, err := r.client.Repositories.GenerateReleaseNotes(r.ctx, owner, repo, opts)
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

// GetLatestRelease gets the latest release for a repository
func (r *GithubImpl) GetLatestRelease(owner, repo string) (*github.RepositoryRelease, error) {
	release, response, err := r.client.Repositories.GetLatestRelease(r.ctx, owner, repo)
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
	return release, nil
}

// GetPullRequest gets a specific pull request by number
func (r *GithubImpl) GetPullRequest(owner, repo string, number int) (*github.PullRequest, error) {
	pr, response, err := r.client.PullRequests.Get(r.ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request %d for %s/%s: %w", number, owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get pull request %d for %s/%s: %s", number, owner, repo, response.Status)
	}
	return pr, nil
}

// GetPullRequests lists pull requests for a repository
func (r *GithubImpl) GetPullRequests(owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	prs, response, err := r.client.PullRequests.List(r.ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repo, err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %s", owner, repo, response.Status)
	}
	return prs, nil
}

// GetReleases lists releases for a repository
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
