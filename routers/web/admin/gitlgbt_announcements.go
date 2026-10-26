// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package admin

import (
	"errors"
	"net/http"
	"strconv"

	system_model "forgejo.org/models/system"
	"forgejo.org/modules/base"
	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/setting/config"
	"forgejo.org/modules/web"
	"forgejo.org/services/context"
	"forgejo.org/services/forms"

	"github.com/google/uuid"
)

const tplAnnouncements base.TplName = "admin/gitlgbt_announcements"

func announcementPage(ctx *context.Context, items setting.Announcements, version, status int) {
	ctx.Data["Title"] = ctx.Tr("admin.announcements")
	ctx.Data["PageIsAdminAnnouncements"] = true
	ctx.Data["Announcements"] = items
	type entry struct {
		Announcement setting.Announcement
		Form         *forms.AdminAnnouncementForm
		HasError     bool
	}
	var entries []entry
	submitted, _ := ctx.Data["AnnouncementForm"].(*forms.AdminAnnouncementForm)
	newForm := forms.NewAdminAnnouncementForm(setting.Announcement{Enabled: true})
	for _, item := range items {
		form := forms.NewAdminAnnouncementForm(item)
		hasError := submitted != nil && submitted.ID == item.ID
		if hasError {
			form = submitted
		}
		entries = append(entries, entry{item, form, hasError})
	}
	if submitted != nil && submitted.ID == "" {
		newForm = submitted
	}
	ctx.Data["AnnouncementEntries"] = entries
	ctx.Data["NewAnnouncementForm"] = newForm
	ctx.Data["NewAnnouncementHasError"] = submitted != nil && submitted.ID == ""
	ctx.Data["AnnouncementVersion"] = version
	ctx.HTML(status, tplAnnouncements)
}

func loadAnnouncements(ctx *context.Context) (setting.Announcements, int, bool) {
	row, err := system_model.GetAnnouncementsSetting(ctx)
	if err != nil {
		ctx.ServerError("Load announcements", err)
		return nil, 0, false
	}
	var items setting.Announcements
	if row.SettingValue != "" {
		if err := json.Unmarshal([]byte(row.SettingValue), &items); err != nil {
			ctx.ServerError("Decode announcements", err)
			return nil, 0, false
		}
	}
	return items, row.Version, true
}

// Announcements shows all global announcements, including scheduled and disabled messages.
func Announcements(ctx *context.Context) {
	items, version, ok := loadAnnouncements(ctx)
	if ok {
		announcementPage(ctx, items, version, http.StatusOK)
	}
}

// AnnouncementsPost creates, edits, or deletes a single announcement without changing the others.
func AnnouncementsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminAnnouncementForm)
	items, version, ok := loadAnnouncements(ctx)
	if !ok {
		return
	}
	submittedVersion, err := strconv.Atoi(form.Version)
	if err != nil || submittedVersion != version {
		ctx.Flash.Error(ctx.Tr("admin.announcements.conflict"), true)
		announcementPage(ctx, items, version, http.StatusConflict)
		return
	}
	id := form.ID
	index := -1
	for i, item := range items {
		if item.ID == id {
			index = i
			break
		}
	}
	if id != "" && index == -1 {
		ctx.NotFound("Announcement", nil)
		return
	}
	if ctx.HasError() {
		ctx.Flash.Error(ctx.Tr("admin.announcements.invalid"), true)
		ctx.Data["AnnouncementForm"] = form
		announcementPage(ctx, items, version, http.StatusUnprocessableEntity)
		return
	}
	if form.Action == "delete" {
		if index == -1 {
			ctx.NotFound("Announcement", nil)
			return
		}
		items = append(items[:index:index], items[index+1:]...)
	} else {
		next := form.Announcement()
		if index == -1 {
			next.ID = uuid.NewString()
			items = append(items, next)
		} else {
			// Schedule-only edits preserve dismissals; content changes and re-enabling reset them.
			old := items[index]
			if old.Message != next.Message || (!old.Enabled && next.Enabled) {
				next.ID = uuid.NewString()
			}
			items[index] = next
		}
	}
	value, err := json.Marshal(items)
	if err != nil {
		ctx.ServerError("Marshal announcements", err)
		return
	}
	err = system_model.SaveAnnouncementsSetting(ctx, version, string(value))
	if errors.Is(err, system_model.ErrSettingChanged) {
		ctx.Flash.Error(ctx.Tr("admin.announcements.conflict"))
		ctx.Redirect(setting.AppSubURL + "/admin/announcements")
		return
	}
	if err != nil {
		ctx.ServerError("Save announcements", err)
		return
	}
	config.GetDynGetter().InvalidateCache()
	ctx.Flash.Success(ctx.Tr("admin.announcements.saved"))
	ctx.Redirect(setting.AppSubURL + "/admin/announcements")
}
