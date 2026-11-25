// Copyright 2026 The Gitlgbt Authors. All rights reserved.
// Copyright 2026 Fryrvo All rights reserved.
// SPDX-License-Identifier: MIT

package frontend

import "regexp"

// config name have been move here
const (
	Folder      = "fryrvo_template_switcher"
	DefaultName = "gitlgbt"
	SettingKey  = "frontend"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidName(name string) bool {
	return validName.MatchString(name)
}
