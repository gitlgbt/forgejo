// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"
	"forgejo.org/services/authz"
	"xorm.io/builder"
)

const (
	// contributionsMaxAgeSeconds How old data to retrieve for the heatmap.
	// 371 days to cover the entire heatmap (53 *full* weeks)
	contributionsMaxAgeSeconds = 32054400
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     timeutil.TimeStamp `json:"timestamp"`
	Contributions int64              `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData
func GetUserHeatmapDataByUser(ctx context.Context, user, doer *user_model.User, reducer authz.AuthorizationReducer) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, user, nil, doer, reducer)
}

// GetUserHeatmapDataByUserTeam returns an array of UserHeatmapData
func GetUserHeatmapDataByUserTeam(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, user, team, doer, nil)
}

func getUserHeatmapData(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User, reducer authz.AuthorizationReducer) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !ActivityReadable(user, doer) {
		return hdata, nil
	}

	// Group by 15 minute intervals which will allow the client to accurately shift the timestamp to their timezone.
	// The interval is based on the fact that there are timezones such as UTC +5:30 and UTC +12:45.
	groupBy := "created_unix / 900 * 900"
	if setting.Database.Type.IsMySQL() {
		groupBy = "created_unix DIV 900 * 900"
	}

	cond, err := activityQueryCondition(ctx, GetFeedsOptions{
		RequestedUser:  user,
		RequestedTeam:  team,
		Actor:          doer,
		IncludePrivate: true, // don't filter by private, as we already filter by repo access
		// * Heatmaps for individual users only include actions that the user themself did.
		// * For organizations actions by all users that were made in owned
		//   repositories are counted.
		OnlyPerformedBy:      !user.IsOrganization(),
		AuthorizationReducer: reducer,
	})
	if err != nil {
		return nil, err
	}

	return hdata, db.GetEngine(ctx).
		Select(groupBy+" AS timestamp, count(user_id) as contributions").
		Table("action").
		Where(cond).
		And("created_unix >= ?", timeutil.TimeStampNow()-contributionsMaxAgeSeconds).
		GroupBy("timestamp").
		OrderBy("timestamp").
		Find(&hdata)
}

// GetTotalContributionsInHeatmap returns the total number of contributions in a heatmap
func GetTotalContributionsInHeatmap(hdata []*UserHeatmapData) int64 {
	var total int64
	for _, v := range hdata {
		total += v.Contributions
	}
	return total
}

// heatmap data
func GetUserHeatmapDataByUserFiltered(ctx context.Context, user, doer *user_model.User, reducer authz.AuthorizationReducer, year int, orgID int64) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !ActivityReadable(user, doer) {
		return hdata, nil
	}

	groupBy := "created_unix / 900 * 900"
	if setting.Database.Type.IsMySQL() {
		groupBy = "created_unix DIV 900 * 900"
	}

	cond, err := activityQueryCondition(ctx, GetFeedsOptions{
		RequestedUser:        user,
		Actor:                doer,
		IncludePrivate:       true,
		OnlyPerformedBy:      !user.IsOrganization(),
		AuthorizationReducer: reducer,
	})
	if err != nil {
		return nil, err
	}

	q := db.GetEngine(ctx).
		Select(groupBy + " AS timestamp, count(user_id) as contributions").
		Table("action").
		Where(cond)

	if year == 0 {
		q = q.And("created_unix >= ?", int64(timeutil.TimeStampNow())-contributionsMaxAgeSeconds)
	} else {
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
		end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
		q = q.And("created_unix >= ?", start).And("created_unix < ?", end)
	}

	if orgID != 0 {
		q = q.And(builder.In("repo_id", builder.Select("id").From("repository").Where(builder.Eq{"owner_id": orgID})))
	}

	return hdata, q.GroupBy("timestamp").OrderBy("timestamp").Find(&hdata)
}

func GetUserHeatmapDataByUserYear(ctx context.Context, user, doer *user_model.User, reducer authz.AuthorizationReducer, year int) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !ActivityReadable(user, doer) {
		return hdata, nil
	}

	groupBy := "created_unix / 900 * 900"
	if setting.Database.Type.IsMySQL() {
		groupBy = "created_unix DIV 900 * 900"
	}

	cond, err := activityQueryCondition(ctx, GetFeedsOptions{
		RequestedUser:        user,
		Actor:                doer,
		IncludePrivate:       true,
		OnlyPerformedBy:      !user.IsOrganization(),
		AuthorizationReducer: reducer,
	})
	if err != nil {
		return nil, err
	}

	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

	return hdata, db.GetEngine(ctx).
		Select(groupBy+" AS timestamp, count(user_id) as contributions").
		Table("action").
		Where(cond).
		And("created_unix >= ?", start).
		And("created_unix < ?", end).
		GroupBy("timestamp").
		OrderBy("timestamp").
		Find(&hdata)
}

type userOrgContribution struct {
	OwnerID       int64 `xorm:"owner_id"`
	Contributions int64 `xorm:"contributions"`
}

func GetUserOrgContributionCounts(ctx context.Context, user, doer *user_model.User, reducer authz.AuthorizationReducer, year int) (map[int64]int64, error) {
	counts := make(map[int64]int64)

	if !ActivityReadable(user, doer) {
		return counts, nil
	}

	opts := GetFeedsOptions{
		RequestedUser:        user,
		Actor:                doer,
		IncludePrivate:       true,
		OnlyPerformedBy:      !user.IsOrganization(),
		AuthorizationReducer: reducer,
	}
	opts.repositoryTableAvailable = true
	cond, err := activityQueryCondition(ctx, opts)
	if err != nil {
		return nil, err
	}

	q := db.GetEngine(ctx).
		Select("`repository`.owner_id AS owner_id, count(`action`.id) AS contributions").
		Table("action").
		Join("INNER", "repository", "`repository`.id = `action`.repo_id").
		Where(cond)

	if year == 0 {
		q = q.And("`action`.created_unix >= ?", int64(timeutil.TimeStampNow())-contributionsMaxAgeSeconds)
	} else {
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
		end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
		q = q.And("`action`.created_unix >= ?", start).And("`action`.created_unix < ?", end)
	}

	rows := make([]*userOrgContribution, 0)
	if err := q.GroupBy("`repository`.owner_id").Find(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.OwnerID] = r.Contributions
	}
	return counts, nil
}
