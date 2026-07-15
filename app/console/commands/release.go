package commands

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v89/github"
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
	"openai",
	"anthropic",
	"gemini",
	"installer",
	"goravel-lite",
}

var exampleDeps = []string{
	"framework",
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
	"openai",
	"anthropic",
	"gemini",
}

func parseExtraPackages(packagesStr string) map[string]string {
	if packagesStr == "" {
		return nil
	}

	result := make(map[string]string)
	for _, pkg := range strings.Split(packagesStr, ",") {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}
		parts := strings.SplitN(pkg, "@", 2)
		if len(parts) != 2 {
			continue
		}
		name, version := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if name == "" || version == "" {
			continue
		}
		result[name] = version
	}

	return result
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
	testRef := "master"

	var branch string
	if strings.HasSuffix(tag, ".0") {
		branch = strings.TrimSuffix(tag, ".0") + ".x"
	}

	packagesReleaseInfo, err := r.getReleases(tag, append(packages, "framework"))
	if err != nil {
		return err
	}

	if !r.ctx.Confirm("Did you confirm the release information?") {
		if err := r.confirmReleaseInformation(packagesReleaseInfo); err != nil {
			return err
		}
	}

	examplePR, err := r.createUpgradePRForExample(tag, r.exampleDependencyCommands(testRef))
	if err != nil {
		return err
	}

	// if err := r.triggerExampleAICI(examplePR); err != nil {
	// 	return err
	// }

	driverPRs, err := r.createUpgradePRsForPackages(testRef, tag)
	if err != nil {
		return err
	}

	allPRs := make(map[string]*github.PullRequest)
	for k, v := range driverPRs {
		allPRs[k] = v
	}
	allPRs["example"] = examplePR

	if err := r.checkCIPRsStatus(allPRs); err != nil {
		return err
	}

	if err := r.releaseFramework(branch, packagesReleaseInfo["framework"]); err != nil {
		return err
	}

	if err := r.updateAllPackageDependencies(tag); err != nil {
		return err
	}

	if err := r.checkPRsMergeStatus(driverPRs); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	if err := r.releasePackages(packagesReleaseInfo, tag, branch); err != nil {
		return err
	}

	if err := r.updatePRDependencies("example", tag, r.exampleDependencyCommands(tag)); err != nil {
		return err
	}

	if err := r.releaseExample(examplePR, branch); err != nil {
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
	testRef := r.getBranchFromTag("framework", tag)

	extraPackages := parseExtraPackages(r.ctx.Option("packages"))

	repos := []string{"goravel-lite", "framework"}
	releaseInfos, err := r.getReleases(tag, repos)
	if err != nil {
		return err
	}
	liteReleaseInfo := releaseInfos["goravel-lite"]
	frameworkReleaseInfo := releaseInfos["framework"]

	extraReleaseInfos := make(map[string]*ReleaseInformation)
	for pkg, pkgTag := range extraPackages {
		info, err := r.getRelease(pkg, pkgTag)
		if err != nil {
			return err
		}
		extraReleaseInfos[pkg] = info
	}

	confirmMap := map[string]*ReleaseInformation{
		"goravel-lite": liteReleaseInfo,
		"framework":    frameworkReleaseInfo,
	}
	for pkg, info := range extraReleaseInfos {
		confirmMap[pkg] = info
	}

	if !r.ctx.Confirm("Did you confirm the release information?") {
		if err := r.confirmReleaseInformation(confirmMap); err != nil {
			return err
		}
	}

	examplePR, err := r.createUpgradePRForExample(tag, r.buildExampleDeps(testRef, extraPackages, false))
	if err != nil {
		return err
	}

	litePR, err := r.createUpgradePRForLite(tag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", testRef),
	})
	if err != nil {
		return err
	}

	extraPackagePRs := make(map[string]*github.PullRequest)
	for pkg := range extraPackages {
		pr, err := r.createUpgradePR(pkg, r.getBranchFromTag(pkg, tag), tag, []string{
			fmt.Sprintf("go get github.com/goravel/framework@%s", testRef),
		})
		if err != nil {
			return err
		}
		extraPackagePRs[pkg] = pr
	}

	ciPRs := map[string]*github.PullRequest{
		"example":      examplePR,
		"goravel-lite": litePR,
	}
	for pkg, pr := range extraPackagePRs {
		ciPRs[pkg] = pr
	}

	if err := r.checkCIPRsStatus(ciPRs); err != nil {
		return err
	}

	if err := r.releaseRepo(frameworkReleaseInfo); err != nil {
		return err
	}

	for pkg := range extraPackages {
		if err := r.updatePRDependencies(pkg, tag, []string{
			fmt.Sprintf("go get github.com/goravel/framework@%s", tag),
		}); err != nil {
			return err
		}
	}

	if err := r.checkPRsMergeStatus(extraPackagePRs); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status for extra packages: %w", err)
	}

	for pkg := range extraPackages {
		if err := r.releaseRepo(extraReleaseInfos[pkg]); err != nil {
			return err
		}
	}

	if err := r.updatePRDependencies("example", tag, r.buildExampleDeps(tag, extraPackages, true)); err != nil {
		return err
	}

	if err := r.updatePRDependencies("goravel-lite", tag, []string{
		fmt.Sprintf("go get github.com/goravel/framework@%s", tag),
	}); err != nil {
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

	r.releasePatchSuccess(tag, extraPackages)

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
		releaseInfos, err = r.getReleases(tag, append(packages, "framework"))
		if err != nil {
			return err
		}
	} else {
		releaseInfos, err = r.getReleases(tag, []string{"goravel-lite", "framework"})
		if err != nil {
			return err
		}
	}

	for _, releaseInfo := range releaseInfos {
		r.printReleaseInformation(releaseInfo)
	}

	return nil
}

func (r *Release) checkCIPRsStatus(repoToPR map[string]*github.PullRequest) error {
	for repo, pr := range repoToPR {
		if pr == nil {
			color.Black().Println(fmt.Sprintf("%-10s: no need to upgrade", repo))
			continue
		}

		color.Black().Println(fmt.Sprintf("%-10s: %s", repo, *pr.HTMLURL))
	}

	passed := make(map[string]bool)

	for {
		choice, err := r.ctx.Choice("Check CI status?", []console.Choice{
			{
				Key:   "Check",
				Value: "Check",
			},
		})
		if err != nil {
			return err
		}

		if choice == "Check" {
			var notPassed []string
			var mu sync.Mutex
			var wg sync.WaitGroup
			errCh := make(chan error, len(repoToPR))

			if err := r.ctx.Spinner("Checking CI status for all repos...", console.SpinnerOption{
				Action: func() error {
					for repo, pr := range repoToPR {
						if pr == nil || passed[repo] {
							continue
						}

						wg.Add(1)
						go func(repo string, pr *github.PullRequest) {
							defer wg.Done()

							latestPR, err := r.github.GetPullRequest(owner, repo, pr.GetNumber())
							if err != nil {
								errCh <- err
								return
							}

							mu.Lock()
							repoToPR[repo] = latestPR
							mu.Unlock()

							if latestPR.Head == nil || latestPR.Head.SHA == nil {
								color.Yellow().Println(fmt.Sprintf("%s/%s PR has no head SHA, skipping CI check", owner, repo))
								return
							}

							checkRuns, err := r.github.GetCheckRunsForRef(owner, repo, *latestPR.Head.SHA)
							if err != nil {
								errCh <- err
								return
							}

							mu.Lock()
							switch {
							case anyCheckRunFailed(checkRuns):
								color.Red().Println(fmt.Sprintf("%s/%s CI failed", owner, repo))
								notPassed = append(notPassed, fmt.Sprintf("%s/%s", owner, repo))
							case allCheckRunsCompleted(checkRuns):
								color.Green().Println(fmt.Sprintf("%s/%s CI passed", owner, repo))
								passed[repo] = true
							default:
								color.Yellow().Println(fmt.Sprintf("%s/%s CI pending", owner, repo))
								notPassed = append(notPassed, fmt.Sprintf("%s/%s", owner, repo))
							}
							mu.Unlock()
						}(repo, pr)
					}

					wg.Wait()
					close(errCh)

					if err := <-errCh; err != nil {
						return err
					}

					return nil
				},
			}); err != nil {
				return err
			}

			if len(notPassed) == 0 {
				return nil
			} else {
				color.Yellow().Println(fmt.Sprintf("Not passed CI: %s", strings.Join(notPassed, ", ")))
			}
		}
	}
}

func allCheckRunsCompleted(checkRuns []*github.CheckRun) bool {
	if len(checkRuns) == 0 {
		return false
	}

	for _, cr := range checkRuns {
		if cr.GetStatus() != "completed" {
			return false
		}
	}

	return true
}

func anyCheckRunFailed(checkRuns []*github.CheckRun) bool {
	for _, cr := range checkRuns {
		conclusion := cr.GetConclusion()
		if conclusion == "failure" || conclusion == "cancelled" || conclusion == "timed_out" || conclusion == "action_required" {
			return true
		}
	}

	return false
}

func (r *Release) updatePRDependencies(repo, tag string, dependencies []string) error {
	defer func() {
		_ = facades.Process().Run(fmt.Sprintf("rm -rf %s", repo))
	}()

	prTitle := r.upgradePRTitle(tag)

	if mergedPR, err := r.findMergedUpgradePR(repo, prTitle); err != nil {
		return err
	} else if mergedPR != nil {
		color.Yellow().Println(fmt.Sprintf("[%s/%s] Upgrade PR already merged, skip update: %s", owner, repo, *mergedPR.HTMLURL))
		return nil
	}

	if err := r.ctx.Spinner(fmt.Sprintf("Updating upgrade PR for %s...", repo), console.SpinnerOption{
		Action: func() error {
			if !r.real {
				color.Yellow().Println(fmt.Sprintf("Preview mode, skip updating upgrade PR for %s", repo))
				return nil
			}

			dependencyCommands := strings.Join(dependencies, " && ")

			commandToCloneAndMod := fmt.Sprintf(`export GONOSUMDB=github.com/goravel && rm -rf %s && git clone git@github.com:%s/%s.git &&
cd %s && git checkout auto-upgrade/%s && %s && go mod tidy`,
				repo, owner, repo, repo, tag, dependencyCommands)

			if res := facades.Process().Run(commandToCloneAndMod); res.Failed() {
				return fmt.Errorf("failed to update upgrade PR for %s: %w", repo, res.Error())
			}

			commandToPush := fmt.Sprintf(`cd %s && git add . && git commit -m "%s" && git push origin auto-upgrade/%s -f`,
				repo, prTitle, tag)
			if res := facades.Process().Run(commandToPush); res.Failed() {
				return fmt.Errorf("failed to push updated upgrade PR for %s: %w", repo, res.Error())
			}

			color.Green().Println(fmt.Sprintf("[%s/%s] Update upgrade PR for %s success!", owner, repo, repo))

			return nil
		},
	}); err != nil {
		return err
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

		if pkg == "goravel-lite" {
			color.Black().Println(fmt.Sprintf("%-10s: %s", pkg, *pr.HTMLURL+"/files (This should be merged last)"))
		} else {
			color.Black().Println(fmt.Sprintf("%-10s: %s", pkg, *pr.HTMLURL+"/files"))
		}
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
		r.printReleaseInformation(releaseInfo)

		if !r.ctx.Confirm(fmt.Sprintf("%s/%s confirmed?", owner, releaseInfo.repo)) {
			return fmt.Errorf("%s/%s not confirmed", owner, releaseInfo.repo)
		}
	}

	return nil
}

func (r *Release) createRelease(repo, tag string, notes *github.RepositoryReleaseNotes) error {
	_, err := r.github.CreateRelease(owner, repo, &github.CreateReleaseRequest{
		TagName:         tag,
		TargetCommitish: convert.Pointer(r.getBranchFromTag(repo, tag)),
		Name:            convert.Pointer(notes.Name),
		Body:            convert.Pointer(notes.Body),
	})

	return err
}

func (r *Release) exampleDependencyCommands(ref string) []string {
	var cmds []string
	for _, pkg := range exampleDeps {
		cmds = append(cmds, fmt.Sprintf("go get github.com/goravel/%s@%s", pkg, ref))

		if pkg == "framework" {
			cmds = append(cmds, "go mod edit -replace github.com/goravel/framework=github.com/goravel/framework@$(go list -m -f '{{.Version}}' github.com/goravel/framework)")
		}
	}

	return cmds
}

func (r *Release) buildExampleDeps(frameworkRef string, extraPackages map[string]string, useExtraTags bool) []string {
	cmds := []string{fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkRef)}

	pkgNames := sortedKeys(extraPackages)
	for _, pkg := range pkgNames {
		ref := frameworkRef
		if useExtraTags {
			ref = extraPackages[pkg]
		}
		cmds = append(cmds, fmt.Sprintf("go get github.com/goravel/%s@%s", pkg, ref))
	}
	cmds = append(cmds, "go mod edit -replace github.com/goravel/framework=github.com/goravel/framework@$(go list -m -f '{{.Version}}' github.com/goravel/framework)")
	return cmds
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (r *Release) createUpgradePRForExample(frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	repo := "example"

	return r.createUpgradePR(repo, r.getBranchFromTag(repo, frameworkTag), frameworkTag, dependencies)
}

func (r *Release) triggerExampleAICI(examplePR *github.PullRequest) error {
	repo := "example"
	workflowFileName := "test-ai.yml"

	if examplePR == nil || examplePR.Head == nil || examplePR.Head.Ref == nil {
		color.Yellow().Println(fmt.Sprintf(
			"Example PR has no head ref, skipping %s workflow trigger", workflowFileName,
		))
		return nil
	}

	ref := *examplePR.Head.Ref

	return r.ctx.Spinner(
		fmt.Sprintf("Triggering %s workflow on goravel/example@%s...", workflowFileName, ref),
		console.SpinnerOption{
			Action: func() error {
				return r.github.CreateWorkflowDispatchEvent(owner, repo, workflowFileName, ref)
			},
		},
	)
}

func (r *Release) createUpgradePRForLite(frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	repo := "goravel-lite"

	return r.createUpgradePR(repo, r.getBranchFromTag(repo, frameworkTag), frameworkTag, dependencies)
}

func (r *Release) createUpgradePRsForPackages(frameworkRef, frameworkTag string) (map[string]*github.PullRequest, error) {
	packageToPR := make(map[string]*github.PullRequest)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(packages))

	for _, pkg := range packages {
		wg.Add(1)
		go func(pkg string) {
			defer wg.Done()
			pr, err := r.createUpgradePR(pkg, "master", frameworkTag, []string{
				fmt.Sprintf("go get github.com/goravel/framework@%s", frameworkRef),
			})
			if err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			packageToPR[pkg] = pr
			mu.Unlock()
		}(pkg)
	}

	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}

	return packageToPR, nil
}

func (r *Release) updateAllPackageDependencies(tag string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(packages))

	if err := r.ctx.Spinner("Updating upgrade PRs for all packages...", console.SpinnerOption{
		Action: func() error {
			for _, pkg := range packages {
				wg.Add(1)
				go func(pkg string) {
					defer wg.Done()
					if err := r.updatePRDependencies(pkg, tag, []string{
						fmt.Sprintf("go get github.com/goravel/framework@%s", tag),
					}); err != nil {
						errCh <- err
					}
				}(pkg)
			}

			wg.Wait()
			close(errCh)

			if err := <-errCh; err != nil {
				return err
			}

			return nil
		},
	}); err != nil {
		return err
	}

	return nil
}

func (r *Release) upgradePRTitle(tag string) string {
	return fmt.Sprintf("chore: Upgrade framework to %s (auto)", tag)
}

func (r *Release) findExistingUpgradePR(repo, title string) (*github.PullRequest, error) {
	prs, err := r.github.GetPullRequests(owner, repo, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return nil, err
	}

	for _, p := range prs {
		if p.Title != nil && *p.Title == title {
			return p, nil
		}
	}

	return nil, nil
}

func (r *Release) findMergedUpgradePR(repo, title string) (*github.PullRequest, error) {
	prs, err := r.github.GetPullRequests(owner, repo, &github.PullRequestListOptions{
		State: "closed",
	})
	if err != nil {
		return nil, err
	}

	for _, p := range prs {
		if p.MergedAt != nil && p.Title != nil && *p.Title == title {
			return p, nil
		}
	}

	return nil, nil
}

func (r *Release) createUpgradePR(repo, baseBranch, frameworkTag string, dependencies []string) (*github.PullRequest, error) {
	defer func() {
		_ = facades.Process().Run(fmt.Sprintf("rm -rf %s", repo))
	}()

	upgradeBranch := "auto-upgrade/" + frameworkTag
	prTitle := r.upgradePRTitle(frameworkTag)

	if existingPR, err := r.findExistingUpgradePR(repo, prTitle); err != nil {
		return nil, err
	} else if existingPR != nil {
		color.Green().Println(fmt.Sprintf("[%s/%s] Upgrade PR already exists: %s", owner, repo, *existingPR.HTMLURL))
		return existingPR, nil
	}

	if mergedPR, err := r.findMergedUpgradePR(repo, prTitle); err != nil {
		return nil, err
	} else if mergedPR != nil {
		color.Yellow().Println(fmt.Sprintf("[%s/%s] Upgrade PR already merged: %s", owner, repo, *mergedPR.HTMLURL))
		return nil, nil
	}

	dependencyCommands := strings.Join(dependencies, " && ")

	var pr *github.PullRequest

	if err := r.ctx.Spinner(fmt.Sprintf("Creating upgrade PR for %s...", repo), console.SpinnerOption{
		Action: func() error {
			if !r.real {
				color.Yellow().Println(fmt.Sprintf("Preview mode, skip creating upgrade PR for %s", repo))
				pr = &github.PullRequest{
					Title:   convert.Pointer(prTitle),
					HTMLURL: convert.Pointer(fmt.Sprintf("https://github.com/%s/%s/pull/%s", owner, repo, upgradeBranch)),
					Number:  convert.Pointer(1),
					Head: &github.PullRequestBranch{
						SHA: convert.Pointer("fake-sha"),
					},
				}

				return nil
			}

			commandToCloneAndMod := fmt.Sprintf(`export GONOSUMDB=github.com/goravel && rm -rf %s && git clone git@github.com:%s/%s.git &&
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

			var createErr error
			pr, createErr = r.github.CreatePullRequest(owner, repo, &github.NewPullRequest{
				Title: convert.Pointer(prTitle),
				Head:  convert.Pointer(upgradeBranch),
				Base:  convert.Pointer(baseBranch),
			})
			if createErr != nil {
				return createErr
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

func (r *Release) getReleases(tag string, repos []string) (map[string]*ReleaseInformation, error) {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs []error
		info = make(map[string]*ReleaseInformation)
	)

	if err := r.ctx.Spinner(fmt.Sprintf("Fetching release information for %d packages...", len(repos)), console.SpinnerOption{
		Action: func() error {
			for _, repo := range repos {
				wg.Add(1)
				go func(repo string) {
					defer wg.Done()

					releaseInfo, err := r.getRelease(repo, tag)

					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						errs = append(errs, err)
						return
					}
					info[repo] = releaseInfo
				}(repo)
			}

			wg.Wait()

			if len(errs) > 0 {
				return errors.Join(errs...)
			}

			return nil
		},
	}); err != nil {
		return nil, err
	}

	return info, nil
}

func (r *Release) getRelease(repo string, tag string) (*ReleaseInformation, error) {
	latestTag, err := r.getLatestTag(repo, tag)
	if err != nil {
		return nil, err
	}

	branch := r.getBranchFromTag(repo, tag)
	notes, err := r.generateReleaseNotes(repo, tag, latestTag, branch)
	if err != nil {
		return nil, err
	}

	releaseInfo := &ReleaseInformation{
		notes:     notes,
		tag:       tag,
		latestTag: latestTag,
		repo:      repo,
	}

	if repo == "installer" {
		currentTag, err := r.getInstallerCurrentTag()
		if err != nil {
			return nil, err
		}

		releaseInfo.currentTag = currentTag
	}

	if repo == "framework" {
		currentTag, err := r.getFrameworkCurrentTag(r.getBranchFromTag("framework", tag))
		if err != nil {
			return nil, err
		}

		releaseInfo.currentTag = currentTag
	}

	return releaseInfo, nil
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
	notes, err := r.github.GenerateReleaseNotes(owner, repo, &github.GenerateNotesRequest{
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

	if latestRelease.TagName == "" {
		return "", fmt.Errorf("latest release tag name is empty for %s/%s", owner, repo)
	}

	return latestRelease.TagName, nil
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
		if release.TagName != "" && release.TagName == tag {
			return true, nil
		}
	}

	return false, nil
}

func (r *Release) pushBranch(repo, branch string) error {
	if err := r.ctx.Spinner(fmt.Sprintf("Pushing branch %s for %s...", branch, repo), console.SpinnerOption{
		Action: func() error {
			if !r.real {
				color.Yellow().Println(fmt.Sprintf("Preview mode, skip pushing branch %s for %s", branch, repo))
				return nil
			}

			exist, err := r.github.CheckBranchExists(owner, repo, branch)
			if err != nil {
				return fmt.Errorf("failed to check branch %s exist for %s: %w", branch, repo, err)
			}
			if exist {
				color.Yellow().Println(fmt.Sprintf("[%s/%s] Branch %s already exists, skipping push", owner, repo, branch))
				return nil
			}

			if err := r.github.CreateBranch(owner, repo, branch); err != nil {
				return fmt.Errorf("failed to create branch %s for %s: %w", branch, repo, err)
			}

			color.Green().Println(fmt.Sprintf("[%s/%s] Push %s branch success!", owner, repo, branch))

			return nil
		},
	}); err != nil {
		return err
	}

	return nil
}

func (r *Release) printReleaseInformation(releaseInfo *ReleaseInformation) {
	r.divider()
	color.Yellow().Println(fmt.Sprintf("Please check %s/%s information:", owner, releaseInfo.repo))
	r.ctx.NewLine()

	color.Black().Println(releaseInfo.notes.Name)
	color.Black().Println(releaseInfo.notes.Body)

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
			color.Red().Println("---------------------- WARNING ----------------------")
			color.Red().Println("The current tag is not the same as the tag to release")
			color.Red().Println("-----------------------------------------------------")
		}
	}

	r.ctx.NewLine()
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

func (r *Release) releaseExample(examplePR *github.PullRequest, branch string) error {
	repo := "example"

	if err := r.checkPRsMergeStatus(map[string]*github.PullRequest{
		repo: examplePR,
	}); err != nil {
		return fmt.Errorf("failed to check upgrade PRs merge status: %w", err)
	}

	if branch != "" {
		if err := r.pushBranch("example", branch); err != nil {
			return err
		}

		// if err := r.setDefaultBranch("example", branch); err != nil {
		// 	return err
		// }
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

	var goravelReleaseInfo *ReleaseInformation
	if err := r.ctx.Spinner(fmt.Sprintf("Getting %s release information for %s...", repo, tag), console.SpinnerOption{
		Action: func() error {
			var err error
			goravelReleaseInfo, err = r.getRelease(repo, tag)
			return err
		},
	}); err != nil {
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
		// if err := r.setDefaultBranch(repo, branch); err != nil {
		// 	return err
		// }
	}

	return nil
}

func (r *Release) releasePackages(packagesReleaseInfo map[string]*ReleaseInformation, tag, branch string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(packagesReleaseInfo))

	for pkg, releaseInfo := range packagesReleaseInfo {
		if pkg == "framework" {
			continue
		}

		wg.Add(1)
		go func(pkg string, releaseInfo *ReleaseInformation) {
			defer wg.Done()

			if err := r.releaseRepo(releaseInfo); err != nil {
				errCh <- err
				return
			}

			if releaseInfo.repo == "goravel-lite" {
				if branch != "" {
					if err := r.pushBranch(releaseInfo.repo, branch); err != nil {
						errCh <- err
						return
					}
					// Comment this given the operation requires Administrator permission, it's risky.
					// if err := r.setDefaultBranch(releaseInfo.repo, branch); err != nil {
					// 	errCh <- err
					// 	return
					// }
				}
			} else {
				if branch != "" {
					if err := r.pushBranch(releaseInfo.repo, branch); err != nil {
						errCh <- err
						return
					}
				}
			}
		}(pkg, releaseInfo)
	}

	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return err
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
	color.Black().Println("1. Modify the default branch to the new version:")
	color.Black().Println("   https://github.com/goravel/goravel-lite/settings")
	color.Black().Println("   https://github.com/goravel/example/settings")
	color.Black().Println("   https://github.com/goravel/goravel/settings")
	color.Black().Println("2. Install the new version via goravel/installer and test the project works fine")
	color.Black().Println("3. Modify the support policy: https://www.goravel.dev/prologue/releases.html#support-policy")
}

func (r *Release) releasePatchSuccess(frameworkTag string, extraPackages map[string]string) {
	r.ctx.NewLine()
	color.Green().Println(fmt.Sprintf("Release goravel/framework %s success!", frameworkTag))
	if len(extraPackages) > 0 {
		var parts []string
		for _, pkg := range sortedKeys(extraPackages) {
			parts = append(parts, fmt.Sprintf("%s@%s", pkg, extraPackages[pkg]))
		}
		color.Green().Println(fmt.Sprintf("  + Extra packages: %s", strings.Join(parts, ", ")))
	}
}

func (r *Release) releaseSuccess(repo, tagName string) {
	color.Green().Println(fmt.Sprintf("[%s/%s] Release %s success!", owner, repo, tagName))
	color.Green().Println(fmt.Sprintf("Release link: https://github.com/%s/%s/releases/tag/%s", owner, repo, tagName))
}

// func (r *Release) setDefaultBranch(repo, branch string) error {
// 	if err := r.github.SetDefaultBranch(owner, repo, branch); err != nil {
// 		return fmt.Errorf("failed to set default branch %s for %s/%s: %w", branch, owner, repo, err)
// 	}

// 	color.Green().Println(fmt.Sprintf("[%s/%s] Set default branch to %s success!", owner, repo, branch))

// 	return nil
// }
