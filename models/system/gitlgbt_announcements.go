// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"context"

	"forgejo.org/models/db"

	"xorm.io/builder"
)

const KeyAnnouncements = "gitlgbt.announcement"

func GetAnnouncementsSetting(ctx context.Context) (*Setting, error) {
	row, exists, err := db.Get[Setting](ctx, builder.Eq{"setting_key": KeyAnnouncements})
	if err != nil {
		return nil, err
	}
	if !exists {
		return &Setting{Version: -1}, nil
	}
	return row, nil
}

// SaveAnnouncementsSetting uses optimistic locking to avoid overwriting another admin's changes.
func SaveAnnouncementsSetting(ctx context.Context, version int, value string) error {
	return SetSettingIfVersion(ctx, KeyAnnouncements, version, value)
}
