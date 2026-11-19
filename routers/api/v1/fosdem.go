// Copyright 2026 Gusted. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package v1

import (
	"net/http"
	"time"

	activities_model "forgejo.org/models/activities"
	"forgejo.org/models/db"
	user_model "forgejo.org/models/user"
	"forgejo.org/services/context"
)

func FosdemStats(ctx *context.APIContext) {
	since := max(ctx.FormInt64("since"), time.Now().Add(-time.Hour*48).Unix())

	var total int64
	_, err := db.GetEngine(ctx).SQL("SELECT COUNT(*) FROM user WHERE created_unix >= ? AND type = ?", since, user_model.UserTypeIndividual).Get(&total)
	if err != nil {
		ctx.ServerError("Counting", err)
		return
	}

	var commits int64
	_, err = db.GetEngine(ctx).SQL("SELECT SUM(JSON_LENGTH(content, '$.Commits')) FROM action WHERE created_unix >= ? AND op_type = ? AND content != ''", since, activities_model.ActionCommitRepo).Get(&commits)
	if err != nil {
		ctx.ServerError("Commits", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"users":   total,
		"commits": commits,
	})
}
