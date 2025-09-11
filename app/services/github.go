package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v73/github"
	"github.com/goravel/framework/facades"
)

type Github interface {
	CheckBranchExists(owner, repo, branch string) (bool, error)
	CreatePullRequest(owner, repo string, pr *github.NewPullRequest) (*github.PullRequest, error)
	CreateRelease(owner, repo string, release *github.RepositoryRelease) (*github.RepositoryRelease, error)
	GenerateReleaseNotes(owner, repo string, opts *github.GenerateNotesOptions) (*github.RepositoryReleaseNotes, error)
	GetLatestRelease(owner, repo, tag string) (*github.RepositoryRelease, error)
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

// CheckBranchExists checks if a branch exists in a repository
func (r *GithubImpl) CheckBranchExists(owner, repo, branch string) (bool, error) {
	_, response, err := r.client.Repositories.GetBranch(r.ctx, owner, repo, branch, 0)
	if err != nil {
		var apiErr *github.ErrorResponse
		if errors.As(err, &apiErr) {
			if apiErr.Response.StatusCode == http.StatusNotFound {
				return false, nil
			}
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
