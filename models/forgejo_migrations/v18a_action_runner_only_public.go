// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add OnlyPublic field to ActionRunner",
		Upgrade:     addWorkflowSourceCommit,
	})
}

func addActionRunnerOnlyPublic(x *xorm.Engine) error {
	type ActionRunner struct {
		OnlyPublic bool `xorm:"only_public DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunner))
	return err
}
