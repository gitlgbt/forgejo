// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"errors"
	"net/http"

	"forgejo.org/models/organization"
	"forgejo.org/modules/base"
	"forgejo.org/modules/log"
	"forgejo.org/modules/util"
	shared_user "forgejo.org/routers/web/shared/user"
	"forgejo.org/services/context"
)

const tplBadges base.TplName = "org/badge/badges"

func Badges(ctx *context.Context) {
	org := ctx.Org.Organization
	ctx.Data["Title"] = org.FullName
	ctx.Data["PageIsOrgBadges"] = true

	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	badges, err := organization.FindOrgBadgeItems(ctx, org.ID, false)
	if err != nil {
		ctx.ServerError("FindOrgBadgeItems", err)
		return
	}
	ctx.Data["Badges"] = badges
	ctx.HTML(http.StatusOK, tplBadges)
}

func BadgesAction(ctx *context.Context) {
	if !ctx.Org.IsOwner {
		ctx.Error(http.StatusNotFound)
		return
	}
	action := ctx.Params(":action")
	if action != "public" && action != "private" {
		ctx.Error(http.StatusNotFound)
		return
	}

	err := organization.SetOrgBadgeAwardPublic(ctx, ctx.Org.Organization.ID, ctx.FormInt64("id"), action == "public")
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound)
			return
		}
		log.Error("BadgesAction(%s): %v", action, err)
		ctx.JSON(http.StatusOK, map[string]any{"ok": false, "err": err.Error()})
		return
	}
	back := ctx.Org.OrgLink + "/badges"
	if ctx.FormString("back") == "settings" {
		back = ctx.Org.OrgLink + "/settings/badges"
	}
	ctx.JSONRedirect(back)
}
