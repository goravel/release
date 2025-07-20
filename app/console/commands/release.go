package commands

import (
	"context"
	"fmt"
	"goravel/app/services"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/google/go-github/v73/github"
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/contracts/http/client"
	"github.com/goravel/framework/facades"
	"github.com/goravel/framework/support/color"
	"github.com/goravel/framework/support/convert"
)

const owner = "goravel"

var packages = []string{
	"gin",
	"fiber",
	"s3",
	"oss",
	"cos",
	"minio",
	"postgres",
	"mysql",
	"sqlserver",
	"sqlite",
	"redis",
}

type ReleaseInformation struct {
	// The current tag in master, goravel/framework has this tag for now.
	currentTag string
	// The latest tag actually
	latestTag string
	// The release notes
	notes *github.RepositoryReleaseNotes
	// The repo name
	repo string
	// The tag to release
	tag string
}

type Release struct {
	github  services.Github
	process services.Process
	http    client.Request
	real    bool
}

func NewRelease() *Release {
	release := &Release{
		github:  services.NewGithubImpl(context.Background()),
		process: services.NewProcessImpl(),
		http:    facades.Http(),
	}

	return release
}

// Signature The name and signature of the console command.
func (r *Release) Signature() string {
	return "release"
}

// Description The console command description.
func (r *Release) Description() string {
	return "Release Goravel packages"
}

// Extend The console command extend.
func (r *Release) Extend() command.Extend {
	return command.Extend{
		Category: "app",
		Flags: []command.Flag{
			&command.StringFlag{
				Name:    "framework",
				Aliases: []string{"f"},
				Usage:   "Release framework tag",
			},
			&command.StringFlag{
				Name:    "packages",
				Aliases: []string{"p"},
				Usage:   "Release packages tag",
			},
			&command.BoolFlag{
				Name:    "real",
				Aliases: []string{"r"},
				Usage:   "Real release",
			},
		},
	}
}

// Handle Execute the console command.
func (r *Release) Handle(ctx console.Context) error {
	r.real = ctx.OptionBool("real")

	if ctx.Argument(0) == "major" {
		return r.releaseMajor(ctx)
	}

	return nil
}

func (r *Release) checkPRsMergeStatus(ctx console.Context, repoToPR map[string]*github.PullRequest) error {
	for pkg, pr := range repoToPR {
		if pr == nil {
			color.Black().Println(fmt.Sprintf("%-10s: no need to upgrade", pkg))
			continue
		}

		color.Black().Println(fmt.Sprintf("%-10s: %s", pkg, *pr.HTMLURL+"/files"))
	}

	for {
		choice, err := ctx.Choice("Check PRs merge status?", []console.Choice{
			{
				Key:   "Check",
				Value: "Check",
			},
		})
		if err != nil {
			return err
		}

		if choice == "Check" {
			var notMerged []string
			for repo, pr := range repoToPR {
				if pr == nil {
					continue
				}

				ctx.Spinner(fmt.Sprintf("Checking %s/%s merge status...", owner, repo), console.SpinnerOption{
					Action: func() error {
						merged, err := r.checkPRMergeStatus(repo, pr)
						if err != nil {
							return err
						}

						if merged {
							color.Green().Println(fmt.Sprintf("%s/%s merged", owner, repo))

							repoToPR[repo] = nil
						} else {
							notMerged = append(notMerged, fmt.Sprintf("%s/%s", owner, repo))
						}

						return nil
					},
				})
			}

			if len(notMerged) == 0 {
				return nil
			} else {
				color.Yellow().Println(fmt.Sprintf("Not merged PRs: %s", strings.Join(notMerged, ", ")))
			}
		}
	}
}

func (r *Release) checkPRMergeStatus(repo string, pr *github.PullRequest) (bool, error) {
	if pr == nil {
		return true, nil
	}
	if !r.real {
		color.Yellow().Println(fmt.Sprintf("Preview mode, skip checking merge status for %s/%s", owner, repo))
		return true, nil
	}

	pr, err := r.github.GetPullRequest(owner, repo, *pr.Number)
	if err != nil {
		return false, err
	}
	if pr.Merged != nil && *pr.Merged {
		return true, nil
	}

	return false, nil
}

func (r *Release) confirmReleaseInformations(ctx console.Context, releaseInfos []*ReleaseInformation) error {
	for _, releaseInfo := range releaseInfos {
		r.divider(ctx)
		color.Yellow().Println(fmt.Sprintf("Please check %s/%s information:", owner, releaseInfo.repo))
		ctx.NewLine()

		color.Black().Print("The latest tag is:     ")
		color.Red().Println(releaseInfo.latestTag)

		color.Black().Print("The tag to release is: ")
		color.Red().Println(releaseInfo.tag)

		if releaseInfo.currentTag != "" {
			color.Black().Print("The current tag is:    ")
			color.Red().Println(releaseInfo.currentTag)

			if releaseInfo.currentTag != releaseInfo.tag {
				ctx.NewLine()
				color.Red().Println("The current tag is not the same as the tag to release")
			}
		}

		ctx.NewLine()
		color.Black().Println(releaseInfo.notes.Name)
		color.Black().Println(releaseInfo.notes.Body)

		ctx.NewLine()
		if !ctx.Confirm(fmt.Sprintf("%s/%s confirmed?", owner, releaseInfo.repo)) {
			return fmt.Errorf("%s/%s not confirmed", owner, releaseInfo.repo)
		}
	}

	return nil
}

func (r *Release) createRelease(repo, tagName string, notes *github.RepositoryReleaseNotes) error {
	if !r.real {
		color.Yellow().Println("Preview mode, skip creating release for " + repo)
		return nil
	}

	_, err := r.github.CreateRelease(owner, repo, &github.RepositoryRelease{
		TagName:         convert.Pointer(tagName),
		TargetCommitish: convert.Pointer("master"),
		Name:            convert.Pointer(notes.Name),
		Body:            convert.Pointer(notes.Body),
	})

	return err
}

func (r *Release) createUpgradePRForExample(ctx console.Context, frameworkTag, packageTag string) (*github.PullRequest, error) {
	return r.createUpgradePR(ctx, "example", frameworkTag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
		fmt.Sprintf("go get github.com/goravel/gin@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/fiber@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/s3@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/oss@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/cos@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/minio@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/postgres@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/mysql@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/sqlserver@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/sqlite@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/redis@%s", packageTag),
	})
}

func (r *Release) createUpgradePRForExampleClient(ctx console.Context, frameworkTag, packageTag string) (*github.PullRequest, error) {
	return r.createUpgradePR(ctx, "example-client", frameworkTag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
		fmt.Sprintf("go get github.com/goravel/gin@%s", packageTag),
	})
}

func (r *Release) createUpgradePRForGoravel(ctx console.Context, frameworkTag, packageTag string) (*github.PullRequest, error) {
	return r.createUpgradePR(ctx, "goravel", frameworkTag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
		fmt.Sprintf("go get github.com/goravel/gin@%s", packageTag),
		fmt.Sprintf("go get github.com/goravel/postgres@%s", packageTag),
	})
}

func (r *Release) createUpgradePRsForPackages(ctx console.Context, frameworkTag string) (map[string]*github.PullRequest, error) {
	packageToPR := make(map[string]*github.PullRequest)

	for _, pkg := range packages {
		pr, err := r.createUpgradePR(ctx, pkg, frameworkTag, []string{
			fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
		})
		if err != nil {
			return nil, err
		}

		packageToPR[pkg] = pr
	}

	return packageToPR, nil
}

func (r *Release) createUpgradePR(ctx console.Context, repo, frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	defer func() {
		r.process.Run(fmt.Sprintf("rm -rf %s", repo))
	}()

	var pr *github.PullRequest

	dependencyCommands := strings.Join(dependencies, " && ")

	if err := ctx.Spinner(fmt.Sprintf("Creating upgrade PR for %s...", repo), console.SpinnerOption{
		Action: func() error {
			// Clone repo and mod
			upgradeBranch := "auto-upgrade/" + frameworkTag
			prTitle := fmt.Sprintf("chore: Upgrade framework to %s (auto)", frameworkTag)

			if !r.real {
				color.Yellow().Println(fmt.Sprintf("Preview mode, skip creating upgrade PR for %s", repo))
				pr = &github.PullRequest{
					Title:   convert.Pointer(prTitle),
					HTMLURL: convert.Pointer(fmt.Sprintf("https://github.com/%s/%s/pull/%s", owner, repo, upgradeBranch)),
				}

				return nil
			}

			commandToCloneAndMod := fmt.Sprintf(`rm -rf %s && git clone git@github.com:%s/%s.git &&
cd %s && git checkout master && git branch -D %s 2>/dev/null || true && git checkout -b %s &&
%s && go mod tidy`, repo, owner, repo, repo, upgradeBranch, upgradeBranch, dependencyCommands)
			if _, err := r.executeCommand(commandToCloneAndMod); err != nil {
				return fmt.Errorf("failed to clone repo and mod for %s: %w", repo, err)
			}

			// Check status
			commandToCheckStatus := fmt.Sprintf(`cd %s && git status`, repo)
			output, err := r.executeCommand(commandToCheckStatus)
			if err != nil {
				return fmt.Errorf("failed to check status for %s: %w", repo, err)
			}
			if strings.Contains(output, "nothing to commit, working tree clean") {
				color.Yellow().Println(fmt.Sprintf("%s/%s is already up to date", owner, repo))
				return nil
			}

			// Push upgrade branch
			commandToPush := fmt.Sprintf(`cd %s && git add . && git commit -m "%s" && git push origin %s -f`, repo, prTitle, upgradeBranch)
			output, err = r.executeCommand(commandToPush)
			if err != nil {
				return fmt.Errorf("failed to push upgrade branch for %s: %w", repo, err)
			}
			if !strings.Contains(output, prTitle) {
				return fmt.Errorf("failed to push upgrade branch for %s: %s", repo, output)
			}

			// List PRs
			prs, err := r.github.GetPullRequests(owner, repo, &github.PullRequestListOptions{
				State: "open",
			})
			if err != nil {
				return err
			}

			// Find existing PR
			for _, p := range prs {
				if *p.Title == prTitle {
					pr = p
					break
				}
			}

			// Create PR if not found
			if pr == nil {
				pr, err = r.github.CreatePullRequest(owner, repo, &github.NewPullRequest{
					Title: convert.Pointer(fmt.Sprintf("chore: Upgrade framework to %s (auto)", frameworkTag)),
					Head:  convert.Pointer(upgradeBranch),
					Base:  convert.Pointer("master"),
				})
				if err != nil {
					return err
				}
			}

			return nil
		},
	}); err != nil {
		return nil, err
	}

	return pr, nil
}

func (r *Release) divider(ctx console.Context) {
	ctx.TwoColumnDetail("", "", '-')
}

func (r *Release) executeCommand(command string) (string, error) {
	color.Gray().Println("")
	color.Gray().Println("Executing command:")
	color.Gray().Println(command)

	return r.process.Run(command)
}

func (r *Release) getFrameworkReleaseInfomration(ctx console.Context, tag string) (*ReleaseInformation, error) {
	var releaseInformation *ReleaseInformation

	if err := ctx.Spinner(fmt.Sprintf("Getting framework release infomration for %s...", tag), console.SpinnerOption{
		Action: func() error {
			latestTag, err := r.getLatestTag("framework")
			if err != nil {
				return err
			}

			currentTag, err := r.getFrameworkCurrentTag()
			if err != nil {
				return err
			}

			notes, err := r.generateReleaseNotes("framework", tag, latestTag)
			if err != nil {
				return err
			}

			releaseInformation = &ReleaseInformation{
				notes:      notes,
				tag:        tag,
				currentTag: currentTag,
				latestTag:  latestTag,
				repo:       "framework",
			}

			return nil
		},
	}); err != nil {
		return nil, err
	}

	return releaseInformation, nil
}

func (r *Release) getPackagesReleaseInfomration(ctx console.Context, tag string) ([]*ReleaseInformation, error) {
	releaseInfos := make([]*ReleaseInformation, 0)

	for _, pkg := range packages {
		releaseInfo, err := r.getPackageReleaseInfomration(ctx, pkg, tag)
		if err != nil {
			return nil, err
		}

		releaseInfos = append(releaseInfos, releaseInfo)
	}

	return releaseInfos, nil
}

func (r *Release) getPackageReleaseInfomration(ctx console.Context, repo string, tag string) (*ReleaseInformation, error) {
	var releaseInformation *ReleaseInformation

	if err := ctx.Spinner(fmt.Sprintf("Getting %s release infomration for %s...", repo, tag), console.SpinnerOption{
		Action: func() error {
			latestTag, err := r.getLatestTag(repo)
			if err != nil {
				return err
			}

			notes, err := r.generateReleaseNotes(repo, tag, latestTag)
			if err != nil {
				return err
			}

			releaseInformation = &ReleaseInformation{
				notes:     notes,
				tag:       tag,
				latestTag: latestTag,
				repo:      repo,
			}

			return nil
		},
	}); err != nil {
		return nil, err
	}

	return releaseInformation, nil
}

func (r *Release) getFrameworkCurrentTag() (string, error) {
	response, err := r.http.Get("https://raw.githubusercontent.com/goravel/framework/refs/heads/master/support/constant.go")
	if err != nil {
		return "", err
	}

	body, err := response.Body()
	if err != nil {
		return "", err
	}

	// Extract version from body using regex
	versionRegex := regexp.MustCompile(`Version\s*=\s*"([^"]+)"`)
	matches := versionRegex.FindStringSubmatch(body)
	var currentVersion string
	if len(matches) > 1 {
		currentVersion = matches[1]
	} else {
		return "", fmt.Errorf("could not extract goravel/framework version from code")
	}

	return currentVersion, nil
}

func (r *Release) generateReleaseNotes(repo, tagName, previousTagName string) (*github.RepositoryReleaseNotes, error) {
	notes, err := r.github.GenerateReleaseNotes(owner, repo, &github.GenerateNotesOptions{
		TagName:         tagName,
		PreviousTagName: convert.Pointer(previousTagName),
		TargetCommitish: convert.Pointer("master"),
	})
	if err != nil {
		return nil, err
	}
	if notes == nil {
		return nil, fmt.Errorf("failed to generate release notes, notes is nil")
	}

	return notes, nil
}

func (r *Release) getLatestTag(repo string) (string, error) {
	latestRelease, err := r.github.GetLatestRelease(owner, repo)
	if err != nil {
		return "", err
	}

	if latestRelease == nil {
		return "", nil
	}

	if latestRelease.TagName == nil {
		return "", fmt.Errorf("latest release tag name is nil for %s/%s", owner, repo)
	}

	return *latestRelease.TagName, nil
}

func (r *Release) isReleaseExist(repo string, tag string) (bool, error) {
	releases, err := r.github.GetReleases(owner, repo, &github.ListOptions{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		return false, err
	}

	for _, release := range releases {
		if release.TagName != nil && *release.TagName == tag {
			return true, nil
		}
	}

	return false, nil
}

func (r *Release) pushBranch(ctx console.Context, repo, branch string) error {
	defer func() {
		r.process.Run(fmt.Sprintf("rm -rf %s", repo))
	}()

	if err := ctx.Spinner(fmt.Sprintf("Pushing branch %s for %s...", branch, repo), console.SpinnerOption{
		Action: func() error {
			if !r.real {
				color.Yellow().Println(fmt.Sprintf("Preview mode, skip pushing branch %s for %s", branch, repo))
				return nil
			}

			command := fmt.Sprintf(`rm -rf %s && git clone git@github.com:%s/%s.git && 
cd %s && git checkout master && git branch -D %s 2>/dev/null || true && git checkout -b %s && git push origin %s -f`,
				repo, owner, repo, repo, branch, branch, branch)
			_, err := r.executeCommand(command)
			if err != nil {
				return fmt.Errorf("failed to push upgrade branch for %s: %w", repo, err)
			}

			return nil
		},
	}); err != nil {
		return err
	}

	return nil
}

func (r *Release) releaseRepo(ctx console.Context, releaseInfo *ReleaseInformation) error {
	isExist, err := r.isReleaseExist(releaseInfo.repo, releaseInfo.tag)
	if err != nil {
		return err
	}
	if isExist {
		color.Yellow().Println(fmt.Sprintf("%s/%s %s has already been released", owner, releaseInfo.repo, releaseInfo.tag))
		return nil
	}

	if err := r.createRelease(releaseInfo.repo, releaseInfo.tag, releaseInfo.notes); err != nil {
		return fmt.Errorf("failed to create release: %w", err)
	}

	r.releaseSuccess(ctx, releaseInfo.repo, releaseInfo.tag)

	return nil
}

func (r *Release) releaseMajor(ctx console.Context) error {
	frameworkTag := ctx.Option("framework")
	packageTag := ctx.Option("packages")

	if frameworkTag == "" || packageTag == "" {
		return fmt.Errorf("framework and packages tags are required, please use --framework and --packages to release")
	}

	if !ctx.Confirm("Did you test in example?") {
		if err := r.testInExample(ctx); err != nil {
			return err
		}
	}

	frameworkReleaseInfo, err := r.getFrameworkReleaseInfomration(ctx, frameworkTag)
	if err != nil {
		return err
	}

	packagesReleaseInfo, err := r.getPackagesReleaseInfomration(ctx, packageTag)
	if err != nil {
		return err
	}

	if !ctx.Confirm("Did you confirm the release infomration?") {
		releaseInfos := append(packagesReleaseInfo, frameworkReleaseInfo)
		if err := r.confirmReleaseInformations(ctx, releaseInfos); err != nil {
			return err
		}
	}

	if err := r.releaseRepo(ctx, frameworkReleaseInfo); err != nil {
		return err
	}

	packageToPR, err := r.createUpgradePRsForPackages(ctx, frameworkTag)
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(ctx, packageToPR); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	for _, releaseInfo := range packagesReleaseInfo {
		if err := r.releaseRepo(ctx, releaseInfo); err != nil {
			return err
		}
	}

	examplePR, err := r.createUpgradePRForExample(ctx, frameworkTag, packageTag)
	if err != nil {
		return err
	}

	exampleClientPR, err := r.createUpgradePRForExampleClient(ctx, frameworkTag, packageTag)
	if err != nil {
		return err
	}

	goravelPR, err := r.createUpgradePRForGoravel(ctx, frameworkTag, packageTag)
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(ctx, map[string]*github.PullRequest{
		"example":        examplePR,
		"example-client": exampleClientPR,
		"goravel":        goravelPR,
	}); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	if strings.HasSuffix(frameworkTag, ".0") {
		frameworkMajorTag := strings.TrimSuffix(frameworkTag, ".0") + ".x"
		if err := r.pushBranch(ctx, "framework", frameworkMajorTag); err != nil {
			return err
		}

		if err := r.pushBranch(ctx, "goravel", frameworkMajorTag); err != nil {
			return err
		}

		if err := r.pushBranch(ctx, "example", frameworkMajorTag); err != nil {
			return err
		}
	}

	r.releaseMajorSuccess(ctx, frameworkTag, packageTag)

	return nil
}

func (r *Release) releaseMajorSuccess(ctx console.Context, frameworkTag, packageTag string) {
	ctx.NewLine()
	color.Green().Println(fmt.Sprintf("Release goravel/framework %s and sub-packages %s success!", frameworkTag, packageTag))
	color.Yellow().Println("The rest jobs:")
	color.Black().Println(fmt.Sprintf("1. Set goravel/goravel %s as default branch: https://github.com/goravel/goravel/settings", frameworkTag))
	color.Black().Println(fmt.Sprintf("2. Set goravel/example %s as default branch: https://github.com/goravel/example/settings", frameworkTag))
	color.Black().Println("3. Install the new version via goravel/installer and test the project works fine")
	color.Black().Println("4. Merge the upgrade document PR (this step will be automated in the future): https://github.com/goravel/docs/pulls")
}

func (r *Release) releaseSuccess(ctx console.Context, repo, tagName string) {
	color.Green().Println(fmt.Sprintf("[%s/%s] release %s success!", owner, repo, tagName))
	color.Green().Println(fmt.Sprintf("Release link: https://github.com/%s/%s/releases/tag/%s", owner, repo, tagName))
}

func (r *Release) testInExample(ctx console.Context) error {
	defer func() {
		r.process.Run("rm -rf example")
	}()

	if err := ctx.Spinner("Testing in example...", console.SpinnerOption{
		Action: func() error {
			commands := `rm -rf example && git clone git@github.com:goravel/example.git && 
				cd example && git checkout master &&
				go get github.com/goravel/framework@master && 
				go get github.com/goravel/gin@master && 
				go get github.com/goravel/fiber@master && 
				go get github.com/goravel/s3@master && 
				go get github.com/goravel/oss@master && 
				go get github.com/goravel/cos@master && 
				go get github.com/goravel/minio@master && 
				go get github.com/goravel/postgres@master && 
				go get github.com/goravel/mysql@master && 
				go get github.com/goravel/sqlserver@master && 
				go get github.com/goravel/sqlite@master && 
				go get github.com/goravel/redis@master && 
				go mod tidy && go test ./...`

			cmd := exec.Command("sh", "-c", commands)

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Fatal(err)
			}

			stderr, err := cmd.StderrPipe()
			if err != nil {
				log.Fatal(err)
			}

			if err := cmd.Start(); err != nil {
				return err
			}

			var mu sync.Mutex

			go func() {
				if commandErr := asyncLog(stdout); commandErr != nil {
					mu.Lock()
					err = commandErr
					mu.Unlock()
				}
			}()
			go func() {
				if commandErr := asyncLog(stderr); commandErr != nil {
					mu.Lock()
					err = commandErr
					mu.Unlock()
				}
			}()

			if commandErr := cmd.Wait(); commandErr != nil {
				return commandErr
			}

			return err
		},
	}); err != nil {
		return fmt.Errorf("failed to test in example: %w", err)
	}

	color.Green().Println("Testing in example success!")

	return nil
}

func asyncLog(reader io.ReadCloser) error {
	fail := false
	cache := ""
	buf := make([]byte, 100)
	for {
		num, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				break
			}
			return err
		}
		if num > 0 {
			oByte := string(buf[:num])
			if strings.Contains(oByte, "--- FAIL") {
				fail = true
			}

			oSlice := strings.Split(oByte, "\n")
			line := strings.Join(oSlice[:len(oSlice)-1], "\n")
			color.Black().Println(fmt.Sprintf("%s%s", cache, line))
			cache = oSlice[len(oSlice)-1]
		}
	}

	if fail {
		return fmt.Errorf("test failed")
	}

	return nil
}
