// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	system_model "forgejo.org/models/system"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/setting/config"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminAnnouncements(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	config.GetDynGetter().InvalidateCache()
	t.Cleanup(func() { config.GetDynGetter().InvalidateCache() })
	admin := loginUser(t, "user1")
	ordinary := loginUser(t, "user2")
	ordinary.MakeRequest(t, NewRequest(t, "GET", "/admin/announcements"), http.StatusForbidden)
	ordinary.MakeRequest(t, NewRequestWithValues(t, "POST", "/admin/announcements", map[string]string{"message": "Unauthorized"}), http.StatusForbidden)

	post := func(values map[string]string, status int) *httptest.ResponseRecorder {
		page := admin.MakeRequest(t, NewRequest(t, "GET", "/admin/announcements"), http.StatusOK)
		values["_csrf"] = NewHTMLParser(t, page.Body).GetInputValueByName("_csrf")
		row, err := system_model.GetAnnouncementsSetting(t.Context())
		require.NoError(t, err)
		values["version"] = strconv.Itoa(row.Version)
		return admin.MakeRequest(t, NewRequestWithValues(t, "POST", "/admin/announcements", values), status)
	}
	values := map[string]string{"enabled": "on", "message": "New build runners are available. [Read the changelog](https://example.com/news)"}
	post(values, http.StatusSeeOther)
	preview := admin.MakeRequest(t, NewRequestWithValues(t, "POST", "/admin/announcements/preview", map[string]string{
		"mode": "gfm", "context": setting.AppSubURL + "/",
		"text": "**New release**\n\n- Faster builds\n\n[Unsafe](javascript:alert(1))<script>alert(1)</script>",
	}), http.StatusOK)
	assert.Contains(t, preview.Body.String(), "<strong>New release</strong>")
	assert.Contains(t, preview.Body.String(), "<li>Faster builds</li>")
	assert.NotContains(t, preview.Body.String(), "javascript:")
	assert.NotContains(t, preview.Body.String(), "<script>")
	ordinary.MakeRequest(t, NewRequestWithValues(t, "POST", "/admin/announcements/preview", map[string]string{"text": "test", "mode": "gfm"}), http.StatusForbidden)
	first := setting.Config().Announcement.Value(t.Context())[0]
	assert.NotEmpty(t, first.ID)
	assert.Equal(t, values["message"], first.Message)
	page := admin.MakeRequest(t, NewRequest(t, "GET", "/admin/announcements"), http.StatusOK)
	assert.NotContains(t, page.Body.String(), `name="link_url"`)
	assert.NotContains(t, page.Body.String(), `name="link_text"`)
	values["id"] = first.ID
	post(values, http.StatusSeeOther)
	assert.Equal(t, first.ID, setting.Config().Announcement.Value(t.Context())[0].ID)
	values["ends_at"] = time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:04")
	post(values, http.StatusSeeOther)
	assert.Equal(t, first.ID, setting.Config().Announcement.Value(t.Context())[0].ID)
	values["starts_at"] = "invalid"
	invalid := post(values, http.StatusUnprocessableEntity)
	assert.Contains(t, invalid.Body.String(), "Enter a message of up to 500 characters")
	assert.Contains(t, invalid.Body.String(), `class="field error"`)
	nextPage := admin.MakeRequest(t, NewRequest(t, "GET", "/admin/announcements"), http.StatusOK)
	assert.NotContains(t, nextPage.Body.String(), "Enter a message of up to 500 characters")
	values["starts_at"] = "2030-01-01T00:00"
	values["ends_at"] = "2029-01-01T00:00"
	post(values, http.StatusUnprocessableEntity)
	second := map[string]string{"enabled": "on", "message": "**New release**\n\n- Faster builds\n- Better logs\n\n<script>alert(1)</script>"}
	post(second, http.StatusSeeOther)
	public := MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
	assert.Contains(t, public.Body.String(), "<strong>New release</strong>")
	assert.Contains(t, public.Body.String(), `href="https://example.com/news"`)
	assert.NotContains(t, public.Body.String(), "<script>alert(1)</script>")
	assert.Equal(t, 2, NewHTMLParser(t, public.Body).Find(".gitlgbt-announcement").Length())
	post(map[string]string{"enabled": "on", "message": "Future announcement", "starts_at": "2099-01-01T00:00"}, http.StatusSeeOther)
	post(map[string]string{"enabled": "on", "message": "Expired announcement", "ends_at": "2020-01-01T00:00"}, http.StatusSeeOther)
	public = MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
	assert.Equal(t, 2, NewHTMLParser(t, public.Body).Find(".gitlgbt-announcement").Length())
	row, err := system_model.GetAnnouncementsSetting(t.Context())
	require.NoError(t, err)
	require.ErrorIs(t, system_model.SaveAnnouncementsSetting(t.Context(), row.Version-1, "[]"), system_model.ErrSettingChanged)
	// Stale forms cannot overwrite more recent changes.
	conflict := admin.MakeRequest(t, NewRequestWithValues(t, "POST", "/admin/announcements", second), http.StatusConflict)
	assert.Contains(t, conflict.Body.String(), "Another administrator changed the announcements")
	post(map[string]string{"id": first.ID, "action": "delete"}, http.StatusSeeOther)
	public = MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
	assert.Equal(t, 1, NewHTMLParser(t, public.Body).Find(".gitlgbt-announcement").Length())
	assert.Len(t, setting.Config().Announcement.Value(t.Context()), 3)

	post(map[string]string{"enabled": "on", "message": strings.Repeat("a", 501)}, http.StatusUnprocessableEntity)
	post(map[string]string{"enabled": "on", "message": "   "}, http.StatusUnprocessableEntity)
	// Browser form submissions add CR to each LF without increasing the editor's count.
	message := strings.Repeat("a", 490) + strings.Repeat("\r\nb", 5)
	post(map[string]string{"enabled": "on", "message": message}, http.StatusSeeOther)
	items := setting.Config().Announcement.Value(t.Context())
	assert.Equal(t, strings.ReplaceAll(message, "\r\n", "\n"), items[len(items)-1].Message)
	post(map[string]string{"enabled": "on", "message": "[status](status)"}, http.StatusSeeOther)
	preview = admin.MakeRequest(t, NewRequestWithValues(t, "POST", "/admin/announcements/preview", map[string]string{"mode": "gfm", "context": setting.AppSubURL + "/", "text": "[status](status)"}), http.StatusOK)
	assert.Contains(t, preview.Body.String(), `href="`+setting.AppSubURL+`/status"`)
	public = MakeRequest(t, NewRequest(t, "GET", "/explore/repos"), http.StatusOK)
	assert.Contains(t, public.Body.String(), `href="`+setting.AppSubURL+`/status"`)
}
