// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forms

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/web/middleware"
	"forgejo.org/services/context"

	"code.forgejo.org/go-chi/binding"
)

type AdminAnnouncementForm struct {
	ID       string `binding:"MaxSize(36)"`
	Version  string `binding:"Required"`
	Action   string `binding:"MaxSize(10)"`
	Enabled  bool
	Message  string `binding:"Required;MaxSize(500)" locale:"admin.announcements.message"`
	StartsAt string `locale:"admin.announcements.starts_at"`
	EndsAt   string `locale:"admin.announcements.ends_at"`
}

func (f *AdminAnnouncementForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// HTML forms submit CRLF, while the editor and FormData preview use LF.
	f.Message = strings.TrimSpace(strings.ReplaceAll(f.Message, "\r\n", "\n"))
	filtered := make(binding.Errors, 0, len(errs))
	for _, err := range errs {
		if len(err.FieldNames) > 0 && err.FieldNames[0] != "Message" {
			filtered = append(filtered, err)
		}
	}
	errs = filtered
	if f.Action != "delete" {
		if f.Message == "" {
			errs.Add([]string{"Message"}, binding.ERR_REQUIRED, "Required")
		}
		if utf8.RuneCountInString(f.Message) > 500 {
			errs.Add([]string{"Message"}, binding.ERR_MAX_SIZE, "MaxSize")
		}
		start, startErr := parseAnnouncementTime(f.StartsAt)
		end, endErr := parseAnnouncementTime(f.EndsAt)
		if startErr != nil || start < 0 {
			errs.Add([]string{"StartsAt"}, "InvalidDate", "Invalid UTC start time")
		}
		if endErr != nil || end < 0 || (end != 0 && start != 0 && end <= start) {
			errs.Add([]string{"EndsAt"}, "InvalidDate", "End must be after start")
		}
	}
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

func parseAnnouncementTime(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := time.Parse("2006-01-02T15:04", value)
	if err != nil {
		return 0, err
	}
	return parsed.Unix(), nil
}

// Announcement returns the normalized data after successful form validation.
func (f *AdminAnnouncementForm) Announcement() setting.Announcement {
	start, _ := parseAnnouncementTime(f.StartsAt)
	end, _ := parseAnnouncementTime(f.EndsAt)
	return setting.Announcement{ID: f.ID, Enabled: f.Enabled, Message: f.Message, StartsAt: start, EndsAt: end}
}

func NewAdminAnnouncementForm(a setting.Announcement) *AdminAnnouncementForm {
	return &AdminAnnouncementForm{ID: a.ID, Enabled: a.Enabled, Message: a.Message, StartsAt: a.StartInput(), EndsAt: a.EndInput()}
}
