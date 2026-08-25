// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"forgejo.org/services/otel"
)

// Due to import cycles, the settings live in services/otel

func loadOtel(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "otel", &otel.Setting)
}
