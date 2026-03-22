// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/schemas"
)

type UserBadge struct { //revive:disable-line:exported
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	UserID  int64
}

// TableIndices implements xorm's TableIndices interface
func (n *UserBadge) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 1)
	ubUnique := schemas.NewIndex("unique_user_badge", schemas.UniqueType)
	ubUnique.AddColumn("user_id", "badge_id")
	indices = append(indices, ubUnique)
	return indices
}

// AddUniqueIndexForUserBadge adds a compound unique indexes for user badge table
// and it replaces an old index on user_id
func AddUniqueIndexForUserBadge(x *xorm.Engine) error {
	// remove possible duplicated records in table user_badge
	type result struct {
		UserID  int64
		BadgeID int64
		Cnt     int
	}
	var results []result
	if err := x.Select("user_id, badge_id, count(*) as cnt").
		Table("user_badge").
		GroupBy("user_id, badge_id").
		Having("count(*) > 1").
		Find(&results); err != nil {
		return err
	}
	for _, r := range results {
		var ids []int64
		if err := x.SQL("SELECT id FROM user_badge WHERE user_id = ? and badge_id = ? limit ?", r.UserID, r.BadgeID, r.Cnt-1).Find(&ids); err != nil {
			return err
		}
		if _, err := x.Table("user_badge").In("id", ids).Delete(); err != nil {
			return err
		}
	}

	return x.Sync(new(UserBadge))
}
