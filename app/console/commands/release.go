package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v81/github"
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/support/color"
	"github.com/goravel/framework/support/convert"

	"goravel/app/facades"
	"goravel/app/services"
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
	"installer",
	"goravel-lite",
}

type ReleaseInformation struct {
	// The current tag in master, only goravel/framework and goravel/installer has this tag currently.
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
	ctx    console.Context
	github services.Github
	real   bool
}

func NewRelease(ctx console.Context) *Release {
	release := &Release{
		ctx:    ctx,
		github: services.NewGithubImpl(context.Background()),
	}

	return release
}

func (r *Release) Major() error {
	if r.ctx.OptionBool("refresh") {
		if err := r.refreshGoProxy(); err != nil {
			r.ctx.Error(err.Error())
			return nil
		}
	}

	r.real = r.ctx.OptionBool("real")
	frameworkTag := r.ctx.ArgumentString("tag")

	var frameworkBranch string
	if strings.HasSuffix(frameworkTag, ".0") {
		frameworkBranch = strings.TrimSuffix(frameworkTag, ".0") + ".x"
	}

	packageTag := frameworkTag
	packageBranch := frameworkBranch

	if !r.ctx.Confirm("Did you test in sub-packages?") {
		subPackages := append(packages, "example")
		for _, pkg := range subPackages {
			// frameworkBranch := "master"
			// if fb := r.ctx.Option("framework-branch"); fb != "" {
			// 	frameworkBranch = fb
			// }
			if err := r.testInSubPackage(pkg, "master", "master"); err != nil {
				return err
			}
		}
	}

	frameworkReleaseInfo, err := r.getFrameworkReleaseInformation(frameworkTag)
	if err != nil {
		return err
	}

	packagesReleaseInfo, err := r.getPackagesReleaseInformation(packageTag)
	if err != nil {
		return err
	}

	if !r.ctx.Confirm("Did you confirm the release information?") {
		releaseInfos := append(packagesReleaseInfo, frameworkReleaseInfo)
		if err := r.confirmReleaseInformations(releaseInfos); err != nil {
			return err
		}
	}

	if err := r.releaseRepo(frameworkReleaseInfo); err != nil {
		return err
	}

	packageToPR, err := r.createUpgradePRsForPackages(frameworkTag)
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(packageToPR); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	for _, releaseInfo := range packagesReleaseInfo {
		if err := r.releaseRepo(releaseInfo); err != nil {
			return err
		}

		if releaseInfo.repo == "goravel-lite" {
			if frameworkBranch != "" {
				if err := r.pushBranch(releaseInfo.repo, frameworkBranch); err != nil {
					return err
				}
			}
		} else {
			if packageBranch != "" {
				if err := r.pushBranch(releaseInfo.repo, packageBranch); err != nil {
					return err
				}
			}
		}
	}

	examplePR, err := r.createUpgradePRForExample(frameworkTag, []string{
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
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(map[string]*github.PullRequest{
		"example": examplePR,
	}); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	if frameworkBranch != "" {
		if err := r.pushBranch("framework", frameworkBranch); err != nil {
			return err
		}

		if err := r.pushBranch("example", frameworkBranch); err != nil {
			return err
		}
	}

	r.releaseMajorSuccess(frameworkTag, packageTag)

	return nil
}

func (r *Release) Patch() error {
	r.real = r.ctx.OptionBool("real")
	frameworkTag := r.ctx.ArgumentString("tag")
	frameworkBranch := r.getBranchFromTag("framework", frameworkTag)
	packageBranch := frameworkBranch

	if !r.ctx.Confirm("Did you test in sub-packages?") {
		subPackages := append(packages, "example")
		for _, pkg := range subPackages {
			if err := r.testInSubPackage(pkg, frameworkBranch, packageBranch); err != nil {
				return err
			}
		}
	}

	goravelReleaseInfo, err := r.getPackageReleaseInformation("goravel-lite", frameworkTag)
	if err != nil {
		return err
	}

	frameworkReleaseInfo, err := r.getFrameworkReleaseInformation(frameworkTag)
	if err != nil {
		return err
	}

	if !r.ctx.Confirm("Did you confirm the release information?") {
		releaseInfos := []*ReleaseInformation{goravelReleaseInfo, frameworkReleaseInfo}
		if err := r.confirmReleaseInformations(releaseInfos); err != nil {
			return err
		}
	}

	if err := r.releaseRepo(frameworkReleaseInfo); err != nil {
		return err
	}

	if !r.ctx.Confirm("Do you want to upgrade goravel and example?") {
		r.releasePatchSuccess(frameworkTag)

		return nil
	}

	examplePR, err := r.createUpgradePRForExample(frameworkTag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
	})
	if err != nil {
		return err
	}

	litePR, err := r.createUpgradePRForLite(frameworkTag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
	})
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(map[string]*github.PullRequest{
		"example":      examplePR,
		"goravel-lite": litePR,
	}); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	if err := r.releaseRepo(goravelReleaseInfo); err != nil {
		return err
	}

	r.releasePatchSuccess(frameworkTag)

	return nil
}

func (r *Release) Preview() error {
	frameworkTag := r.ctx.ArgumentString("tag")
	packageTag := frameworkTag

	var releaseInfos []*ReleaseInformation

	if frameworkTag != "" {
		liteReleaseInfo, err := r.getPackageReleaseInformation("goravel-lite", frameworkTag)
		if err != nil {
			return err
		}

		frameworkReleaseInfo, err := r.getFrameworkReleaseInformation(frameworkTag)
		if err != nil {
			return err
		}

		releaseInfos = append(releaseInfos, liteReleaseInfo, frameworkReleaseInfo)
	}

	if packageTag != "" {
		packagesReleaseInfo, err := r.getPackagesReleaseInformation(packageTag)
		if err != nil {
			return err
		}

		releaseInfos = append(releaseInfos, packagesReleaseInfo...)
	}

	for _, releaseInfo := range releaseInfos {
		r.divider()
		color.Yellow().Println(fmt.Sprintf("Please check %s/%s information:", owner, releaseInfo.repo))
		r.ctx.NewLine()

		color.Black().Print("The latest tag is:             ")
		color.Red().Println(releaseInfo.latestTag)

		color.Black().Print("The tag to release is:         ")
		color.Red().Println(releaseInfo.tag)

		if releaseInfo.currentTag != "" {
			color.Black().Print("The current tag in code is:    ")
			color.Red().Println(releaseInfo.currentTag)

			if releaseInfo.currentTag != releaseInfo.tag {
				r.ctx.NewLine()
				color.Red().Println("The current tag is not the same as the tag to release")
			}
		}

		r.ctx.NewLine()
		color.Black().Println(releaseInfo.notes.Name)
		color.Black().Println(releaseInfo.notes.Body)

		r.ctx.NewLine()
	}

	return nil
}

func (r *Release) checkPRsMergeStatus(repoToPR map[string]*github.PullRequest) error {
	for pkg, pr := range repoToPR {
		if pr == nil {
			color.Black().Println(fmt.Sprintf("%-10s: no need to upgrade", pkg))
			continue
		}

		color.Black().Println(fmt.Sprintf("%-10s: %s", pkg, *pr.HTMLURL+"/files"))
	}

	for {
		choice, err := r.ctx.Choice("Check PRs merge status?", []console.Choice{
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

				if err := r.ctx.Spinner(fmt.Sprintf("Checking %s/%s merge status...", owner, repo), console.SpinnerOption{
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
				}); err != nil {
					return err
				}
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

func (r *Release) confirmReleaseInformations(releaseInfos []*ReleaseInformation) error {
	for _, releaseInfo := range releaseInfos {
		r.divider()
		color.Yellow().Println(fmt.Sprintf("Please check %s/%s information:", owner, releaseInfo.repo))
		r.ctx.NewLine()

		color.Black().Print("The latest tag is:             ")
		color.Red().Println(releaseInfo.latestTag)

		color.Black().Print("The tag to release is:         ")
		color.Red().Println(releaseInfo.tag)

		if releaseInfo.currentTag != "" {
			color.Black().Print("The current tag in code is:    ")
			color.Red().Println(releaseInfo.currentTag)

			if releaseInfo.currentTag != releaseInfo.tag {
				r.ctx.NewLine()
				color.Red().Println("The current tag is not the same as the tag to release")
			}
		}

		r.ctx.NewLine()
		color.Black().Println(releaseInfo.notes.Name)
		color.Black().Println(releaseInfo.notes.Body)

		r.ctx.NewLine()
		if !r.ctx.Confirm(fmt.Sprintf("%s/%s confirmed?", owner, releaseInfo.repo)) {
			return fmt.Errorf("%s/%s not confirmed", owner, releaseInfo.repo)
		}
	}

	return nil
}

func (r *Release) createRelease(repo, tag string, notes *github.RepositoryReleaseNotes) error {
	if !r.real {
		color.Yellow().Println("Preview mode, skip creating release for " + repo)
		return nil
	}

	_, err := r.github.CreateRelease(owner, repo, &github.RepositoryRelease{
		TagName:         convert.Pointer(tag),
		TargetCommitish: convert.Pointer(r.getBranchFromTag(repo, tag)),
		Name:            convert.Pointer(notes.Name),
		Body:            convert.Pointer(notes.Body),
	})

	return err
}

func (r *Release) createUpgradePRForExample(frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	repo := "example"

	return r.createUpgradePR(repo, r.getBranchFromTag(repo, frameworkTag), frameworkTag, dependencies)
}

func (r *Release) createUpgradePRForLite(frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	repo := "goravel-lite"

	return r.createUpgradePR(repo, r.getBranchFromTag(repo, frameworkTag), frameworkTag, dependencies)
}

func (r *Release) createUpgradePRsForPackages(frameworkTag string) (map[string]*github.PullRequest, error) {
	packageToPR := make(map[string]*github.PullRequest)

	for _, pkg := range packages {
		pr, err := r.createUpgradePR(pkg, "master", frameworkTag, []string{
			fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkTag),
		})
		if err != nil {
			return nil, err
		}

		packageToPR[pkg] = pr
	}

	return packageToPR, nil
}

func (r *Release) createUpgradePR(repo, baseBranch, frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	defer func() {
		_ = facades.Process().Run(fmt.Sprintf("rm -rf %s", repo))
	}()

	var pr *github.PullRequest

	dependencyCommands := strings.Join(dependencies, " && ")

	if err := r.ctx.Spinner(fmt.Sprintf("Creating upgrade PR for %s...", repo), console.SpinnerOption{
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
cd %s && git checkout %s && git branch -D %s 2>/dev/null || true && git checkout -b %s &&
%s && go mod tidy`, repo, owner, repo, repo, baseBranch, upgradeBranch, upgradeBranch, dependencyCommands)

			if res := facades.Process().Run(commandToCloneAndMod); res.Failed() {
				return fmt.Errorf("failed to clone repo and mod for %s: %w", repo, res.Error())
			}

			// Check status
			commandToCheckStatus := fmt.Sprintf(`cd %s && git status`, repo)
			res := facades.Process().Run(commandToCheckStatus)
			if res.Failed() {
				return fmt.Errorf("failed to check status for %s: %w", repo, res.Error())
			}
			if strings.Contains(res.Output(), "nothing to commit, working tree clean") {
				color.Yellow().Println(fmt.Sprintf("%s/%s is already up to date", owner, repo))
				return nil
			}

			// Push upgrade branch
			commandToPush := fmt.Sprintf(`cd %s && git add . && git commit -m "%s" && git push origin %s -f`, repo, prTitle, upgradeBranch)
			res = facades.Process().Run(commandToPush)
			if res.Failed() {
				return fmt.Errorf("failed to push upgrade branch for %s: %w", repo, res.Error())
			}
			if !strings.Contains(res.Output(), prTitle) {
				return fmt.Errorf("failed to push upgrade branch for %s: %s", repo, res.Output())
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
					Base:  convert.Pointer(baseBranch),
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

func (r *Release) divider() {
	r.ctx.TwoColumnDetail("", "", '-')
}

func (r *Release) getBranchFromTag(repo, tag string) string {
	tagArr := strings.Split(tag, ".")
	branch := strings.Join(append(tagArr[:2], "x"), ".")

	exist, err := r.github.CheckBranchExists(owner, repo, branch)
	if err != nil {
		panic(fmt.Errorf("failed to check branch %s exist for %s/%s: %w", branch, owner, repo, err))
	}

	if !exist {
		branch = "master"
	}

	return branch
}

func (r *Release) getFrameworkReleaseInformation(tag string) (*ReleaseInformation, error) {
	var releaseInformation *ReleaseInformation
	repo := "framework"

	if err := r.ctx.Spinner(fmt.Sprintf("Getting framework release information for %s...", tag), console.SpinnerOption{
		Action: func() error {
			latestTag, err := r.getLatestTag(repo, tag)
			if err != nil {
				return err
			}

			branch := r.getBranchFromTag(repo, tag)
			currentTag, err := r.getFrameworkCurrentTag(branch)
			if err != nil {
				return err
			}

			notes, err := r.generateReleaseNotes(repo, tag, latestTag, branch)
			if err != nil {
				return err
			}

			releaseInformation = &ReleaseInformation{
				notes:      notes,
				tag:        tag,
				currentTag: currentTag,
				latestTag:  latestTag,
				repo:       repo,
			}

			return nil
		},
	}); err != nil {
		return nil, err
	}

	return releaseInformation, nil
}

func (r *Release) getPackagesReleaseInformation(tag string) ([]*ReleaseInformation, error) {
	releaseInfos := make([]*ReleaseInformation, 0)

	for _, pkg := range packages {
		releaseInfo, err := r.getPackageReleaseInformation(pkg, tag)
		if err != nil {
			return nil, err
		}

		if pkg == "installer" {
			currentTag, err := r.getInstallerCurrentTag()
			if err != nil {
				return nil, err
			}

			releaseInfo.currentTag = currentTag
		}

		releaseInfos = append(releaseInfos, releaseInfo)
	}

	return releaseInfos, nil
}

func (r *Release) getPackageReleaseInformation(repo string, tag string) (*ReleaseInformation, error) {
	var releaseInformation *ReleaseInformation

	if err := r.ctx.Spinner(fmt.Sprintf("Getting %s release information for %s...", repo, tag), console.SpinnerOption{
		Action: func() error {
			latestTag, err := r.getLatestTag(repo, tag)
			if err != nil {
				return err
			}

			branch := r.getBranchFromTag(repo, tag)
			notes, err := r.generateReleaseNotes(repo, tag, latestTag, branch)
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

func (r *Release) getFrameworkCurrentTag(branch string) (string, error) {
	return r.getCurrentTag("framework", fmt.Sprintf("https://raw.githubusercontent.com/goravel/framework/refs/heads/%s/support/constant.go", branch))
}

func (r *Release) getInstallerCurrentTag() (string, error) {
	return r.getCurrentTag("installer", "https://raw.githubusercontent.com/goravel/installer/refs/heads/master/support/constant.go")
}

func (r *Release) getCurrentTag(repo, url string) (string, error) {
	response, err := facades.Http().Get(url)
	if err != nil {
		return "", err
	}

	body, err := response.Body()
	if err != nil {
		return "", err
	}

	// Extract version from body using regex
	versionRegex := regexp.MustCompile(`Version\s*.*?=\s*"([^"]+)"`)
	matches := versionRegex.FindStringSubmatch(body)
	var currentVersion string
	if len(matches) > 1 {
		currentVersion = matches[1]
	} else {
		return "", fmt.Errorf("could not extract goravel/%s version from code", repo)
	}

	return currentVersion, nil
}

func (r *Release) generateReleaseNotes(repo, tag, previousTag, branch string) (*github.RepositoryReleaseNotes, error) {
	notes, err := r.github.GenerateReleaseNotes(owner, repo, &github.GenerateNotesOptions{
		TagName:         tag,
		PreviousTagName: convert.Pointer(previousTag),
		TargetCommitish: convert.Pointer(branch),
	})
	if err != nil {
		return nil, err
	}
	if notes == nil {
		return nil, fmt.Errorf("failed to generate release notes, notes is nil")
	}

	return notes, nil
}

func (r *Release) getLatestTag(repo, tag string) (string, error) {
	latestRelease, err := r.github.GetLatestRelease(owner, repo, tag)
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

func (r *Release) pushBranch(repo, branch string) error {
	defer func() {
		_ = facades.Process().Run(fmt.Sprintf("rm -rf %s", repo))
	}()

	if err := r.ctx.Spinner(fmt.Sprintf("Pushing branch %s for %s...", branch, repo), console.SpinnerOption{
		Action: func() error {
			if !r.real {
				color.Yellow().Println(fmt.Sprintf("Preview mode, skip pushing branch %s for %s", branch, repo))
				return nil
			}

			command := fmt.Sprintf(`rm -rf %s && git clone git@github.com:%s/%s.git && 
cd %s && git checkout master && git branch -D %s 2>/dev/null || true && git checkout -b %s && git push origin %s -f`,
				repo, owner, repo, repo, branch, branch, branch)
			if res := facades.Process().Run(command); res.Failed() {
				return fmt.Errorf("failed to push upgrade branch for %s: %w", repo, res.Error())
			}

			color.Green().Println(fmt.Sprintf("[%s/%s] Push %s branch success!", owner, repo, branch))

			return nil
		},
	}); err != nil {
		return err
	}

	return nil
}

func (r *Release) refreshGoProxy() error {
	allPackages := append(packages, "framework", "example")
	var links []string

	for _, pkg := range allPackages {
		links = append(links, fmt.Sprintf("curl https://proxy.golang.org/github.com/goravel/%s/@v/master.info", pkg))
	}

	command := strings.Join(links, " && ")
	if res := facades.Process().Quietly().WithSpinner("Refreshing Go Proxy...").Run(command); res.Failed() {
		return fmt.Errorf("Failed to refresh Go module proxy cache: %s", res.Error().Error())
	}

	color.Green().Println("Refreshed Go module proxy cache successfully")

	return nil
}

func (r *Release) releaseRepo(releaseInfo *ReleaseInformation) error {
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

	r.releaseSuccess(releaseInfo.repo, releaseInfo.tag)

	return nil
}

func (r *Release) releaseMajorSuccess(frameworkTag, packageTag string) {
	r.ctx.NewLine()
	color.Green().Println(fmt.Sprintf("Release goravel/framework %s and sub-packages %s success!", frameworkTag, packageTag))
	color.Yellow().Println("The rest jobs:")
	color.Black().Println("1. Merge the goravel/goravel PR that is created by action based on goravel/goravel-lite: https://github.com/goravel/goravel/pulls")
	color.Black().Println(fmt.Sprintf("2. Create the goravel/goravel %s branch: https://github.com/goravel/goravel/branches", frameworkTag))
	color.Black().Println(fmt.Sprintf("3. Create the goravel/goravel %s release: https://github.com/goravel/goravel/releases", frameworkTag))
	color.Black().Println(fmt.Sprintf("4. Set goravel/goravel %s as default branch: https://github.com/goravel/goravel/settings", frameworkTag))
	color.Black().Println(fmt.Sprintf("5. Set goravel/goravel-lite %s as default branch: https://github.com/goravel/goravel-lite/settings", frameworkTag))
	color.Black().Println(fmt.Sprintf("6. Set goravel/example %s as default branch: https://github.com/goravel/example/settings", frameworkTag))
	color.Black().Println("7. Install the new version via goravel/installer and test the project works fine")
	color.Black().Println("8. Modify the support policy: https://www.goravel.dev/getting-started/releases.html#support-policy")
}

func (r *Release) releasePatchSuccess(frameworkTag string) {
	r.ctx.NewLine()
	color.Green().Println(fmt.Sprintf("Release goravel/framework %s success!", frameworkTag))
}

func (r *Release) releaseSuccess(repo, tagName string) {
	color.Green().Println(fmt.Sprintf("[%s/%s] Release %s success!", owner, repo, tagName))
	color.Green().Println(fmt.Sprintf("Release link: https://github.com/%s/%s/releases/tag/%s", owner, repo, tagName))
}

func (r *Release) testInSubPackage(pkg, frameworkBranch, packageBranch string) error {
	defer func() {
		_ = facades.Process().Run(fmt.Sprintf("rm -rf %s", pkg))
	}()

	packages := fmt.Sprintf("go get github.com/goravel/framework@%s && ", frameworkBranch)
	if pkg == "example" && frameworkBranch == "master" {
		packages = `go get github.com/goravel/gin@master && 
				go get github.com/goravel/fiber@master && 
				go get github.com/goravel/s3@master && 
				go get github.com/goravel/oss@master && 
				go get github.com/goravel/cos@master && 
				go get github.com/goravel/minio@master && 
				go get github.com/goravel/postgres@master && 
				go get github.com/goravel/mysql@master && 
				go get github.com/goravel/sqlserver@master && 
				go get github.com/goravel/sqlite@master && 
				go get github.com/goravel/redis@master && `
	}

	initCommand := fmt.Sprintf(`rm -rf %s && git clone git@github.com:goravel/%s.git && 
				cd %s && git checkout %s && %s go mod tidy && cp .env.example .env 2>/dev/null || true && go test ./...`, pkg, pkg, pkg, packageBranch, packages)
	if res := facades.Process().Run(initCommand); res.Failed() {
		return fmt.Errorf("failed to test in %s: %w", pkg, res.Error())
	}

	color.Green().Println(fmt.Sprintf("Testing in %s success!", pkg))

	return nil
}
