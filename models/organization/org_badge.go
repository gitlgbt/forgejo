// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package organization

import (
	"context"

	"forgejo.org/models/db"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"
)

type OrgBadgeType struct {
	ID          int64              `xorm:"pk autoincr"`
	Name        string             `xorm:"VARCHAR(100) NOT NULL"`
	Description string             `xorm:"TEXT"`
	IconURL     string             `xorm:"VARCHAR(2048) NOT NULL"`
	LinkURL     string             `xorm:"VARCHAR(2048)"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

type OrgBadgeAward struct {
	ID          int64              `xorm:"pk autoincr"`
	OrgID       int64              `xorm:"UNIQUE(org_badge_award) NOT NULL"`
	BadgeID     int64              `xorm:"UNIQUE(org_badge_award) INDEX NOT NULL"`
	IsPublic    bool               `xorm:"NOT NULL DEFAULT true"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

type OrgBadgeItem struct {
	AwardID  int64
	IsPublic bool
	OrgBadgeType
}

func init() {
	db.RegisterModel(new(OrgBadgeType))
	db.RegisterModel(new(OrgBadgeAward))
}

func (b OrgBadgeType) HoverText() string {
	if b.Description != "" {
		return b.Name + ": " + b.Description
	}
	return b.Name
}

// site admin

func FindOrgBadgeTypes(ctx context.Context) ([]*OrgBadgeType, error) {
	types := make([]*OrgBadgeType, 0, 10)
	return types, db.GetEngine(ctx).Asc("id").Find(&types)
}

func GetOrgBadgeType(ctx context.Context, id int64) (*OrgBadgeType, error) {
	b := new(OrgBadgeType)
	has, err := db.GetEngine(ctx).ID(id).Get(b)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return b, nil
}

func CreateOrgBadgeType(ctx context.Context, b *OrgBadgeType) error {
	_, err := db.GetEngine(ctx).Insert(b)
	return err
}

func UpdateOrgBadgeType(ctx context.Context, b *OrgBadgeType) error {
	_, err := db.GetEngine(ctx).ID(b.ID).Cols("name", "description", "icon_url", "link_url").Update(b)
	return err
}

func DeleteOrgBadgeType(ctx context.Context, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("badge_id = ?", id).Delete(new(OrgBadgeAward)); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).ID(id).Delete(new(OrgBadgeType))
		return err
	})
}

func CountOrgBadgeAwards(ctx context.Context, badgeID int64) (int64, error) {
	return db.GetEngine(ctx).Where("badge_id = ?", badgeID).Count(new(OrgBadgeAward))
}

func FindOrgBadgeAwards(ctx context.Context, badgeID int64) ([]*OrgBadgeAward, error) {
	awards := make([]*OrgBadgeAward, 0, 10)
	return awards, db.GetEngine(ctx).Where("badge_id = ?", badgeID).Asc("id").Find(&awards)
}

func AwardOrgBadge(ctx context.Context, orgID, badgeID int64) (bool, error) {
	has, err := db.GetEngine(ctx).Where("org_id = ? AND badge_id = ?", orgID, badgeID).Exist(new(OrgBadgeAward))
	if err != nil || has {
		return false, err
	}
	_, err = db.GetEngine(ctx).Insert(&OrgBadgeAward{OrgID: orgID, BadgeID: badgeID, IsPublic: true})
	return err == nil, err
}

func RevokeOrgBadge(ctx context.Context, orgID, badgeID int64) error {
	_, err := db.GetEngine(ctx).Where("org_id = ? AND badge_id = ?", orgID, badgeID).Delete(new(OrgBadgeAward))
	return err
}

func DeleteOrgBadgeAwardsByOrg(ctx context.Context, orgID int64) error {
	_, err := db.GetEngine(ctx).Where("org_id = ?", orgID).Delete(new(OrgBadgeAward))
	return err
}

func SetOrgBadgeAwardPublic(ctx context.Context, orgID, awardID int64, public bool) error {
	a := new(OrgBadgeAward)
	has, err := db.GetEngine(ctx).ID(awardID).And("org_id = ?", orgID).Get(a)
	if err != nil {
		return err
	}
	if !has {
		return util.ErrNotExist
	}
	a.IsPublic = public
	_, err = db.GetEngine(ctx).ID(a.ID).Cols("is_public").Update(a)
	return err
}

func FindOrgBadgeItems(ctx context.Context, orgID int64, includeHidden bool) ([]*OrgBadgeItem, error) {
	awards := make([]*OrgBadgeAward, 0, 10)
	sess := db.GetEngine(ctx).Where("org_id = ?", orgID)
	if !includeHidden {
		sess = sess.And("is_public = ?", true)
	}
	if err := sess.Asc("id").Find(&awards); err != nil {
		return nil, err
	}
	if len(awards) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(awards))
	for _, a := range awards {
		ids = append(ids, a.BadgeID)
	}
	types := make([]*OrgBadgeType, 0, len(ids))
	if err := db.GetEngine(ctx).In("id", ids).Find(&types); err != nil {
		return nil, err
	}
	byID := make(map[int64]*OrgBadgeType, len(types))
	for _, t := range types {
		byID[t.ID] = t
	}

	items := make([]*OrgBadgeItem, 0, len(awards))
	for _, a := range awards {
		t, ok := byID[a.BadgeID]
		if !ok {
			continue
		}
		items = append(items, &OrgBadgeItem{AwardID: a.ID, IsPublic: a.IsPublic, OrgBadgeType: *t})
	}
	return items, nil
}
