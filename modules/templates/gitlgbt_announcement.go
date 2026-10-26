// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"context"
	"html/template"
	"sync"

	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/setting"
)

type announcementRenderKey struct{ id, message, base string }

var announcementHTML = struct {
	sync.Mutex
	entries map[announcementRenderKey]template.HTML
}{entries: make(map[announcementRenderKey]template.HTML)}

// RenderAnnouncement caches sanitized Markdown independently of request URLs and users.
func RenderAnnouncement(ctx context.Context, id, message string) template.HTML {
	key := announcementRenderKey{id, message, setting.AppSubURL + "/"}
	announcementHTML.Lock()
	defer announcementHTML.Unlock()
	if rendered, ok := announcementHTML.entries[key]; ok {
		return rendered
	}
	rendered, err := markdown.RenderString(&markup.RenderContext{
		Ctx:   ctx,
		Links: markup.Links{Base: key.base, AbsolutePrefix: true},
		Metas: map[string]string{"mode": "document"},
	}, message)
	if err != nil {
		log.Error("RenderAnnouncement: %v", err)
		return rendered
	}
	// Bound memory when administrators repeatedly edit or delete announcements.
	if len(announcementHTML.entries) >= 128 {
		clear(announcementHTML.entries)
	}
	announcementHTML.entries[key] = rendered
	return rendered
}
