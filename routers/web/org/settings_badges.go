// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"net/http"

	"forgejo.org/models/organization"
	"forgejo.org/modules/base"
	shared_user "forgejo.org/routers/web/shared/user"
	"forgejo.org/services/context"
)

const tplSettingsBadges base.TplName = "org/settings/badges"

func SettingsBadges(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.badges")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsBadges"] = true

	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	badges, err := organization.FindOrgBadgeItems(ctx, ctx.Org.Organization.ID, true)
	if err != nil {
		ctx.ServerError("FindOrgBadgeItems", err)
		return
	}
	ctx.Data["Badges"] = badges
	ctx.HTML(http.StatusOK, tplSettingsBadges)
}
