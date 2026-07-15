package commands

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-github/v88/github"
	"github.com/goravel/framework/contracts/console"
	mocksconsole "github.com/goravel/framework/mocks/console"
	mocksclient "github.com/goravel/framework/mocks/http/client"
	mocksprocess "github.com/goravel/framework/mocks/process"
	"github.com/goravel/framework/support/convert"
	testingmock "github.com/goravel/framework/testing/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	mocksservices "goravel/app/mocks/services"
)

type ReleaseTestSuite struct {
	suite.Suite
	mockContext *mocksconsole.Context
	mockGithub  *mocksservices.Github
	mockProcess *mocksprocess.Process
	mockHttp    *mocksclient.Factory
	release     *Release
}

func TestReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaseTestSuite))
}

func (s *ReleaseTestSuite) SetupTest() {
	packages = []string{
		"gin",
		"goravel-lite",
	}

	mockFactory := testingmock.Factory()
	s.mockContext = mocksconsole.NewContext(s.T())
	s.mockGithub = mocksservices.NewGithub(s.T())
	s.mockProcess = mockFactory.Process()
	s.mockHttp = mockFactory.Http()

	s.release = &Release{
		ctx:    s.mockContext,
		real:   true,
		github: s.mockGithub,
	}
}

func (s *ReleaseTestSuite) Test_checkPRMergeStatus() {
	var (
		repo = "gin"
		pr   = &github.PullRequest{
			Number:  convert.Pointer(1),
			HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
			State:   convert.Pointer("open"),
		}
	)

	tests := []struct {
		name       string
		real       bool
		pr         *github.PullRequest
		setup      func()
		wantResult bool
		wantErr    error
	}{
		{
			name:       "pr is nil",
			pr:         nil,
			setup:      func() {},
			wantResult: true,
		},
		{
			name: "happy path - not real",
			real: false,
			pr:   pr,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequest(owner, repo, *pr.Number).Return(&github.PullRequest{
					Number:  convert.Pointer(1),
					HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
					Merged:  convert.Pointer(true),
				}, nil).Once()
			},
			wantResult: true,
		},
		{
			name: "failed to get pull request",
			real: true,
			pr:   pr,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequest(owner, repo, *pr.Number).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "pr is merged",
			real: true,
			pr:   pr,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequest(owner, repo, *pr.Number).Return(&github.PullRequest{
					Number:  convert.Pointer(1),
					HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
					Merged:  convert.Pointer(true),
				}, nil).Once()
			},
			wantResult: true,
		},
		{
			name: "pr is not merged",
			real: true,
			pr:   pr,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequest(owner, repo, *pr.Number).Return(&github.PullRequest{
					Number:  convert.Pointer(1),
					HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
					Merged:  convert.Pointer(false),
				}, nil).Once()
			},
			wantResult: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.release.real = tt.real

			tt.setup()

			result, err := s.release.checkPRMergeStatus(repo, tt.pr)

			s.Equal(tt.wantResult, result)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_checkPRsMergeStatus() {
	var (
		repo1 = "gin"
		repo2 = "fiber"
		repo3 = "example"
		pr1   = &github.PullRequest{
			Number:  convert.Pointer(1),
			HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
			State:   convert.Pointer("open"),
		}
		pr2 = &github.PullRequest{
			Number:  convert.Pointer(2),
			HTMLURL: convert.Pointer("https://github.com/goravel/fiber/pull/2"),
			State:   convert.Pointer("open"),
		}
		repoToPR map[string]*github.PullRequest
	)

	beforeEach := func() {
		repoToPR = map[string]*github.PullRequest{
			repo1: pr1,
			repo2: pr2,
			repo3: nil, // No PR needed for this repo
		}
	}

	tests := []struct {
		name    string
		setup   func()
		wantErr error
	}{
		{
			name: "choice fails on first attempt",
			setup: func() {
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("", assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "user chooses not to check initially, then choice fails",
			setup: func() {
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Not Check", nil).Once()
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("", assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "all PRs merged in first iteration",
			setup: func() {
				// First choice - user chooses to check
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()

				// Check gin PR - merged
				s.mockContext.EXPECT().Spinner("Checking goravel/gin merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						Merged:  convert.Pointer(true),
					}, nil).Once()

				// Check fiber PR - merged
				s.mockContext.EXPECT().Spinner("Checking goravel/fiber merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo2, *pr2.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(2),
						HTMLURL: convert.Pointer("https://github.com/goravel/fiber/pull/2"),
						Merged:  convert.Pointer(true),
					}, nil).Once()
			},
			wantErr: nil,
		},
		{
			name: "some PRs not merged, then all merged in second iteration",
			setup: func() {
				// First choice - user chooses to check
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()

				// Check gin PR - not merged
				s.mockContext.EXPECT().Spinner("Checking goravel/gin merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						Merged:  convert.Pointer(false),
					}, nil).Once()

				// Check fiber PR - merged
				s.mockContext.EXPECT().Spinner("Checking goravel/fiber merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo2, *pr2.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(2),
						HTMLURL: convert.Pointer("https://github.com/goravel/fiber/pull/2"),
						Merged:  convert.Pointer(true),
					}, nil).Once()

				// Second choice - user chooses to check again
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()

				// Check gin PR again - now merged
				s.mockContext.EXPECT().Spinner("Checking goravel/gin merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						Merged:  convert.Pointer(true),
					}, nil).Once()
			},
			wantErr: nil,
		},

		{
			name: "user chooses not to check after seeing not merged PRs",
			setup: func() {
				// First choice - user chooses to check
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()

				// Check gin PR - not merged
				s.mockContext.EXPECT().Spinner("Checking goravel/gin merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						Merged:  convert.Pointer(false),
					}, nil).Once()

				// Check fiber PR - not merged
				s.mockContext.EXPECT().Spinner("Checking goravel/fiber merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo2, *pr2.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(2),
						HTMLURL: convert.Pointer("https://github.com/goravel/fiber/pull/2"),
						Merged:  convert.Pointer(false),
					}, nil).Once()

				// Second choice - user chooses not to check
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Not Check", nil).Once()

				// Third choice - user choice fails
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("", assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "mixed scenario with nil PRs",
			setup: func() {
				// First choice - user chooses to check
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()

				// Check gin PR - merged
				s.mockContext.EXPECT().Spinner("Checking goravel/gin merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						Merged:  convert.Pointer(true),
					}, nil).Once()

				// Check fiber PR - merged
				s.mockContext.EXPECT().Spinner("Checking goravel/fiber merge status...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo2, *pr2.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(2),
						HTMLURL: convert.Pointer("https://github.com/goravel/fiber/pull/2"),
						Merged:  convert.Pointer(true),
					}, nil).Once()
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			beforeEach()
			tt.setup()

			err := s.release.checkPRsMergeStatus(repoToPR)

			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_createRelease() {
	var (
		repo   = "goravel-lite"
		tag    = "v1.0.0"
		branch = "v1.0.x"
		notes  = &github.RepositoryReleaseNotes{
			Name: "v1.0.0",
			Body: "v1.0.0",
		}
	)

	tests := []struct {
		name    string
		real    bool
		setup   func()
		wantErr error
	}{
		{
			name: "happy path - not real",
			real: false,
			setup: func() {
				s.mockGithub.EXPECT().CheckBranchExists(owner, repo, branch).Return(true, nil).Once()
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer(branch),
					Name:            convert.Pointer(notes.Name),
					Body:            convert.Pointer(notes.Body),
				}).Return(nil, nil).Once()
			},
		},
		{
			name: "happy path - real",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().CheckBranchExists(owner, repo, branch).Return(true, nil).Once()
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer(branch),
					Name:            convert.Pointer(notes.Name),
					Body:            convert.Pointer(notes.Body),
				}).Return(nil, nil).Once()
			},
		},
		{
			name: "failed to create release",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().CheckBranchExists(owner, repo, branch).Return(true, nil).Once()
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer(branch),
					Name:            convert.Pointer(notes.Name),
					Body:            convert.Pointer(notes.Body),
				}).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.release.real = tt.real

			tt.setup()

			err := s.release.createRelease(repo, tag, notes)

			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_createUpgradePR() {
	var (
		repo          = "example"
		frameworkTag  = "v1.16.0"
		dependencies  = []string{"go get github.com/goravel/framework@v1.16.0", "go get github.com/goravel/gin@v1.4.0"}
		upgradeBranch = "auto-upgrade/v1.16.0"
		prTitle       = "chore: Upgrade framework to v1.16.0 (auto)"
	)

	tests := []struct {
		name    string
		real    bool
		setup   func()
		wantPR  *github.PullRequest
		wantErr error
	}{
		{
			name: "preview mode - returns mock PR",
			real: false,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/auto-upgrade/v1.16.0"),
				Number:  convert.Pointer(1),
				Head: &github.PullRequestBranch{
					SHA: convert.Pointer("fake-sha"),
				},
			},
			wantErr: nil,
		},
		{
			name: "real mode - early check fails",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return(nil, assert.AnError).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "real mode - existing PR found early, skip clone",
			real: true,
			setup: func() {
				existingPR := &github.PullRequest{
					Title:   convert.Pointer(prTitle),
					HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/123"),
					Number:  convert.Pointer(123),
				}
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{existingPR}, nil).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/123"),
				Number:  convert.Pointer(123),
			},
			wantErr: nil,
		},
		{
			name: "real mode - merged PR with same title exists, skip creation",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				mergedPR := &github.PullRequest{
					Title:    convert.Pointer(prTitle),
					HTMLURL:  convert.Pointer("https://github.com/goravel/example/pull/100"),
					Number:   convert.Pointer(100),
					MergedAt: &github.Timestamp{},
				}
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{mergedPR}, nil).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: nil,
		},
		{
			name: "real mode - spinner fails",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					Return(assert.AnError).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "real mode - clone and mod fails",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(true).Once()
				mockProcessResult.EXPECT().Error().Return(assert.AnError).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to clone repo and mod for example: %w", assert.AnError),
		},
		{
			name: "real mode - check status fails",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(true).Once()
				mockProcessResult.EXPECT().Error().Return(assert.AnError).Once()
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return(mockProcessResult).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to check status for example: %w", assert.AnError),
		},
		{
			name: "real mode - working tree clean, no changes needed",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("nothing to commit, working tree clean").Once()
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return(mockProcessResult).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: nil,
		},
		{
			name: "real mode - push branch fails",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("modified: go.mod").Once()
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(true).Once()
				mockProcessResult.EXPECT().Error().Return(assert.AnError).Once()
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return(mockProcessResult).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to push upgrade branch for example: %w", assert.AnError),
		},
		{
			name: "real mode - push branch output doesn't contain commit message",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("modified: go.mod").Once()
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("pushed to remote").Twice()
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return(mockProcessResult).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: errors.New("failed to push upgrade branch for example: pushed to remote"),
		},
		{
			name: "real mode - create new PR fails",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("modified: go.mod").Once()
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("chore: Upgrade framework to v1.16.0 (auto)").Once()
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return(mockProcessResult).Once()

				s.mockGithub.EXPECT().CreatePullRequest(owner, repo, &github.NewPullRequest{
					Title: convert.Pointer(prTitle),
					Head:  convert.Pointer(upgradeBranch),
					Base:  convert.Pointer("master"),
				}).Return(nil, assert.AnError).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "real mode - successful PR creation",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()

				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				mockProcessResult := mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				s.mockProcess.EXPECT().Run(`export GONOSUMDB=github.com/goravel && rm -rf example && git clone git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("modified: go.mod").Once()
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return(mockProcessResult).Once()

				mockProcessResult = mocksprocess.NewResult(s.T())
				mockProcessResult.EXPECT().Failed().Return(false).Once()
				mockProcessResult.EXPECT().Output().Return("chore: Upgrade framework to v1.16.0 (auto)").Once()
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return(mockProcessResult).Once()

				newPR := &github.PullRequest{
					Title:   convert.Pointer(prTitle),
					HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/456"),
					Number:  convert.Pointer(456),
				}
				s.mockGithub.EXPECT().CreatePullRequest(owner, repo, &github.NewPullRequest{
					Title: convert.Pointer(prTitle),
					Head:  convert.Pointer(upgradeBranch),
					Base:  convert.Pointer("master"),
				}).Return(newPR, nil).Once()

				s.mockProcess.EXPECT().Run("rm -rf example").Return(nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/456"),
				Number:  convert.Pointer(456),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.release.real = tt.real
			tt.setup()

			pr, err := s.release.createUpgradePR(repo, "master", frameworkTag, dependencies)

			s.Equal(tt.wantPR, pr)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_getBranchFromTag() {
	s.Run("happy path - branch doesn't exist", func() {
		s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", "v1.16.x").Return(false, nil).Once()
		branch := s.release.getBranchFromTag("framework", "v1.16.0")
		s.Equal("master", branch)
	})

	s.Run("happy path - branch exists", func() {
		s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", "v1.16.x").Return(true, nil).Once()
		branch := s.release.getBranchFromTag("framework", "v1.16.0")
		s.Equal("v1.16.x", branch)
	})

	s.Run("failed to check branch exists", func() {
		s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", "v1.16.x").Return(false, assert.AnError).Once()
		s.Panics(func() {
			_ = s.release.getBranchFromTag("framework", "v1.16.0")
		})
	})
}

func (s *ReleaseTestSuite) Test_getReleases() {
	tag := "v1.4.0"
	branch := "v1.4.x"
	packages = []string{
		"gin",
		"installer",
	}

	spinnerMsg := fmt.Sprintf("Fetching release information for %d packages...", len(append(packages, "framework")))

	tests := []struct {
		name    string
		setup   func()
		want    map[string]*ReleaseInformation
		wantErr error
	}{
		{
			name: "successful retrieval for all packages",
			setup: func() {
				allPackages := append(packages, "framework")

				s.mockContext.EXPECT().Spinner(spinnerMsg, mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				for _, pkg := range allPackages {
					s.mockGithub.EXPECT().GetLatestRelease(owner, pkg, tag).Return(&github.RepositoryRelease{
						TagName: convert.Pointer("v1.3.0"),
						Name:    convert.Pointer(fmt.Sprintf("Release v1.3.0 for %s", pkg)),
					}, nil).Once()

					if pkg == "framework" {
						s.mockGithub.EXPECT().CheckBranchExists(owner, pkg, branch).Return(true, nil).Once()
					} else {
						s.mockGithub.EXPECT().CheckBranchExists(owner, pkg, branch).Return(false, nil).Once()
					}

					expectedNotes := &github.RepositoryReleaseNotes{
						Name: fmt.Sprintf("Release v1.4.0 for %s", pkg),
						Body: fmt.Sprintf("## What's Changed\n* Feature A for %s\n* Bug fix B for %s\n\n**Full Changelog**: https://github.com/goravel/%s/compare/v1.3.0...v1.4.0", pkg, pkg, pkg),
					}
					if pkg == "framework" {
						s.mockGithub.EXPECT().GenerateReleaseNotes(owner, pkg, &github.GenerateNotesOptions{
							TagName:         "v1.4.0",
							PreviousTagName: convert.Pointer("v1.3.0"),
							TargetCommitish: convert.Pointer("v1.4.x"),
						}).Return(expectedNotes, nil).Once()
					} else {
						s.mockGithub.EXPECT().GenerateReleaseNotes(owner, pkg, &github.GenerateNotesOptions{
							TagName:         "v1.4.0",
							PreviousTagName: convert.Pointer("v1.3.0"),
							TargetCommitish: convert.Pointer("master"),
						}).Return(expectedNotes, nil).Once()
					}

					if pkg == "installer" {
						mockResponse := mocksclient.NewResponse(s.T())
						mockResponse.EXPECT().Body().Return(`package support

const Version string = "v1.4.0"`, nil)

						s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/installer/refs/heads/master/support/constant.go").
							Return(mockResponse, nil).Once()
					}

					if pkg == "framework" {
						mockResponse := mocksclient.NewResponse(s.T())
						mockResponse.EXPECT().Body().Return(`package support

const Version string = "v1.4.0"`, nil)

						s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", "v1.4.x").Return(true, nil).Once()
						s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/v1.4.x/support/constant.go").
							Return(mockResponse, nil).Once()
					}
				}
			},
			want: func() map[string]*ReleaseInformation {
				result := make(map[string]*ReleaseInformation)
				allPackages := append(packages, "framework")
				for _, pkg := range allPackages {
					releaseInformation := &ReleaseInformation{
						currentTag: "",
						latestTag:  "v1.3.0",
						notes: &github.RepositoryReleaseNotes{
							Name: fmt.Sprintf("Release v1.4.0 for %s", pkg),
							Body: fmt.Sprintf("## What's Changed\n* Feature A for %s\n* Bug fix B for %s\n\n**Full Changelog**: https://github.com/goravel/%s/compare/v1.3.0...v1.4.0", pkg, pkg, pkg),
						},
						repo: pkg,
						tag:  "v1.4.0",
					}

					if pkg == "installer" || pkg == "framework" {
						releaseInformation.currentTag = "v1.4.0"
					}

					result[pkg] = releaseInformation
				}
				return result
			}(),
			wantErr: nil,
		},
		{
			name: "one package fails with error",
			setup: func() {

				s.mockContext.EXPECT().Spinner(spinnerMsg, mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// gin: fails
				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin", tag).Return(nil, assert.AnError).Once()

				// installer: succeeds (runs in parallel)
				s.mockGithub.EXPECT().GetLatestRelease(owner, "installer", tag).Return(&github.RepositoryRelease{
					TagName: convert.Pointer("v1.3.0"),
				}, nil).Once()
				s.mockGithub.EXPECT().CheckBranchExists(owner, "installer", branch).Return(false, nil).Once()
				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "installer", &github.GenerateNotesOptions{
					TagName:         "v1.4.0",
					PreviousTagName: convert.Pointer("v1.3.0"),
					TargetCommitish: convert.Pointer("master"),
				}).Return(&github.RepositoryReleaseNotes{
					Name: "Release v1.4.0 for installer",
					Body: "## What's Changed",
				}, nil).Once()
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const Version string = "v1.4.0"`, nil)
				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/installer/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()

				// framework: succeeds (runs in parallel)
				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework", tag).Return(&github.RepositoryRelease{
					TagName: convert.Pointer("v1.3.0"),
				}, nil).Once()
				s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", branch).Return(true, nil).Once()
				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
					TagName:         "v1.4.0",
					PreviousTagName: convert.Pointer("v1.3.0"),
					TargetCommitish: convert.Pointer("v1.4.x"),
				}).Return(&github.RepositoryReleaseNotes{
					Name: "Release v1.4.0 for framework",
					Body: "## What's Changed",
				}, nil).Once()
				mockResponse2 := mocksclient.NewResponse(s.T())
				mockResponse2.EXPECT().Body().Return(`package support

const Version string = "v1.4.0"`, nil)
				s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", "v1.4.x").Return(true, nil).Once()
				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/v1.4.x/support/constant.go").
					Return(mockResponse2, nil).Once()
			},
			want:    nil,
			wantErr: assert.AnError,
		},
		{
			name: "spinner fails",
			setup: func() {
				s.mockContext.EXPECT().Spinner(spinnerMsg, mock.AnythingOfType("console.SpinnerOption")).
					Return(assert.AnError).Once()
			},
			want:    nil,
			wantErr: assert.AnError,
		},
		{
			name: "getLatestTag returns nil release - should succeed with empty latestTag",
			setup: func() {
				allPackages := append(packages, "framework")

				s.mockContext.EXPECT().Spinner(spinnerMsg, mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				for _, pkg := range allPackages {
					if pkg == "gin" {
						s.mockGithub.EXPECT().GetLatestRelease(owner, pkg, tag).Return(nil, nil).Once()
					} else {
						s.mockGithub.EXPECT().GetLatestRelease(owner, pkg, tag).Return(&github.RepositoryRelease{
							TagName: convert.Pointer("v1.3.0"),
						}, nil).Once()
					}

					if pkg == "framework" {
						s.mockGithub.EXPECT().CheckBranchExists(owner, pkg, branch).Return(true, nil).Once()
					} else {
						s.mockGithub.EXPECT().CheckBranchExists(owner, pkg, branch).Return(false, nil).Once()
					}

					var previousTag *string
					if pkg == "gin" {
						previousTag = convert.Pointer("")
					} else {
						previousTag = convert.Pointer("v1.3.0")
					}
					var targetBranch string
					if pkg == "framework" {
						targetBranch = "v1.4.x"
					} else {
						targetBranch = "master"
					}
					s.mockGithub.EXPECT().GenerateReleaseNotes(owner, pkg, &github.GenerateNotesOptions{
						TagName:         "v1.4.0",
						PreviousTagName: previousTag,
						TargetCommitish: convert.Pointer(targetBranch),
					}).Return(&github.RepositoryReleaseNotes{
						Name: fmt.Sprintf("Release v1.4.0 for %s", pkg),
						Body: fmt.Sprintf("## What's Changed\n* Feature A for %s", pkg),
					}, nil).Once()

					if pkg == "installer" {
						mockResponse := mocksclient.NewResponse(s.T())
						mockResponse.EXPECT().Body().Return(`package support

const Version string = "v1.4.0"`, nil)
						s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/installer/refs/heads/master/support/constant.go").
							Return(mockResponse, nil).Once()
					}

					if pkg == "framework" {
						mockResponse := mocksclient.NewResponse(s.T())
						mockResponse.EXPECT().Body().Return(`package support

const Version string = "v1.4.0"`, nil)
						s.mockGithub.EXPECT().CheckBranchExists(owner, "framework", "v1.4.x").Return(true, nil).Once()
						s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/v1.4.x/support/constant.go").
							Return(mockResponse, nil).Once()
					}
				}
			},
			want: func() map[string]*ReleaseInformation {
				result := make(map[string]*ReleaseInformation)
				result["gin"] = &ReleaseInformation{
					currentTag: "",
					latestTag:  "",
					notes: &github.RepositoryReleaseNotes{
						Name: "Release v1.4.0 for gin",
						Body: "## What's Changed\n* Feature A for gin",
					},
					repo: "gin",
					tag:  "v1.4.0",
				}
				result["installer"] = &ReleaseInformation{
					currentTag: "v1.4.0",
					latestTag:  "v1.3.0",
					notes: &github.RepositoryReleaseNotes{
						Name: "Release v1.4.0 for installer",
						Body: "## What's Changed\n* Feature A for installer",
					},
					repo: "installer",
					tag:  "v1.4.0",
				}
				result["framework"] = &ReleaseInformation{
					currentTag: "v1.4.0",
					latestTag:  "v1.3.0",
					notes: &github.RepositoryReleaseNotes{
						Name: "Release v1.4.0 for framework",
						Body: "## What's Changed\n* Feature A for framework",
					},
					repo: "framework",
					tag:  "v1.4.0",
				}
				return result
			}(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			result, err := s.release.getReleases(tag, append(packages, "framework"))

			s.Equal(tt.want, result)
			if tt.wantErr != nil {
				assert.Error(s.T(), err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *ReleaseTestSuite) Test_getCurrentTag() {
	tests := []struct {
		name        string
		setup       func()
		wantVersion string
		wantErr     error
	}{
		{
			name: "successful version extraction",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const (
	Version = "v1.16.0"
	// other constants...
)`, nil)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "v1.16.0",
			wantErr:     nil,
		},
		{
			name: "version with different spacing",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const (
	Version="v1.16.0"
	// other constants...
)`, nil)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "v1.16.0",
			wantErr:     nil,
		},
		{
			name: "version with extra whitespace",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const (
	Version   =   "v1.16.0"
	// other constants...
)`, nil)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "v1.16.0",
			wantErr:     nil,
		},
		{
			name: "version with tabs",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const (
	Version	=	"v1.16.0"
	// other constants...
)`, nil)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "v1.16.0",
			wantErr:     nil,
		},
		{
			name: "version with complex version string",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const (
	Version = "v1.16.0-beta.1+meta"
	// other constants...
)`, nil)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "v1.16.0-beta.1+meta",
			wantErr:     nil,
		},
		{
			name: "http request fails",
			setup: func() {
				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(nil, assert.AnError).Once()
			},
			wantVersion: "",
			wantErr:     assert.AnError,
		},
		{
			name: "response body reading fails",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return("", assert.AnError)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "",
			wantErr:     assert.AnError,
		},
		{
			name: "no version constant found",
			setup: func() {
				mockResponse := mocksclient.NewResponse(s.T())
				mockResponse.EXPECT().Body().Return(`package support

const (
	// No Version constant here
	OtherConstant = "value"
	AnotherConstant = 123
)`, nil)

				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
					Return(mockResponse, nil).Once()
			},
			wantVersion: "",
			wantErr:     fmt.Errorf("could not extract goravel/framework version from code"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			version, err := s.release.getCurrentTag("framework", "https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go")

			s.Equal(tt.wantVersion, version)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_generateReleaseNotes() {
	branch := "master"

	tests := []struct {
		name            string
		repo            string
		tagName         string
		previousTagName string
		setup           func()
		wantNotes       *github.RepositoryReleaseNotes
		wantErr         error
	}{
		{
			name:            "successful release notes generation",
			repo:            "framework",
			tagName:         "v1.16.0",
			previousTagName: "v1.15.0",
			setup: func() {
				expectedNotes := &github.RepositoryReleaseNotes{
					Name: "Release v1.16.0",
					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
				}
				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
					TagName:         "v1.16.0",
					PreviousTagName: convert.Pointer("v1.15.0"),
					TargetCommitish: convert.Pointer(branch),
				}).Return(expectedNotes, nil).Once()
			},
			wantNotes: &github.RepositoryReleaseNotes{
				Name: "Release v1.16.0",
				Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
			},
			wantErr: nil,
		},
		{
			name:            "github API error",
			repo:            "framework",
			tagName:         "v1.16.0",
			previousTagName: "v1.15.0",
			setup: func() {
				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
					TagName:         "v1.16.0",
					PreviousTagName: convert.Pointer("v1.15.0"),
					TargetCommitish: convert.Pointer(branch),
				}).Return(nil, assert.AnError).Once()
			},
			wantNotes: nil,
			wantErr:   assert.AnError,
		},
		{
			name:            "github API returns nil notes",
			repo:            "gin",
			tagName:         "v1.16.0",
			previousTagName: "v1.15.0",
			setup: func() {
				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "gin", &github.GenerateNotesOptions{
					TagName:         "v1.16.0",
					PreviousTagName: convert.Pointer("v1.15.0"),
					TargetCommitish: convert.Pointer(branch),
				}).Return(nil, nil).Once()
			},
			wantNotes: nil,
			wantErr:   fmt.Errorf("failed to generate release notes, notes is nil"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			notes, err := s.release.generateReleaseNotes(tt.repo, tt.tagName, tt.previousTagName, branch)

			s.Equal(tt.wantNotes, notes)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_getLatestTag() {
	tag := "v1.16.2"

	tests := []struct {
		name    string
		repo    string
		setup   func()
		wantTag string
		wantErr error
	}{
		{
			name: "successful tag retrieval",
			repo: "gin",
			setup: func() {
				mockRelease := &github.RepositoryRelease{
					TagName: convert.Pointer("v1.16.0"),
					Name:    convert.Pointer("Release v1.16.0"),
					Body:    convert.Pointer("This is a test release"),
				}
				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin", tag).Return(mockRelease, nil).Once()
			},
			wantTag: "v1.16.0",
			wantErr: nil,
		},
		{
			name: "github API error",
			repo: "gin",
			setup: func() {
				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin", tag).Return(nil, assert.AnError).Once()
			},
			wantTag: "",
			wantErr: assert.AnError,
		},
		{
			name: "github API returns nil release",
			repo: "fiber",
			setup: func() {
				s.mockGithub.EXPECT().GetLatestRelease(owner, "fiber", tag).Return(nil, nil).Once()
			},
			wantTag: "",
		},
		{
			name: "github API returns release with nil TagName",
			repo: "framework",
			setup: func() {
				mockRelease := &github.RepositoryRelease{
					TagName: nil,
					Name:    convert.Pointer("Release without tag"),
					Body:    convert.Pointer("This release has no tag"),
				}
				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework", tag).Return(mockRelease, nil).Once()
			},
			wantTag: "",
			wantErr: fmt.Errorf("latest release tag name is nil for %s/framework", owner),
		},
		{
			name: "github API returns release with empty TagName",
			repo: "example",
			setup: func() {
				mockRelease := &github.RepositoryRelease{
					TagName: convert.Pointer(""),
					Name:    convert.Pointer("Release with empty tag"),
					Body:    convert.Pointer("This release has empty tag"),
				}
				s.mockGithub.EXPECT().GetLatestRelease(owner, "example", tag).Return(mockRelease, nil).Once()
			},
			wantTag: "",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			tag, err := s.release.getLatestTag(tt.repo, tag)

			s.Equal(tt.wantTag, tag)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_isReleaseExist() {
	tests := []struct {
		name    string
		repo    string
		tag     string
		setup   func()
		want    bool
		wantErr error
	}{
		{
			name: "release exists",
			repo: "framework",
			tag:  "v1.16.0",
			setup: func() {
				releases := []*github.RepositoryRelease{
					{
						TagName: convert.Pointer("v1.15.0"),
						Name:    convert.Pointer("Release v1.15.0"),
					},
					{
						TagName: convert.Pointer("v1.16.0"),
						Name:    convert.Pointer("Release v1.16.0"),
					},
					{
						TagName: convert.Pointer("v1.14.0"),
						Name:    convert.Pointer("Release v1.14.0"),
					},
				}
				s.mockGithub.EXPECT().GetReleases(owner, "framework", &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return(releases, nil).Once()
			},
			want:    true,
			wantErr: nil,
		},
		{
			name: "release does not exist",
			repo: "gin",
			tag:  "v1.5.0",
			setup: func() {
				releases := []*github.RepositoryRelease{
					{
						TagName: convert.Pointer("v1.4.0"),
						Name:    convert.Pointer("Release v1.4.0"),
					},
					{
						TagName: convert.Pointer("v1.3.0"),
						Name:    convert.Pointer("Release v1.3.0"),
					},
				}
				s.mockGithub.EXPECT().GetReleases(owner, "gin", &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return(releases, nil).Once()
			},
			want:    false,
			wantErr: nil,
		},
		{
			name: "no releases found",
			repo: "new-repo",
			tag:  "v1.0.0",
			setup: func() {
				s.mockGithub.EXPECT().GetReleases(owner, "new-repo", &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return([]*github.RepositoryRelease{}, nil).Once()
			},
			want:    false,
			wantErr: nil,
		},
		{
			name: "github API error",
			repo: "framework",
			tag:  "v1.16.0",
			setup: func() {
				s.mockGithub.EXPECT().GetReleases(owner, "framework", &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return(nil, assert.AnError).Once()
			},
			want:    false,
			wantErr: assert.AnError,
		},
		{
			name: "release with nil TagName",
			repo: "fiber",
			tag:  "v1.0.0",
			setup: func() {
				releases := []*github.RepositoryRelease{
					{
						TagName: nil,
						Name:    convert.Pointer("Release without tag"),
					},
					{
						TagName: convert.Pointer("v1.0.0"),
						Name:    convert.Pointer("Release v1.0.0"),
					},
				}
				s.mockGithub.EXPECT().GetReleases(owner, "fiber", &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return(releases, nil).Once()
			},
			want:    true,
			wantErr: nil,
		},
		{
			name: "case sensitive tag matching",
			repo: "example",
			tag:  "V1.0.0",
			setup: func() {
				releases := []*github.RepositoryRelease{
					{
						TagName: convert.Pointer("v1.0.0"),
						Name:    convert.Pointer("Release v1.0.0"),
					},
				}
				s.mockGithub.EXPECT().GetReleases(owner, "example", &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return(releases, nil).Once()
			},
			want:    false,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			result, err := s.release.isReleaseExist(tt.repo, tt.tag)

			s.Equal(tt.want, result)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_pushBranch() {
	var (
		repo   = "framework"
		branch = "v1.16.x"
	)

	tests := []struct {
		name    string
		real    bool
		setup   func()
		wantErr error
	}{
		{
			name: "preview mode - skips push",
			real: false,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()
			},
			wantErr: nil,
		},
		{
			name: "real mode - spinner fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					Return(assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "real mode - CheckBranchExists fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockGithub.EXPECT().CheckBranchExists("goravel", "framework", "v1.16.x").
					Return(false, assert.AnError).Once()
			},
			wantErr: fmt.Errorf("failed to check branch v1.16.x exist for framework: %w", assert.AnError),
		},
		{
			name: "real mode - branch already exists",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockGithub.EXPECT().CheckBranchExists("goravel", "framework", "v1.16.x").
					Return(true, nil).Once()
			},
			wantErr: nil,
		},
		{
			name: "real mode - CreateBranch fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockGithub.EXPECT().CheckBranchExists("goravel", "framework", "v1.16.x").
					Return(false, nil).Once()

				s.mockGithub.EXPECT().CreateBranch("goravel", "framework", "v1.16.x").
					Return(assert.AnError).Once()
			},
			wantErr: fmt.Errorf("failed to create branch v1.16.x for framework: %w", assert.AnError),
		},
		{
			name: "real mode - successful push",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockGithub.EXPECT().CheckBranchExists("goravel", "framework", "v1.16.x").
					Return(false, nil).Once()

				s.mockGithub.EXPECT().CreateBranch("goravel", "framework", "v1.16.x").
					Return(nil).Once()
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.release.real = tt.real
			tt.setup()

			err := s.release.pushBranch(repo, branch)

			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_releaseRepo() {
	var (
		repo   = "framework"
		tag    = "v1.16.0"
		branch = "v1.16.x"
		notes  = &github.RepositoryReleaseNotes{
			Name: "Release v1.16.0",
			Body: "## What's Changed\n* Feature A\n* Bug fix B",
		}
		releaseInfo = &ReleaseInformation{
			currentTag: "v1.16.0",
			latestTag:  "v1.15.0",
			notes:      notes,
			repo:       repo,
			tag:        tag,
		}
	)

	tests := []struct {
		name        string
		releaseInfo *ReleaseInformation
		setup       func()
		wantErr     error
	}{
		{
			name:        "release already exists",
			releaseInfo: releaseInfo,
			setup: func() {
				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return([]*github.RepositoryRelease{
					{
						TagName: convert.Pointer("v1.15.0"),
						Name:    convert.Pointer("Release v1.15.0"),
					},
					{
						TagName: convert.Pointer("v1.16.0"),
						Name:    convert.Pointer("Release v1.16.0"),
					},
				}, nil).Once()
			},
			wantErr: nil,
		},
		{
			name:        "release does not exist, create release succeeds",
			releaseInfo: releaseInfo,
			setup: func() {
				// Mock isReleaseExist returns false
				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return([]*github.RepositoryRelease{
					{
						TagName: convert.Pointer("v1.15.0"),
						Name:    convert.Pointer("Release v1.15.0"),
					},
				}, nil).Once()

				s.mockGithub.EXPECT().CheckBranchExists(owner, repo, branch).Return(false, nil).Once()

				// Mock createRelease succeeds
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer("master"),
					Name:            convert.Pointer(notes.Name),
					Body:            convert.Pointer(notes.Body),
				}).Return(nil, nil).Once()
			},
			wantErr: nil,
		},
		{
			name:        "isReleaseExist fails",
			releaseInfo: releaseInfo,
			setup: func() {
				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name:        "createRelease fails",
			releaseInfo: releaseInfo,
			setup: func() {
				// Mock isReleaseExist returns false
				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
					Page:    1,
					PerPage: 10,
				}).Return([]*github.RepositoryRelease{
					{
						TagName: convert.Pointer("v1.15.0"),
						Name:    convert.Pointer("Release v1.15.0"),
					},
				}, nil).Once()

				s.mockGithub.EXPECT().CheckBranchExists(owner, repo, branch).Return(false, nil).Once()

				// Mock createRelease fails
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer("master"),
					Name:            convert.Pointer(notes.Name),
					Body:            convert.Pointer(notes.Body),
				}).Return(nil, assert.AnError).Once()
			},
			wantErr: fmt.Errorf("failed to create release: %w", assert.AnError),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			err := s.release.releaseRepo(tt.releaseInfo)

			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_parseExtraPackages() {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single package",
			input: "gin@v1.17.1",
			want:  map[string]string{"gin": "v1.17.1"},
		},
		{
			name:  "multiple packages",
			input: "gin@v1.17.1,fiber@v1.17.1",
			want:  map[string]string{"gin": "v1.17.1", "fiber": "v1.17.1"},
		},
		{
			name:  "with spaces around commas",
			input: "gin@v1.17.1, fiber@v1.17.1",
			want:  map[string]string{"gin": "v1.17.1", "fiber": "v1.17.1"},
		},
		{
			name:  "malformed - no @",
			input: "ginv1.17.1",
			want:  map[string]string{},
		},
		{
			name:  "malformed - empty name",
			input: "@v1.17.1",
			want:  map[string]string{},
		},
		{
			name:  "malformed - empty version",
			input: "gin@",
			want:  map[string]string{},
		},
		{
			name:  "mixed valid and invalid",
			input: "gin@v1.17.1,invalid,fiber@v1.17.1",
			want:  map[string]string{"gin": "v1.17.1", "fiber": "v1.17.1"},
		},
		{
			name:  "trailing comma",
			input: "gin@v1.17.1,",
			want:  map[string]string{"gin": "v1.17.1"},
		},
		{
			name:  "multiple @ signs uses first as separator",
			input: "gin@v1.17.1@extra",
			want:  map[string]string{"gin": "v1.17.1@extra"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := parseExtraPackages(tt.input)
			s.Equal(tt.want, result)
		})
	}
}

func (s *ReleaseTestSuite) Test_sortedKeys() {
	tests := []struct {
		name  string
		input map[string]string
		want  []string
	}{
		{
			name:  "empty map",
			input: map[string]string{},
			want:  []string{},
		},
		{
			name:  "single key",
			input: map[string]string{"gin": "v1.17.1"},
			want:  []string{"gin"},
		},
		{
			name:  "multiple keys sorted",
			input: map[string]string{"fiber": "v1.17.1", "gin": "v1.17.1", "example": "v1.0.0"},
			want:  []string{"example", "fiber", "gin"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := sortedKeys(tt.input)
			s.Equal(tt.want, result)
		})
	}
}

func (s *ReleaseTestSuite) Test_buildExampleDeps() {
	tests := []struct {
		name          string
		frameworkRef  string
		extraPackages map[string]string
		useExtraTags  bool
		want          []string
	}{
		{
			name:          "no extra packages",
			frameworkRef:  "v1.17.x",
			extraPackages: nil,
			useExtraTags:  false,
			want: []string{
				"go get github.com/goravel/framework@v1.17.x",
				"go mod edit -replace github.com/goravel/framework=github.com/goravel/framework@$(go list -m -f '{{.Version}}' github.com/goravel/framework)",
			},
		},
		{
			name:          "with extras, useExtraTags=false (PR creation phase)",
			frameworkRef:  "v1.17.x",
			extraPackages: map[string]string{"gin": "v1.17.1", "fiber": "v1.17.1"},
			useExtraTags:  false,
			want: []string{
				"go get github.com/goravel/framework@v1.17.x",
				"go get github.com/goravel/fiber@v1.17.x",
				"go get github.com/goravel/gin@v1.17.x",
				"go mod edit -replace github.com/goravel/framework=github.com/goravel/framework@$(go list -m -f '{{.Version}}' github.com/goravel/framework)",
			},
		},
		{
			name:          "with extras, useExtraTags=true (update deps phase)",
			frameworkRef:  "v1.17.3",
			extraPackages: map[string]string{"gin": "v1.17.1", "fiber": "v1.17.1"},
			useExtraTags:  true,
			want: []string{
				"go get github.com/goravel/framework@v1.17.3",
				"go get github.com/goravel/fiber@v1.17.1",
				"go get github.com/goravel/gin@v1.17.1",
				"go mod edit -replace github.com/goravel/framework=github.com/goravel/framework@$(go list -m -f '{{.Version}}' github.com/goravel/framework)",
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := s.release.buildExampleDeps(tt.frameworkRef, tt.extraPackages, tt.useExtraTags)
			s.Equal(tt.want, result)
		})
	}
}

func (s *ReleaseTestSuite) Test_exampleDependencyCommands() {
	result := s.release.exampleDependencyCommands("master")
	s.Equal([]string{
		"go get github.com/goravel/framework@master",
		"go mod edit -replace github.com/goravel/framework=github.com/goravel/framework@$(go list -m -f '{{.Version}}' github.com/goravel/framework)",
		"go get github.com/goravel/gin@master",
		"go get github.com/goravel/fiber@master",
		"go get github.com/goravel/s3@master",
		"go get github.com/goravel/oss@master",
		"go get github.com/goravel/cos@master",
		"go get github.com/goravel/minio@master",
		"go get github.com/goravel/postgres@master",
		"go get github.com/goravel/mysql@master",
		"go get github.com/goravel/sqlserver@master",
		"go get github.com/goravel/sqlite@master",
		"go get github.com/goravel/redis@master",
		"go get github.com/goravel/openai@master",
		"go get github.com/goravel/anthropic@master",
		"go get github.com/goravel/gemini@master",
	}, result)
}

func (s *ReleaseTestSuite) Test_allCheckRunsCompleted() {
	tests := []struct {
		name      string
		checkRuns []*github.CheckRun
		want      bool
	}{
		{
			name:      "empty check runs",
			checkRuns: []*github.CheckRun{},
			want:      false,
		},
		{
			name: "all completed",
			checkRuns: []*github.CheckRun{
				{Status: convert.Pointer("completed")},
				{Status: convert.Pointer("completed")},
			},
			want: true,
		},
		{
			name: "some pending",
			checkRuns: []*github.CheckRun{
				{Status: convert.Pointer("completed")},
				{Status: convert.Pointer("in_progress")},
			},
			want: false,
		},
		{
			name: "all queued",
			checkRuns: []*github.CheckRun{
				{Status: convert.Pointer("queued")},
				{Status: convert.Pointer("queued")},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := allCheckRunsCompleted(tt.checkRuns)
			s.Equal(tt.want, result)
		})
	}
}

func (s *ReleaseTestSuite) Test_anyCheckRunFailed() {
	tests := []struct {
		name      string
		checkRuns []*github.CheckRun
		want      bool
	}{
		{
			name:      "empty check runs",
			checkRuns: []*github.CheckRun{},
			want:      false,
		},
		{
			name: "no failures",
			checkRuns: []*github.CheckRun{
				{Conclusion: convert.Pointer("success")},
				{Conclusion: convert.Pointer("neutral")},
				{Conclusion: convert.Pointer("skipped")},
			},
			want: false,
		},
		{
			name: "one failure",
			checkRuns: []*github.CheckRun{
				{Conclusion: convert.Pointer("success")},
				{Conclusion: convert.Pointer("failure")},
			},
			want: true,
		},
		{
			name: "cancelled",
			checkRuns: []*github.CheckRun{
				{Conclusion: convert.Pointer("success")},
				{Conclusion: convert.Pointer("cancelled")},
			},
			want: true,
		},
		{
			name: "timed_out",
			checkRuns: []*github.CheckRun{
				{Conclusion: convert.Pointer("timed_out")},
			},
			want: true,
		},
		{
			name: "action_required",
			checkRuns: []*github.CheckRun{
				{Conclusion: convert.Pointer("action_required")},
			},
			want: true,
		},
		{
			name: "nil conclusion",
			checkRuns: []*github.CheckRun{
				{Conclusion: nil},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := anyCheckRunFailed(tt.checkRuns)
			s.Equal(tt.want, result)
		})
	}
}

func (s *ReleaseTestSuite) Test_upgradePRTitle() {
	result := s.release.upgradePRTitle("v1.17.0")
	s.Equal("chore: Upgrade framework to v1.17.0 (auto)", result)
}

func (s *ReleaseTestSuite) Test_findExistingUpgradePR() {
	var (
		repo  = "gin"
		title = s.release.upgradePRTitle("v1.17.0")
	)

	tests := []struct {
		name    string
		setup   func()
		wantPR  *github.PullRequest
		wantErr error
	}{
		{
			name: "no existing PR",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()
			},
			wantPR: nil,
		},
		{
			name: "found existing PR",
			setup: func() {
				existing := &github.PullRequest{
					Title:  convert.Pointer(title),
					Number: convert.Pointer(123),
				}
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{existing}, nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:  convert.Pointer(title),
				Number: convert.Pointer(123),
			},
		},
		{
			name: "api error",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return(nil, assert.AnError).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "multiple PRs, title mismatch",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{
					{Title: convert.Pointer("other title"), Number: convert.Pointer(1)},
					{Title: convert.Pointer("another title"), Number: convert.Pointer(2)},
				}, nil).Once()
			},
			wantPR: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			pr, err := s.release.findExistingUpgradePR(repo, title)

			s.Equal(tt.wantPR, pr)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_findMergedUpgradePR() {
	var (
		repo  = "gin"
		title = s.release.upgradePRTitle("v1.17.0")
	)

	tests := []struct {
		name    string
		setup   func()
		wantPR  *github.PullRequest
		wantErr error
	}{
		{
			name: "no merged PR matching",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{}, nil).Once()
			},
			wantPR: nil,
		},
		{
			name: "found merged PR with matching title",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{
					{Title: convert.Pointer(title), Number: convert.Pointer(100), MergedAt: &github.Timestamp{}},
				}, nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:    convert.Pointer(title),
				Number:   convert.Pointer(100),
				MergedAt: &github.Timestamp{},
			},
		},
		{
			name: "closed but not merged",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return([]*github.PullRequest{
					{Title: convert.Pointer(title), Number: convert.Pointer(200), MergedAt: nil},
				}, nil).Once()
			},
			wantPR: nil,
		},
		{
			name: "api error",
			setup: func() {
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "closed",
				}).Return(nil, assert.AnError).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			pr, err := s.release.findMergedUpgradePR(repo, title)

			s.Equal(tt.wantPR, pr)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_triggerExampleAICI() {
	tests := []struct {
		name    string
		pr      *github.PullRequest
		setup   func()
		wantErr error
	}{
		{
			name: "nil PR - skipped",
			pr:   nil,
			setup: func() {
			},
			wantErr: nil,
		},
		{
			name: "nil Head - skipped",
			pr: &github.PullRequest{
				Head: nil,
			},
			setup: func() {
			},
			wantErr: nil,
		},
		{
			name: "nil Head.Ref - skipped",
			pr: &github.PullRequest{
				Head: &github.PullRequestBranch{
					Ref: nil,
				},
			},
			setup: func() {
			},
			wantErr: nil,
		},
		{
			name: "spinner fails",
			pr: &github.PullRequest{
				Head: &github.PullRequestBranch{
					Ref: convert.Pointer("auto-upgrade/v1.17.0"),
				},
			},
			setup: func() {
				s.mockContext.EXPECT().Spinner(mock.AnythingOfType("string"), mock.AnythingOfType("console.SpinnerOption")).
					Return(assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "workflow dispatch succeeds",
			pr: &github.PullRequest{
				Head: &github.PullRequestBranch{
					Ref: convert.Pointer("auto-upgrade/v1.17.0"),
				},
			},
			setup: func() {
				s.mockContext.EXPECT().Spinner(mock.AnythingOfType("string"), mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockGithub.EXPECT().CreateWorkflowDispatchEvent(owner, "example", "test-ai.yml", "auto-upgrade/v1.17.0").
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name: "workflow dispatch fails",
			pr: &github.PullRequest{
				Head: &github.PullRequestBranch{
					Ref: convert.Pointer("auto-upgrade/v1.17.0"),
				},
			},
			setup: func() {
				s.mockContext.EXPECT().Spinner(mock.AnythingOfType("string"), mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				s.mockGithub.EXPECT().CreateWorkflowDispatchEvent(owner, "example", "test-ai.yml", "auto-upgrade/v1.17.0").
					Return(assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			err := s.release.triggerExampleAICI(tt.pr)

			s.Equal(tt.wantErr, err)
		})
	}
}
