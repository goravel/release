package commands

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v82/github"
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
		ctx: ctx,
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
	r.github = services.NewGithubImpl(r.real)
	tag := r.ctx.ArgumentString("tag")

	var branch string
	if strings.HasSuffix(tag, ".0") {
		branch = strings.TrimSuffix(tag, ".0") + ".x"
	}

	if err := r.testInSubPackages("master"); err != nil {
		return err
	}

	packagesReleaseInfo, err := r.getPackagesReleaseInformation(tag)
	if err != nil {
		return err
	}

	if !r.ctx.Confirm("Did you confirm the release information?") {
		if err := r.confirmReleaseInformation(packagesReleaseInfo); err != nil {
			return err
		}
	}

	if err := r.releaseFramework(branch, packagesReleaseInfo["framework"]); err != nil {
		return err
	}

	if err := r.releasePackages(packagesReleaseInfo, tag, branch); err != nil {
		return err
	}

	if err := r.releaseExample(tag, branch); err != nil {
		return err
	}

	if err := r.releaseGoravel(tag, branch); err != nil {
		return err
	}

	r.releaseMajorSuccess(tag)

	return nil
}

func (r *Release) Patch() error {
	r.real = r.ctx.OptionBool("real")
	r.github = services.NewGithubImpl(r.real)
	tag := r.ctx.ArgumentString("tag")
	branch := r.getBranchFromTag("framework", tag)

	if err := r.testInSubPackages(branch); err != nil {
		return err
	}

	liteReleaseInfo, err := r.getPackageReleaseInformation("goravel-lite", tag)
	if err != nil {
		return err
	}

	frameworkReleaseInfo, err := r.getPackageReleaseInformation("framework", tag)
	if err != nil {
		return err
	}

	if !r.ctx.Confirm("Did you confirm the release information?") {
		releaseInfos := map[string]*ReleaseInformation{
			"goravel-lite": liteReleaseInfo,
			"framework":    frameworkReleaseInfo,
		}
		if err := r.confirmReleaseInformation(releaseInfos); err != nil {
			return err
		}
	}

	if err := r.releaseRepo(frameworkReleaseInfo); err != nil {
		return err
	}

	examplePR, err := r.createUpgradePRForExample(tag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", tag),
	})
	if err != nil {
		return err
	}

	litePR, err := r.createUpgradePRForLite(tag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", tag),
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

	if err := r.releaseRepo(liteReleaseInfo); err != nil {
		return err
	}

	if err := r.releaseGoravel(tag, ""); err != nil {
		return err
	}

	r.releasePatchSuccess(tag)

	return nil
}

func (r *Release) Preview() error {
	r.github = services.NewGithubImpl(true)
	tag := r.ctx.ArgumentString("tag")
	containPackages := r.ctx.OptionBool("packages")

	var (
		releaseInfos map[string]*ReleaseInformation
		err          error
	)

	if containPackages {
		releaseInfos, err = r.getPackagesReleaseInformation(tag)
		if err != nil {
			return err
		}
	} else {
		liteReleaseInfo, err := r.getPackageReleaseInformation("goravel-lite", tag)
		if err != nil {
			return err
		}

		frameworkReleaseInfo, err := r.getPackageReleaseInformation("framework", tag)
		if err != nil {
			return err
		}

		releaseInfos = map[string]*ReleaseInformation{
			"goravel-lite": liteReleaseInfo,
			"framework":    frameworkReleaseInfo,
		}
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

func (r *Release) checkGoravelAutoUpgradePRMergeStatus(ctx console.Context) bool {
	return ctx.Confirm("Is Goravel auto upgrade PR merged? https://github.com/goravel/goravel/pulls")
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

	pr, err := r.github.GetPullRequest(owner, repo, *pr.Number)
	if err != nil {
		return false, err
	}
	if pr.Merged != nil && *pr.Merged {
		return true, nil
	}

	return false, nil
}

func (r *Release) confirmReleaseInformation(pkgToReleaseInfo map[string]*ReleaseInformation) error {
	for _, releaseInfo := range pkgToReleaseInfo {
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
					Number:  convert.Pointer(1),
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
					Title: convert.Pointer(prTitle),
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

func (r *Release) getPackagesReleaseInformation(tag string) (map[string]*ReleaseInformation, error) {
	pkgToReleaseInfo := make(map[string]*ReleaseInformation, 0)
	allPackages := append(packages, "framework")

	for _, pkg := range allPackages {
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

		if pkg == "framework" {
			currentTag, err := r.getFrameworkCurrentTag(r.getBranchFromTag("framework", tag))
			if err != nil {
				return nil, err
			}

			releaseInfo.currentTag = currentTag
		}

		pkgToReleaseInfo[pkg] = releaseInfo
	}

	return pkgToReleaseInfo, nil
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
		return fmt.Errorf("failed to refresh Go module proxy cache: %s", res.Error().Error())
	}

	color.Green().Println("Refreshed Go module proxy cache successfully")

	return nil
}

func (r *Release) releaseExample(tag, branch string) error {
	repo := "example"
	examplePR, err := r.createUpgradePRForExample(tag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", tag),
		fmt.Sprintf("go get github.com/goravel/gin@%s", tag),
		fmt.Sprintf("go get github.com/goravel/fiber@%s", tag),
		fmt.Sprintf("go get github.com/goravel/s3@%s", tag),
		fmt.Sprintf("go get github.com/goravel/oss@%s", tag),
		fmt.Sprintf("go get github.com/goravel/cos@%s", tag),
		fmt.Sprintf("go get github.com/goravel/minio@%s", tag),
		fmt.Sprintf("go get github.com/goravel/postgres@%s", tag),
		fmt.Sprintf("go get github.com/goravel/mysql@%s", tag),
		fmt.Sprintf("go get github.com/goravel/sqlserver@%s", tag),
		fmt.Sprintf("go get github.com/goravel/sqlite@%s", tag),
		fmt.Sprintf("go get github.com/goravel/redis@%s", tag),
	})
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(map[string]*github.PullRequest{
		repo: examplePR,
	}); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	if branch != "" {
		if err := r.pushBranch("example", branch); err != nil {
			return err
		}

		if err := r.setDefaultBranch("example", branch); err != nil {
			return err
		}
	}

	return nil
}

func (r *Release) releaseFramework(branch string, releaseInfo *ReleaseInformation) error {
	if err := r.releaseRepo(releaseInfo); err != nil {
		return err
	}

	if branch != "" {
		if err := r.pushBranch("framework", branch); err != nil {
			return err
		}
	}

	return nil
}

func (r *Release) releaseGoravel(tag, branch string) error {
	repo := "goravel"

	if !r.checkGoravelAutoUpgradePRMergeStatus(r.ctx) {
		return fmt.Errorf("failed to check goravel auto upgrade PR merge status")
	}

	goravelReleaseInfo, err := r.getPackageReleaseInformation(repo, tag)
	if err != nil {
		return err
	}
	if err := r.confirmReleaseInformation(map[string]*ReleaseInformation{
		repo: goravelReleaseInfo,
	}); err != nil {
		return err
	}
	if err := r.releaseRepo(goravelReleaseInfo); err != nil {
		return err
	}

	if branch != "" {
		if err := r.pushBranch(repo, branch); err != nil {
			return err
		}
		if err := r.setDefaultBranch(repo, branch); err != nil {
			return err
		}
	}

	return nil
}

func (r *Release) releasePackages(packagesReleaseInfo map[string]*ReleaseInformation, tag, branch string) error {
	packageToPR, err := r.createUpgradePRsForPackages(tag)
	if err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(packageToPR); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	for pkg, releaseInfo := range packagesReleaseInfo {
		// Skip framework, already released
		if pkg == "framework" {
			continue
		}

		if err := r.releaseRepo(releaseInfo); err != nil {
			return err
		}

		if releaseInfo.repo == "goravel-lite" {
			if branch != "" {
				if err := r.pushBranch(releaseInfo.repo, branch); err != nil {
					return err
				}
				if err := r.setDefaultBranch(releaseInfo.repo, branch); err != nil {
					return err
				}
			}
		} else {
			if branch != "" {
				if err := r.pushBranch(releaseInfo.repo, branch); err != nil {
					return err
				}
			}
		}
	}

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

func (r *Release) releaseMajorSuccess(tag string) {
	r.ctx.NewLine()
	color.Green().Println(fmt.Sprintf("Release %s success!", tag))
	color.Yellow().Println("The rest jobs:")
	color.Black().Println("1. Install the new version via goravel/installer and test the project works fine")
	color.Black().Println("2. Modify the support policy: https://www.goravel.dev/prologue/releases.html#support-policy")
}

func (r *Release) releasePatchSuccess(frameworkTag string) {
	r.ctx.NewLine()
	color.Green().Println(fmt.Sprintf("Release goravel/framework %s success!", frameworkTag))
}

func (r *Release) releaseSuccess(repo, tagName string) {
	color.Green().Println(fmt.Sprintf("[%s/%s] Release %s success!", owner, repo, tagName))
	color.Green().Println(fmt.Sprintf("Release link: https://github.com/%s/%s/releases/tag/%s", owner, repo, tagName))
}

func (r *Release) setDefaultBranch(repo, branch string) error {
	if err := r.github.SetDefaultBranch(owner, repo, branch); err != nil {
		return fmt.Errorf("failed to set default branch %s for %s/%s: %w", branch, owner, repo, err)
	}

	color.Green().Println(fmt.Sprintf("[%s/%s] Set default branch to %s success!", owner, repo, branch))

	return nil
}

func (r *Release) testInSubPackages(branch string) error {
	if !r.ctx.Confirm("Did you test in sub-packages?") {
		packagesWithExample := append(packages, "example")
		for _, pkg := range packagesWithExample {
			if err := r.testInSubPackage(pkg, branch); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Release) testInSubPackage(pkg, branch string) error {
	defer func() {
		_ = facades.Process().Run(fmt.Sprintf("rm -rf %s", pkg))
	}()

	packages := fmt.Sprintf("go get github.com/goravel/framework@%s && ", branch)
	if pkg == "example" && branch == "master" {
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
				cd %s && git checkout %s && %s go mod tidy && cp .env.example .env 2>/dev/null || true && go test ./...`, pkg, pkg, pkg, branch, packages)
	if res := facades.Process().Run(initCommand); res.Failed() {
		return fmt.Errorf("failed to test in %s: %w", pkg, res.Error())
	}

	color.Green().Println(fmt.Sprintf("Testing in %s success!", pkg))

	return nil
}
