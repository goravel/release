package commands

import (
	"fmt"
	"testing"

	"github.com/google/go-github/v73/github"
	"github.com/goravel/framework/contracts/console"
	mocksconsole "github.com/goravel/framework/mocks/console"
	mocksclient "github.com/goravel/framework/mocks/http/client"
	"github.com/goravel/framework/support/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	mocksservices "goravel/app/mocks/services"
)

type ReleaseTestSuite struct {
	suite.Suite
	mockContext *mocksconsole.Context
	mockGithub  *mocksservices.Github
	mockProcess *mocksservices.Process
	mockHttp    *mocksclient.Request
	release     *Release
}

func TestReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaseTestSuite))
}

func (s *ReleaseTestSuite) SetupTest() {
	packages = []string{
		"gin",
		"fiber",
	}

	s.mockContext = mocksconsole.NewContext(s.T())
	s.mockGithub = mocksservices.NewGithub(s.T())
	s.mockProcess = mocksservices.NewProcess(s.T())
	s.mockHttp = mocksclient.NewRequest(s.T())

	s.release = &Release{
		real:    true,
		github:  s.mockGithub,
		process: s.mockProcess,
		http:    s.mockHttp,
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
			name:       "happy path - not real",
			real:       false,
			pr:         pr,
			setup:      func() {},
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
					State:   convert.Pointer("merged"),
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
					State:   convert.Pointer("open"),
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
		pr1   = &github.PullRequest{
			Number:  convert.Pointer(1),
			HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
			State:   convert.Pointer("open"),
		}
		repoToPR map[string]*github.PullRequest
	)

	beforeEach := func() {
		repoToPR = map[string]*github.PullRequest{
			repo1: pr1,
			repo2: nil,
		}
	}

	tests := []struct {
		name    string
		setup   func()
		wantErr error
	}{
		{
			name: "failed to choice",
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
			name: "choice is not check",
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
			name: "failed to check PR merge status",
			setup: func() {
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "pr is merged",
			setup: func() {
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						State:   convert.Pointer("merged"),
					}, nil).Once()
			},
		},
		{
			name: "pr is not merged",
			setup: func() {
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("Check", nil).Once()
				s.mockGithub.EXPECT().GetPullRequest(owner, repo1, *pr1.Number).
					Return(&github.PullRequest{
						Number:  convert.Pointer(1),
						HTMLURL: convert.Pointer("https://github.com/goravel/gin/pull/1"),
						State:   convert.Pointer("open"),
					}, nil).Once()
				s.mockContext.EXPECT().Choice("Check PRs merge status?", []console.Choice{
					{
						Key:   "Check",
						Value: "Check",
					},
				}).Return("", assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			beforeEach()
			tt.setup()

			err := s.release.checkPRsMergeStatus(s.mockContext, repoToPR)

			s.Equal(tt.wantErr, err)
		})
	}
}

// func (s *ReleaseTestSuite) Test_confirmReleaseMajor() {
// 	var (
// 		frameworkTag = "v1.16.0"
// 		packageTag   = "v1.16.0"
// 	)

// 	tests := []struct {
// 		name            string
// 		frameworkTag    string
// 		packageTag      string
// 		real            bool
// 		setup           func()
// 		wantRepoToNotes map[string]*github.RepositoryReleaseNotes
// 		wantErr         error
// 	}{
// 		{
// 			name:         "successful confirmation for all repositories - non-real mode",
// 			frameworkTag: frameworkTag,
// 			packageTag:   packageTag,
// 			real:         false,
// 			setup: func() {
// 				// Mock framework release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Times(2) // divider, before notes

// 				// Mock getLatestTag for framework
// 				mockFrameworkRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(mockFrameworkRelease, nil).Once()

// 				// Mock getFrameworkCurrentTag
// 				mockResponse := mocksclient.NewResponse(s.T())
// 				mockResponse.EXPECT().Body().Return(`package support

// const (
// 	Version = "v1.16.0"
// 	// other constants...
// )`, nil)
// 				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
// 					Return(mockResponse, nil).Once()

// 				// Mock generateReleaseNotes for framework
// 				frameworkNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(frameworkNotes, nil).Once()

// 				// Mock framework confirmation
// 				s.mockContext.EXPECT().NewLine().Once()
// 				s.mockContext.EXPECT().Confirm("goravel/framework confirmed?").Return(true).Once()

// 				// Mock gin package release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once() // divider

// 				// Mock getLatestTag for gin
// 				mockGinRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin").Return(mockGinRelease, nil).Once()

// 				// Mock generateReleaseNotes for gin
// 				ginNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/gin/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "gin", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(ginNotes, nil).Once()

// 				// Mock gin confirmation
// 				s.mockContext.EXPECT().NewLine().Once()
// 				s.mockContext.EXPECT().Confirm("goravel/gin confirmed?").Return(true).Once()

// 				// Mock fiber package release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once() // divider

// 				// Mock getLatestTag for fiber
// 				mockFiberRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "fiber").Return(mockFiberRelease, nil).Once()

// 				// Mock generateReleaseNotes for fiber
// 				fiberNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/fiber/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "fiber", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(fiberNotes, nil).Once()

// 				// Mock fiber confirmation
// 				s.mockContext.EXPECT().NewLine().Once()
// 				s.mockContext.EXPECT().Confirm("goravel/fiber confirmed?").Return(true).Once()
// 			},
// 			wantRepoToNotes: map[string]*github.RepositoryReleaseNotes{
// 				"framework": {
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				},
// 				"gin": {
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/gin/compare/v1.15.0...v1.16.0",
// 				},
// 				"fiber": {
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/fiber/compare/v1.15.0...v1.16.0",
// 				},
// 			},
// 			wantErr: nil,
// 		},
// 		{
// 			name:         "successful confirmation for all repositories - real mode",
// 			frameworkTag: frameworkTag,
// 			packageTag:   packageTag,
// 			real:         true,
// 			setup: func() {
// 				// Mock framework release information (no confirmations in real mode)
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Times(2) // divider, before notes

// 				// Mock getLatestTag for framework
// 				mockFrameworkRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(mockFrameworkRelease, nil).Once()

// 				// Mock getFrameworkCurrentTag
// 				mockResponse := mocksclient.NewResponse(s.T())
// 				mockResponse.EXPECT().Body().Return(`package support

// const (
// 	Version = "v1.16.0"
// 	// other constants...
// )`, nil)
// 				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
// 					Return(mockResponse, nil).Once()

// 				// Mock generateReleaseNotes for framework
// 				frameworkNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(frameworkNotes, nil).Once()

// 				// Mock NewLine after framework (in real mode)
// 				s.mockContext.EXPECT().NewLine().Once()

// 				// Mock gin package release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once() // divider

// 				// Mock getLatestTag for gin
// 				mockGinRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin").Return(mockGinRelease, nil).Once()

// 				// Mock generateReleaseNotes for gin
// 				ginNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/gin/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "gin", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(ginNotes, nil).Once()

// 				// Mock NewLine after gin (in real mode)
// 				s.mockContext.EXPECT().NewLine().Once()

// 				// Mock fiber package release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once() // divider

// 				// Mock getLatestTag for fiber
// 				mockFiberRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "fiber").Return(mockFiberRelease, nil).Once()

// 				// Mock generateReleaseNotes for fiber
// 				fiberNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/fiber/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "fiber", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(fiberNotes, nil).Once()

// 				// Mock NewLine after fiber (in real mode)
// 				s.mockContext.EXPECT().NewLine().Once()
// 			},
// 			wantRepoToNotes: map[string]*github.RepositoryReleaseNotes{
// 				"framework": {
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				},
// 				"gin": {
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/gin/compare/v1.15.0...v1.16.0",
// 				},
// 				"fiber": {
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/fiber/compare/v1.15.0...v1.16.0",
// 				},
// 			},
// 			wantErr: nil,
// 		},
// 		{
// 			name:         "framework getLatestTag fails",
// 			frameworkTag: frameworkTag,
// 			packageTag:   packageTag,
// 			real:         false,
// 			setup: func() {
// 				// Mock framework release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once()

// 				// Mock getLatestTag for framework fails
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(nil, assert.AnError).Once()
// 			},
// 			wantRepoToNotes: nil,
// 			wantErr:         fmt.Errorf("failed to get latest tag: %w", assert.AnError),
// 		},
// 		{
// 			name:         "framework confirmation rejected",
// 			frameworkTag: frameworkTag,
// 			packageTag:   packageTag,
// 			real:         false,
// 			setup: func() {
// 				// Mock framework release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Times(2) // divider, before notes

// 				// Mock getLatestTag for framework
// 				mockFrameworkRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(mockFrameworkRelease, nil).Once()

// 				// Mock getFrameworkCurrentTag
// 				mockResponse := mocksclient.NewResponse(s.T())
// 				mockResponse.EXPECT().Body().Return(`package support

// 		const (
// 			Version = "v1.16.0"
// 			// other constants...
// 		)`, nil)
// 				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
// 					Return(mockResponse, nil).Once()

// 				// Mock generateReleaseNotes for framework
// 				frameworkNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(frameworkNotes, nil).Once()

// 				// Mock framework confirmation rejected
// 				s.mockContext.EXPECT().NewLine().Once()
// 				s.mockContext.EXPECT().Confirm("goravel/framework confirmed?").Return(false).Once()
// 			},
// 			wantRepoToNotes: nil,
// 			wantErr:         fmt.Errorf("goravel/framework not confirmed"),
// 		},
// 		{
// 			name:         "gin package getLatestTag fails",
// 			frameworkTag: frameworkTag,
// 			packageTag:   packageTag,
// 			real:         false,
// 			setup: func() {
// 				// Mock framework release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Times(2) // divider, before notes

// 				// Mock getLatestTag for framework
// 				mockFrameworkRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(mockFrameworkRelease, nil).Once()

// 				// Mock getFrameworkCurrentTag
// 				mockResponse := mocksclient.NewResponse(s.T())
// 				mockResponse.EXPECT().Body().Return(`package support

// 		const (
// 			Version = "v1.16.0"
// 			// other constants...
// 		)`, nil)
// 				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
// 					Return(mockResponse, nil).Once()

// 				// Mock generateReleaseNotes for framework
// 				frameworkNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(frameworkNotes, nil).Once()

// 				// Mock framework confirmation
// 				s.mockContext.EXPECT().NewLine().Once()
// 				s.mockContext.EXPECT().Confirm("goravel/framework confirmed?").Return(true).Once()

// 				// Mock gin package release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once() // divider

// 				// Mock getLatestTag for gin fails
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin").Return(nil, assert.AnError).Once()
// 			},
// 			wantRepoToNotes: nil,
// 			wantErr:         fmt.Errorf("failed to get and print gin release infomration: %w", fmt.Errorf("failed to get latest tag for goravel/gin: %w", assert.AnError)),
// 		},
// 		{
// 			name:         "gin package confirmation rejected",
// 			frameworkTag: frameworkTag,
// 			packageTag:   packageTag,
// 			real:         false,
// 			setup: func() {
// 				// Mock framework release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Times(2) // divider, before notes

// 				// Mock getLatestTag for framework
// 				mockFrameworkRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(mockFrameworkRelease, nil).Once()

// 				// Mock getFrameworkCurrentTag
// 				mockResponse := mocksclient.NewResponse(s.T())
// 				mockResponse.EXPECT().Body().Return(`package support

// 		const (
// 			Version = "v1.16.0"
// 			// other constants...
// 		)`, nil)
// 				s.mockHttp.EXPECT().Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go").
// 					Return(mockResponse, nil).Once()

// 				// Mock generateReleaseNotes for framework
// 				frameworkNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "framework", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(frameworkNotes, nil).Once()

// 				// Mock framework confirmation
// 				s.mockContext.EXPECT().NewLine().Once()
// 				s.mockContext.EXPECT().Confirm("goravel/framework confirmed?").Return(true).Once()

// 				// Mock gin package release information
// 				s.mockContext.EXPECT().TwoColumnDetail("", "", '-').Once()
// 				s.mockContext.EXPECT().NewLine().Once() // divider

// 				// Mock getLatestTag for gin
// 				mockGinRelease := &github.RepositoryRelease{
// 					TagName: convert.Pointer("v1.15.0"),
// 					Name:    convert.Pointer("Release v1.15.0"),
// 					Body:    convert.Pointer("Previous release"),
// 				}
// 				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin").Return(mockGinRelease, nil).Once()

// 				// Mock generateReleaseNotes for gin
// 				ginNotes := &github.RepositoryReleaseNotes{
// 					Name: "Release v1.16.0",
// 					Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/gin/compare/v1.15.0...v1.16.0",
// 				}
// 				s.mockGithub.EXPECT().GenerateReleaseNotes(owner, "gin", &github.GenerateNotesOptions{
// 					TagName:         "v1.16.0",
// 					PreviousTagName: convert.Pointer("v1.15.0"),
// 					TargetCommitish: convert.Pointer("master"),
// 				}).Return(ginNotes, nil).Once()

// 				// Mock NewLine after gin package notes (before confirmation)
// 				s.mockContext.EXPECT().NewLine().Once()

// 				// Mock gin confirmation rejected
// 				s.mockContext.EXPECT().Confirm("goravel/gin confirmed?").Return(false).Once()
// 			},
// 			wantRepoToNotes: nil,
// 			wantErr:         fmt.Errorf("goravel/gin not confirmed"),
// 		},
// 	}

// 	for _, tt := range tests {
// 		s.Run(tt.name, func() {
// 			s.release.real = tt.real
// 			tt.setup()

// 			repoToNotes, err := s.release.confirmReleaseMajor(s.mockContext, tt.frameworkTag, tt.packageTag)

// 			s.Equal(tt.wantRepoToNotes, repoToNotes)
// 			s.Equal(tt.wantErr, err)
// 		})
// 	}
// }

func (s *ReleaseTestSuite) Test_createRelease() {
	var (
		repo  = "goravel"
		tag   = "v1.0.0"
		notes = &github.RepositoryReleaseNotes{
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
			name:  "happy path - not real",
			real:  false,
			setup: func() {},
		},
		{
			name: "happy path - real",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer("master"),
					Name:            convert.Pointer(notes.Name),
					Body:            convert.Pointer(notes.Body),
				}).Return(nil, nil).Once()
			},
		},
		{
			name: "failed to create release",
			real: true,
			setup: func() {
				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
					TagName:         convert.Pointer(tag),
					TargetCommitish: convert.Pointer("master"),
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
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					Run(func(msg string, opts console.SpinnerOption) {
						if opts.Action != nil {
							opts.Action()
						}
					}).Return(nil).Once()
				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer(fmt.Sprintf("https://github.com/%s/%s/pull/%s", owner, repo, upgradeBranch)),
			},
			wantErr: nil,
		},
		{
			name: "real mode - spinner fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					Return(assert.AnError).Once()
				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "real mode - clone and mod fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock the first process.Run call (clone and mod) to fail
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("", assert.AnError).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to clone repo and mod for example: %w", assert.AnError),
		},
		{
			name: "real mode - check status fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status fails
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("", assert.AnError).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to check status for example: %w", assert.AnError),
		},
		{
			name: "real mode - working tree clean, no changes needed",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns clean working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("nothing to commit, working tree clean", nil).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: nil,
		},
		{
			name: "real mode - push branch fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch fails
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("", assert.AnError).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to push upgrade branch for example: %w", assert.AnError),
		},
		{
			name: "real mode - push branch output doesn't contain commit message",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch succeeds but output doesn't contain commit message
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("pushed to remote", nil).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: fmt.Errorf("failed to push upgrade branch for example: pushed to remote"),
		},
		{
			name: "real mode - get pull requests fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch succeeds
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("chore: Upgrade framework to v1.16.0 (auto)", nil).Once()

				// Mock get pull requests fails
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return(nil, assert.AnError).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "real mode - existing PR found",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch succeeds
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("chore: Upgrade framework to v1.16.0 (auto)", nil).Once()

				// Mock get pull requests returns existing PR
				existingPR := &github.PullRequest{
					Title:   convert.Pointer(prTitle),
					HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/123"),
					Number:  convert.Pointer(123),
				}
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{existingPR}, nil).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			// Note: Due to variable shadowing bug in the actual implementation,
			// the PR is found but not returned because the inner 'pr' variable
			// shadows the outer one
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/123"),
				Number:  convert.Pointer(123),
			},
			wantErr: nil,
		},
		{
			name: "real mode - create new PR fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch succeeds
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("chore: Upgrade framework to v1.16.0 (auto)", nil).Once()

				// Mock get pull requests returns no existing PR
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				// Mock create pull request fails
				s.mockGithub.EXPECT().CreatePullRequest(owner, repo, &github.NewPullRequest{
					Title: convert.Pointer(prTitle),
					Head:  convert.Pointer(upgradeBranch),
					Base:  convert.Pointer("master"),
				}).Return(nil, assert.AnError).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR:  nil,
			wantErr: assert.AnError,
		},
		{
			name: "real mode - successful PR creation",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch succeeds
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("chore: Upgrade framework to v1.16.0 (auto)", nil).Once()

				// Mock get pull requests returns no existing PR
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				// Mock create pull request succeeds
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

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", nil).Once()
			},
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/456"),
				Number:  convert.Pointer(456),
			},
			wantErr: nil,
		},
		{
			name: "real mode - cleanup fails but doesn't affect result",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Creating upgrade PR for example...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock successful clone and mod
				s.mockProcess.EXPECT().Run(`rm -rf example && git clone --depth=1 git@github.com:goravel/example.git &&
cd example && git checkout master && git branch -D auto-upgrade/v1.16.0 2>/dev/null || true && git checkout -b auto-upgrade/v1.16.0 &&
go get github.com/goravel/framework@v1.16.0 && go get github.com/goravel/gin@v1.4.0 && go mod tidy`).
					Return("success", nil).Once()

				// Mock check status returns dirty working tree
				s.mockProcess.EXPECT().Run(`cd example && git status`).Return("modified: go.mod", nil).Once()

				// Mock push branch succeeds
				s.mockProcess.EXPECT().Run(`cd example && git add . && git commit -m "chore: Upgrade framework to v1.16.0 (auto)" && git push origin auto-upgrade/v1.16.0 -f`).
					Return("chore: Upgrade framework to v1.16.0 (auto)", nil).Once()

				// Mock get pull requests returns no existing PR
				s.mockGithub.EXPECT().GetPullRequests(owner, repo, &github.PullRequestListOptions{
					State: "open",
				}).Return([]*github.PullRequest{}, nil).Once()

				// Mock create pull request succeeds
				newPR := &github.PullRequest{
					Title:   convert.Pointer(prTitle),
					HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/789"),
					Number:  convert.Pointer(789),
				}
				s.mockGithub.EXPECT().CreatePullRequest(owner, repo, &github.NewPullRequest{
					Title: convert.Pointer(prTitle),
					Head:  convert.Pointer(upgradeBranch),
					Base:  convert.Pointer("master"),
				}).Return(newPR, nil).Once()

				// Mock the cleanup call in defer fails
				s.mockProcess.EXPECT().Run("rm -rf example").Return("", assert.AnError).Once()
			},
			wantPR: &github.PullRequest{
				Title:   convert.Pointer(prTitle),
				HTMLURL: convert.Pointer("https://github.com/goravel/example/pull/789"),
				Number:  convert.Pointer(789),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.release.real = tt.real
			tt.setup()

			pr, err := s.release.createUpgradePR(s.mockContext, repo, frameworkTag, dependencies)

			s.Equal(tt.wantPR, pr)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_getFrameworkCurrentTag() {
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
				mockResponse.On("Body").Return(`package support

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
				mockResponse.On("Body").Return(`package support

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
				mockResponse.On("Body").Return(`package support

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
				mockResponse.On("Body").Return(`package support

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
				mockResponse.On("Body").Return(`package support

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
				mockResponse.On("Body").Return("", assert.AnError)

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
				mockResponse.On("Body").Return(`package support

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

			version, err := s.release.getFrameworkCurrentTag()

			s.Equal(tt.wantVersion, version)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_generateReleaseNotes() {
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
					TargetCommitish: convert.Pointer("master"),
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
					TargetCommitish: convert.Pointer("master"),
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
					TargetCommitish: convert.Pointer("master"),
				}).Return(nil, nil).Once()
			},
			wantNotes: nil,
			wantErr:   fmt.Errorf("failed to generate release notes, notes is nil"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			notes, err := s.release.generateReleaseNotes(tt.repo, tt.tagName, tt.previousTagName)

			s.Equal(tt.wantNotes, notes)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *ReleaseTestSuite) Test_getLatestTag() {
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
				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin").Return(mockRelease, nil).Once()
			},
			wantTag: "v1.16.0",
			wantErr: nil,
		},
		{
			name: "github API error",
			repo: "gin",
			setup: func() {
				s.mockGithub.EXPECT().GetLatestRelease(owner, "gin").Return(nil, assert.AnError).Once()
			},
			wantTag: "",
			wantErr: assert.AnError,
		},
		{
			name: "github API returns nil release",
			repo: "fiber",
			setup: func() {
				s.mockGithub.EXPECT().GetLatestRelease(owner, "fiber").Return(nil, nil).Once()
			},
			wantTag: "",
			wantErr: fmt.Errorf("latest release is nil for %s/fiber", owner),
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
				s.mockGithub.EXPECT().GetLatestRelease(owner, "framework").Return(mockRelease, nil).Once()
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
				s.mockGithub.EXPECT().GetLatestRelease(owner, "example").Return(mockRelease, nil).Once()
			},
			wantTag: "",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup()

			tag, err := s.release.getLatestTag(tt.repo)

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
					Run(func(msg string, opts console.SpinnerOption) {
						if opts.Action != nil {
							opts.Action()
						}
					}).Return(nil).Once()
				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf framework").Return("", nil).Once()
			},
			wantErr: nil,
		},
		{
			name: "real mode - spinner fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					Return(assert.AnError).Once()
				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf framework").Return("", nil).Once()
			},
			wantErr: assert.AnError,
		},
		{
			name: "real mode - command execution fails",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock the git command execution fails
				expectedCommand := `rm -rf framework && git clone --depth=1 git@github.com:goravel/framework.git && 
cd framework && git checkout master && git branch -D v1.16.x 2>/dev/null || true && git checkout -b v1.16.x && git push origin v1.16.x -f`
				s.mockProcess.EXPECT().Run(expectedCommand).Return("", assert.AnError).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf framework").Return("", nil).Once()
			},
			wantErr: fmt.Errorf("failed to push upgrade branch for framework: %w", assert.AnError),
		},
		{
			name: "real mode - output doesn't contain branch name",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock the git command execution succeeds but output doesn't contain branch name
				expectedCommand := `rm -rf framework && git clone --depth=1 git@github.com:goravel/framework.git && 
cd framework && git checkout master && git branch -D v1.16.x 2>/dev/null || true && git checkout -b v1.16.x && git push origin v1.16.x -f`
				s.mockProcess.EXPECT().Run(expectedCommand).Return("pushed to remote", nil).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf framework").Return("", nil).Once()
			},
			wantErr: fmt.Errorf("failed to push branch for framework: pushed to remote"),
		},
		{
			name: "real mode - successful push",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock the git command execution succeeds with output containing branch name
				expectedCommand := `rm -rf framework && git clone --depth=1 git@github.com:goravel/framework.git && 
cd framework && git checkout master && git branch -D v1.16.x 2>/dev/null || true && git checkout -b v1.16.x && git push origin v1.16.x -f`
				s.mockProcess.EXPECT().Run(expectedCommand).Return("pushed v1.16.x to origin", nil).Once()

				// Mock the cleanup call in defer
				s.mockProcess.EXPECT().Run("rm -rf framework").Return("", nil).Once()
			},
			wantErr: nil,
		},
		{
			name: "real mode - cleanup fails but doesn't affect result",
			real: true,
			setup: func() {
				s.mockContext.EXPECT().Spinner("Pushing branch v1.16.x for framework...", mock.AnythingOfType("console.SpinnerOption")).
					RunAndReturn(func(msg string, opts console.SpinnerOption) error {
						return opts.Action()
					}).Once()

				// Mock the git command execution succeeds
				expectedCommand := `rm -rf framework && git clone --depth=1 git@github.com:goravel/framework.git && 
cd framework && git checkout master && git branch -D v1.16.x 2>/dev/null || true && git checkout -b v1.16.x && git push origin v1.16.x -f`
				s.mockProcess.EXPECT().Run(expectedCommand).Return("pushed v1.16.x to origin", nil).Once()

				// Mock the cleanup call in defer fails
				s.mockProcess.EXPECT().Run("rm -rf framework").Return("", assert.AnError).Once()
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.release.real = tt.real
			tt.setup()

			err := s.release.pushBranch(s.mockContext, repo, branch)

			s.Equal(tt.wantErr, err)
		})
	}
}

// func (s *ReleaseTestSuite) Test_releaseAllPackages() {
// 	var (
// 		packageTag  = "v1.16.0"
// 		repoToNotes = map[string]*github.RepositoryReleaseNotes{
// 			"gin": {
// 				Name: "Release gin v1.16.0",
// 				Body: "## What's Changed\n* Feature A\n* Bug fix B",
// 			},
// 			"fiber": {
// 				Name: "Release fiber v1.16.0",
// 				Body: "## What's Changed\n* Feature C\n* Bug fix D",
// 			},
// 		}
// 	)

// 	tests := []struct {
// 		name        string
// 		repoToNotes map[string]*github.RepositoryReleaseNotes
// 		setup       func()
// 		wantErr     error
// 	}{
// 		{
// 			name:        "successful release of all packages",
// 			repoToNotes: repoToNotes,
// 			setup: func() {
// 				// Mock releaseRepo for gin package
// 				s.mockGithub.EXPECT().GetReleases(owner, "gin", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release gin v1.15.0"),
// 					},
// 				}, nil).Once()

// 				s.mockGithub.EXPECT().CreateRelease(owner, "gin", &github.RepositoryRelease{
// 					TagName:         convert.Pointer(packageTag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(repoToNotes["gin"].Name),
// 					Body:            convert.Pointer(repoToNotes["gin"].Body),
// 				}).Return(nil, nil).Once()

// 				s.mockContext.EXPECT().NewLine().Once()

// 				// Mock releaseRepo for fiber package
// 				s.mockGithub.EXPECT().GetReleases(owner, "fiber", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release fiber v1.15.0"),
// 					},
// 				}, nil).Once()

// 				s.mockGithub.EXPECT().CreateRelease(owner, "fiber", &github.RepositoryRelease{
// 					TagName:         convert.Pointer(packageTag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(repoToNotes["fiber"].Name),
// 					Body:            convert.Pointer(repoToNotes["fiber"].Body),
// 				}).Return(nil, nil).Once()

// 				s.mockContext.EXPECT().NewLine().Once()
// 			},
// 			wantErr: nil,
// 		},
// 		{
// 			name: "missing notes for gin package",
// 			repoToNotes: map[string]*github.RepositoryReleaseNotes{
// 				"fiber": {
// 					Name: "Release fiber v1.16.0",
// 					Body: "## What's Changed\n* Feature C\n* Bug fix D",
// 				},
// 			},
// 			setup:   func() {},
// 			wantErr: fmt.Errorf("notes for gin not found"),
// 		},
// 		{
// 			name:        "releaseRepo fails for gin package",
// 			repoToNotes: repoToNotes,
// 			setup: func() {
// 				// Mock releaseRepo fails for gin package
// 				s.mockGithub.EXPECT().GetReleases(owner, "gin", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return(nil, assert.AnError).Once()
// 			},
// 			wantErr: assert.AnError,
// 		},
// 		{
// 			name:        "releaseRepo fails for fiber package after gin succeeds",
// 			repoToNotes: repoToNotes,
// 			setup: func() {
// 				// Mock releaseRepo succeeds for gin package
// 				s.mockGithub.EXPECT().GetReleases(owner, "gin", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release gin v1.15.0"),
// 					},
// 				}, nil).Once()

// 				s.mockGithub.EXPECT().CreateRelease(owner, "gin", &github.RepositoryRelease{
// 					TagName:         convert.Pointer(packageTag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(repoToNotes["gin"].Name),
// 					Body:            convert.Pointer(repoToNotes["gin"].Body),
// 				}).Return(nil, nil).Once()

// 				s.mockContext.EXPECT().NewLine().Once()

// 				// Mock releaseRepo fails for fiber package
// 				s.mockGithub.EXPECT().GetReleases(owner, "fiber", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return(nil, assert.AnError).Once()
// 			},
// 			wantErr: assert.AnError,
// 		},
// 		{
// 			name:        "createRelease fails for gin package",
// 			repoToNotes: repoToNotes,
// 			setup: func() {
// 				// Mock isReleaseExist succeeds but createRelease fails for gin
// 				s.mockGithub.EXPECT().GetReleases(owner, "gin", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release gin v1.15.0"),
// 					},
// 				}, nil).Once()

// 				s.mockGithub.EXPECT().CreateRelease(owner, "gin", &github.RepositoryRelease{
// 					TagName:         convert.Pointer(packageTag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(repoToNotes["gin"].Name),
// 					Body:            convert.Pointer(repoToNotes["gin"].Body),
// 				}).Return(nil, assert.AnError).Once()
// 			},
// 			wantErr: fmt.Errorf("failed to create release: %w", assert.AnError),
// 		},
// 		{
// 			name:        "gin package already released",
// 			repoToNotes: repoToNotes,
// 			setup: func() {
// 				// Mock gin package already released
// 				s.mockGithub.EXPECT().GetReleases(owner, "gin", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release gin v1.15.0"),
// 					},
// 					{
// 						TagName: convert.Pointer("v1.16.0"),
// 						Name:    convert.Pointer("Release gin v1.16.0"),
// 					},
// 				}, nil).Once()

// 				// Mock releaseRepo for fiber package
// 				s.mockGithub.EXPECT().GetReleases(owner, "fiber", &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release fiber v1.15.0"),
// 					},
// 				}, nil).Once()

// 				s.mockGithub.EXPECT().CreateRelease(owner, "fiber", &github.RepositoryRelease{
// 					TagName:         convert.Pointer(packageTag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(repoToNotes["fiber"].Name),
// 					Body:            convert.Pointer(repoToNotes["fiber"].Body),
// 				}).Return(nil, nil).Once()

// 				s.mockContext.EXPECT().NewLine().Once()
// 			},
// 			wantErr: nil,
// 		},
// 	}

// 	for _, tt := range tests {
// 		s.Run(tt.name, func() {
// 			tt.setup()

// 			err := s.release.releaseAllPackages(s.mockContext, packageTag, tt.repoToNotes)

// 			s.Equal(tt.wantErr, err)
// 		})
// 	}
// }

// func (s *ReleaseTestSuite) Test_releaseRepo() {
// 	var (
// 		repo  = "framework"
// 		tag   = "v1.16.0"
// 		notes = &github.RepositoryReleaseNotes{
// 			Name: "Release v1.16.0",
// 			Body: "## What's Changed\n* Feature A\n* Bug fix B\n\n**Full Changelog**: https://github.com/goravel/framework/compare/v1.15.0...v1.16.0",
// 		}
// 	)

// 	tests := []struct {
// 		name    string
// 		setup   func()
// 		wantErr error
// 	}{
// 		{
// 			name: "successful release creation",
// 			setup: func() {
// 				// Mock isReleaseExist returns false (release doesn't exist)
// 				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release v1.15.0"),
// 					},
// 				}, nil).Once()

// 				// Mock createRelease succeeds
// 				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
// 					TagName:         convert.Pointer(tag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(notes.Name),
// 					Body:            convert.Pointer(notes.Body),
// 				}).Return(nil, nil).Once()

// 				// Mock releaseSuccess calls
// 				s.mockContext.EXPECT().NewLine().Once()
// 			},
// 			wantErr: nil,
// 		},
// 		{
// 			name: "release already exists",
// 			setup: func() {
// 				// Mock isReleaseExist returns true (release already exists)
// 				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release v1.15.0"),
// 					},
// 					{
// 						TagName: convert.Pointer("v1.16.0"),
// 						Name:    convert.Pointer("Release v1.16.0"),
// 					},
// 				}, nil).Once()
// 			},
// 			wantErr: nil,
// 		},
// 		{
// 			name: "isReleaseExist fails",
// 			setup: func() {
// 				// Mock isReleaseExist fails
// 				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return(nil, assert.AnError).Once()
// 			},
// 			wantErr: assert.AnError,
// 		},
// 		{
// 			name: "createRelease fails",
// 			setup: func() {
// 				// Mock isReleaseExist returns false (release doesn't exist)
// 				s.mockGithub.EXPECT().GetReleases(owner, repo, &github.ListOptions{
// 					Page:    1,
// 					PerPage: 10,
// 				}).Return([]*github.RepositoryRelease{
// 					{
// 						TagName: convert.Pointer("v1.15.0"),
// 						Name:    convert.Pointer("Release v1.15.0"),
// 					},
// 				}, nil).Once()

// 				// Mock createRelease fails
// 				s.mockGithub.EXPECT().CreateRelease(owner, repo, &github.RepositoryRelease{
// 					TagName:         convert.Pointer(tag),
// 					TargetCommitish: convert.Pointer("master"),
// 					Name:            convert.Pointer(notes.Name),
// 					Body:            convert.Pointer(notes.Body),
// 				}).Return(nil, assert.AnError).Once()
// 			},
// 			wantErr: fmt.Errorf("failed to create release: %w", assert.AnError),
// 		},
// 	}

// 	for _, tt := range tests {
// 		s.Run(tt.name, func() {
// 			tt.setup()

// 			err := s.release.releaseRepo(s.mockContext, repo, tag, notes)

// 			s.Equal(tt.wantErr, err)
// 		})
// 	}
// }
