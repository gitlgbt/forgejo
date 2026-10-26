// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"time"
)

// Announcement describes a scheduled site-wide message.
type Announcement struct {
	ID       string
	Enabled  bool
	Message  string
	StartsAt int64
	EndsAt   int64
}

// Announcements stores the ordered list in one atomic setting.
type Announcements []Announcement

// Active returns announcements within their inclusive start and exclusive end times.
func (a Announcements) Active() Announcements {
	return a.ActiveAt(time.Now().Unix())
}

func (a Announcements) ActiveAt(now int64) Announcements {
	var active Announcements
	for _, item := range a {
		if item.StatusAt(now) == "active" {
			active = append(active, item)
		}
	}
	return active
}

// Status describes the current publication state for the admin list.
func (a Announcement) Status() string {
	return a.StatusAt(time.Now().Unix())
}

func (a Announcement) StatusAt(now int64) string {
	if !a.Enabled {
		return "disabled"
	}
	if a.EndsAt != 0 && now >= a.EndsAt {
		return "expired"
	}
	if a.StartsAt != 0 && now < a.StartsAt {
		return "scheduled"
	}
	return "active"
}

func (a Announcement) StartInput() string { return announcementTimeInput(a.StartsAt) }
func (a Announcement) EndInput() string   { return announcementTimeInput(a.EndsAt) }
func announcementTimeInput(value int64) string {
	if value == 0 {
		return ""
	}
	return time.Unix(value, 0).UTC().Format("2006-01-02T15:04")
}
