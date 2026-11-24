// Copyright 2022 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/url"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	packages_model "forgejo.org/models/packages"
	access_model "forgejo.org/models/perm/access"
	project_model "forgejo.org/models/project"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/optional"
	project_module "forgejo.org/modules/project"
	"forgejo.org/modules/setting"
	"forgejo.org/routers/web/repo"
	"forgejo.org/services/context"
	funding_service "forgejo.org/services/funding"
)

// i change my mind alot so change here if need to
var (
	userProfileRepoNames = []string{"user", ".user", "me", ".me"}
	orgProfileRepoNames  = []string{"org", ".org", "organization", ".organization"}
)

// prepareContextForCommonProfile store some common data into context data for user's profile related pages (including the nav menu)
// It is designed to be fast and safe to be called multiple times in one request
func prepareContextForCommonProfile(ctx *context.Context) {
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["EnableFeed"] = setting.Other.EnableFeed
	ctx.Data["FeedURL"] = ctx.ContextUser.HomeLink()
}

// PrepareContextForProfileBigAvatar set the context for big avatar view on the profile page
func PrepareContextForProfileBigAvatar(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	ctx.Data["IsModerationEnabled"] = setting.Moderation.Enabled
	ctx.Data["IsBlocked"] = ctx.Doer != nil && user_model.IsBlocked(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
	ctx.Data["IsFollowing"] = ctx.Doer != nil && user_model.IsFollowing(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail && ctx.ContextUser.Email != "" && ctx.IsSigned && !ctx.ContextUser.KeepEmailPrivate
	if setting.Service.UserLocationMapURL != "" {
		ctx.Data["ContextUserLocationMapURL"] = setting.Service.UserLocationMapURL + url.QueryEscape(ctx.ContextUser.Location)
	}
	// Show OpenID URIs
	openIDs, err := user_model.GetUserOpenIDs(ctx, ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openIDs
	if len(ctx.ContextUser.Description) != 0 {
		content, err := markdown.RenderString(&markup.RenderContext{
			Metas: map[string]string{"mode": "document"},
			Ctx:   ctx,
		}, ctx.ContextUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	orgs, err := db.Find[organization.Organization](ctx, organization.FindOrgOptions{
		UserID:         ctx.ContextUser.ID,
		IncludeLimited: ctx.IsSigned,
		IncludePrivate: showPrivate,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = organization.HasOrgsVisible(ctx, orgs, ctx.Doer)

	badges, _, err := user_model.GetUserBadges(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetUserBadges", err)
		return
	}
	ctx.Data["Badges"] = badges

	// in case the numbers are already provided by other functions, no need to query again (which is slow)
	if _, ok := ctx.Data["NumFollowers"]; !ok {
		_, ctx.Data["NumFollowers"], _ = user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{PageSize: 1, Page: 1})
	}
	if _, ok := ctx.Data["NumFollowing"]; !ok {
		_, ctx.Data["NumFollowing"], _ = user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{PageSize: 1, Page: 1})
	}
}

func FindUserProfileReadme(ctx *context.Context, doer *user_model.User) (*repo_model.Repository, *git.Repository, *git.Blob, *funding_service.RepoFunding, func()) {
	names := []string{ctx.ContextUser.Name}
	if ctx.ContextUser.IsOrganization() {
		names = append(names, orgProfileRepoNames...)
	} else {
		names = append(names, userProfileRepoNames...)
	}
	names = append(names, context.OwnerProfileRepositoryName)

	for _, name := range names {
		dbRepo, gitRepo, blob, funding := findProfileReadmeInRepo(ctx, doer, name)
		if blob != nil {
			return dbRepo, gitRepo, blob, funding, func() { _ = gitRepo.Close() }
		}
	}
	return nil, nil, nil, nil, func() {}
}

func findProfileReadmeInRepo(ctx *context.Context, doer *user_model.User, repoName string) (*repo_model.Repository, *git.Repository, *git.Blob, *funding_service.RepoFunding) {
	dbRepo, err := repo_model.GetRepositoryByName(ctx, ctx.ContextUser.ID, repoName)
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			log.Error("FindUserProfileReadme failed to GetRepositoryByName: %v", err)
		}
		return nil, nil, nil, nil
	}
	// no private repo etc you goy it
	if dbRepo.IsFork || dbRepo.IsPrivate || dbRepo.IsEmpty {
		return nil, nil, nil, nil
	}
	if perm, err := access_model.GetUserRepoPermission(ctx, dbRepo, doer); err != nil || !perm.CanRead(unit.TypeCode) {
		return nil, nil, nil, nil
	}
	gitRepo, err := gitrepo.OpenRepository(ctx, dbRepo)
	if err != nil {
		log.Error("FindUserProfileReadme failed to OpenRepository: %v", err)
		return nil, nil, nil, nil
	}
	commit, err := gitRepo.GetBranchCommit(dbRepo.DefaultBranch)
	if err != nil {
		log.Error("FindUserProfileReadme failed to GetBranchCommit: %v", err)
		_ = gitRepo.Close()
		return nil, nil, nil, nil
	}

	root, err := commit.SubTree("")
	if err == nil {
		var rootEntries []*git.TreeEntry
		if rootEntries, err = root.ListEntries(); err == nil {
			lists := [][]*git.TreeEntry{rootEntries}
			for _, e := range rootEntries {
				if e.IsDir() && strings.EqualFold(e.Name(), context.OwnerProfileRepositoryName) {
					if sub, err := commit.SubTree(e.Name()); err == nil {
						if subEntries, err := sub.ListEntries(); err == nil {
							lists = [][]*git.TreeEntry{subEntries, rootEntries}
						}
					}
					break
				}
			}
			for _, entries := range lists {
				if _, readme, err := repo.FindReadmeFileInEntries(ctx, entries, true); err == nil && readme != nil {
					funding, err := funding_service.GetFundingFromCommit(dbRepo, commit)
					if err != nil && !funding_service.IsNotExistError(err) {
						log.Error("FindUserProfileReadme failed to GetFundingFromCommit: %v", err)
					}
					return dbRepo, gitRepo, readme.Blob(), funding
				}
			}
		}
	}
	if err != nil {
		log.Error("FindUserProfileReadme failed to read tree: %v", err)
	}
	_ = gitRepo.Close()
	return nil, nil, nil, nil
}

func RenderUserHeader(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	_, _, profileReadmeBlob, _, profileClose := FindUserProfileReadme(ctx, ctx.Doer)
	defer profileClose()
	ctx.Data["HasProfileReadme"] = profileReadmeBlob != nil
}

func LoadHeaderCount(ctx *context.Context) error {
	prepareContextForCommonProfile(ctx)

	var err error

	ctx.Data["RepoCount"], err = repo_model.CountRepository(ctx, &repo_model.SearchRepoOptions{
		Actor:              ctx.Doer,
		OwnerID:            ctx.ContextUser.ID,
		Private:            ctx.IsSigned,
		Collaborate:        optional.Some(false),
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		return err
	}

	var ownerType project_module.OwnerType
	if ctx.ContextUser.IsOrganization() {
		ownerType = project_module.TypeOrganization
	} else {
		ownerType = project_module.TypeIndividual
	}
	ctx.Data["ProjectCount"], err = db.Count[project_model.Project](ctx, project_model.SearchOptions{
		OwnerID:  ctx.ContextUser.ID,
		IsClosed: optional.Some(false),
		Type:     ownerType,
	})
	if err != nil {
		return err
	}
	ctx.Data["PackageCount"], err = packages_model.CountOwnerPackages(ctx, ctx.ContextUser.ID)
	if err != nil {
		return err
	}

	return nil
}
