// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package admin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"forgejo.org/models/organization"
	"forgejo.org/modules/base"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
	"forgejo.org/services/context"
)

const (
	tplOrgBadges base.TplName = "admin/org_badges"
	tplOrgBadge  base.TplName = "admin/org_badge"
)

type orgBadgeOrgRow struct {
	Org      *organization.Organization
	IsPublic bool
}

type orgBadgeFields struct {
	Name, Description, IconURL, LinkURL string
}

func validHTTPURL(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func readOrgBadgeFields(ctx *context.Context) *orgBadgeFields {
	f := &orgBadgeFields{
		Name:        strings.TrimSpace(ctx.FormString("name")),
		Description: strings.TrimSpace(ctx.FormString("description")),
		IconURL:     strings.TrimSpace(ctx.FormString("icon_url")),
		LinkURL:     strings.TrimSpace(ctx.FormString("link_url")),
	}
	if f.Name == "" || len(f.Name) > 100 || len(f.Description) > 500 ||
		!validHTTPURL(f.IconURL) || (f.LinkURL != "" && !validHTTPURL(f.LinkURL)) {
		return nil
	}
	return f
}

func orgBadgeLink(id int64) string {
	return fmt.Sprintf("%s/admin/badges/%d", setting.AppSubURL, id)
}

func loadOrgBadgeType(ctx *context.Context) *organization.OrgBadgeType {
	b, err := organization.GetOrgBadgeType(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound("GetOrgBadgeType", err)
		} else {
			ctx.ServerError("GetOrgBadgeType", err)
		}
		return nil
	}
	return b
}

func OrgBadges(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.org_badges")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminOrgBadges"] = true

	types, err := organization.FindOrgBadgeTypes(ctx)
	if err != nil {
		ctx.ServerError("FindOrgBadgeTypes", err)
		return
	}
	counts := make(map[int64]int64, len(types))
	for _, t := range types {
		n, err := organization.CountOrgBadgeAwards(ctx, t.ID)
		if err != nil {
			ctx.ServerError("CountOrgBadgeAwards", err)
			return
		}
		counts[t.ID] = n
	}
	ctx.Data["BadgeTypes"] = types
	ctx.Data["BadgeOrgCounts"] = counts
	ctx.HTML(http.StatusOK, tplOrgBadges)
}

func OrgBadgeNew(ctx *context.Context) {
	f := readOrgBadgeFields(ctx)
	if f == nil {
		ctx.Flash.Error(ctx.Tr("org.badges.invalid"))
		ctx.Redirect(setting.AppSubURL + "/admin/badges")
		return
	}
	b := &organization.OrgBadgeType{Name: f.Name, Description: f.Description, IconURL: f.IconURL, LinkURL: f.LinkURL}
	if err := organization.CreateOrgBadgeType(ctx, b); err != nil {
		ctx.ServerError("CreateOrgBadgeType", err)
		return
	}
	ctx.Redirect(orgBadgeLink(b.ID))
}

func OrgBadgeView(ctx *context.Context) {
	b := loadOrgBadgeType(ctx)
	if b == nil {
		return
	}
	ctx.Data["Title"] = b.Name
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminOrgBadges"] = true

	awards, err := organization.FindOrgBadgeAwards(ctx, b.ID)
	if err != nil {
		ctx.ServerError("FindOrgBadgeAwards", err)
		return
	}
	rows := make([]*orgBadgeOrgRow, 0, len(awards))
	for _, a := range awards {
		org, err := organization.GetOrgByID(ctx, a.OrgID)
		if err != nil {
			continue
		}
		rows = append(rows, &orgBadgeOrgRow{Org: org, IsPublic: a.IsPublic})
	}
	ctx.Data["BadgeType"] = b
	ctx.Data["BadgeOrgs"] = rows
	ctx.HTML(http.StatusOK, tplOrgBadge)
}

func OrgBadgeEdit(ctx *context.Context) {
	b := loadOrgBadgeType(ctx)
	if b == nil {
		return
	}
	f := readOrgBadgeFields(ctx)
	if f == nil {
		ctx.Flash.Error(ctx.Tr("org.badges.invalid"))
		ctx.Redirect(orgBadgeLink(b.ID))
		return
	}
	b.Name, b.Description, b.IconURL, b.LinkURL = f.Name, f.Description, f.IconURL, f.LinkURL
	if err := organization.UpdateOrgBadgeType(ctx, b); err != nil {
		ctx.ServerError("UpdateOrgBadgeType", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("admin.org_badges.saved"))
	ctx.Redirect(orgBadgeLink(b.ID))
}

func OrgBadgeDelete(ctx *context.Context) {
	b := loadOrgBadgeType(ctx)
	if b == nil {
		return
	}
	if err := organization.DeleteOrgBadgeType(ctx, b.ID); err != nil {
		ctx.ServerError("DeleteOrgBadgeType", err)
		return
	}
	ctx.JSONRedirect(setting.AppSubURL + "/admin/badges")
}

func OrgBadgeAward(ctx *context.Context) {
	b := loadOrgBadgeType(ctx)
	if b == nil {
		return
	}
	names := strings.FieldsFunc(ctx.FormString("org_names"), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})

	var awarded, already, missing []string
	for _, name := range names {
		org, err := organization.GetOrgByName(ctx, name)
		if err != nil {
			if organization.IsErrOrgNotExist(err) {
				missing = append(missing, name)
				continue
			}
			ctx.ServerError("GetOrgByName", err)
			return
		}
		created, err := organization.AwardOrgBadge(ctx, org.ID, b.ID)
		if err != nil {
			ctx.ServerError("AwardOrgBadge", err)
			return
		}
		if created {
			awarded = append(awarded, org.Name)
		} else {
			already = append(already, org.Name)
		}
	}

	if len(awarded) > 0 {
		ctx.Flash.Success(ctx.Tr("admin.org_badges.awarded", strings.Join(awarded, ", ")))
	}
	if len(already) > 0 {
		ctx.Flash.Info(ctx.Tr("admin.org_badges.already", strings.Join(already, ", ")))
	}
	if len(missing) > 0 {
		ctx.Flash.Error(ctx.Tr("admin.org_badges.org_not_found", strings.Join(missing, ", ")))
	}
	ctx.Redirect(orgBadgeLink(b.ID))
}

func OrgBadgeRevoke(ctx *context.Context) {
	b := loadOrgBadgeType(ctx)
	if b == nil {
		return
	}
	if err := organization.RevokeOrgBadge(ctx, ctx.FormInt64("org_id"), b.ID); err != nil {
		ctx.ServerError("RevokeOrgBadge", err)
		return
	}
	ctx.JSONRedirect(orgBadgeLink(b.ID))
}
