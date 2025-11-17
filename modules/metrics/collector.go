// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package metrics

import (
	"fmt"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/modules/setting"
	"github.com/VictoriaMetrics/metrics"
)

func Register() {
	metrics.NewGauge(fmt.Sprintf("forgejo_build_info{version=%q}", setting.AppVer), func() float64 {
		return 1
	})
	metrics.NewGauge("forgejo_start_timestamp", func() float64 {
		return float64(setting.AppStartTime.Unix())
	})
	metrics.NewGauge("forgejo_uptime_seconds", func() float64 {
		return float64(time.Since(setting.AppStartTime).Seconds())
	})

	metrics.NewGauge("forgejo_db_connections_max", func() float64 {
		stats := db.GetEngine(db.DefaultContext).DB().Stats()
		return float64(stats.MaxOpenConnections)
	})
	metrics.NewGauge("forgejo_db_connections_total", func() float64 {
		stats := db.GetEngine(db.DefaultContext).DB().Stats()
		return float64(stats.OpenConnections)
	})
	metrics.NewGauge("forgejo_db_connections_in_use", func() float64 {
		stats := db.GetEngine(db.DefaultContext).DB().Stats()
		return float64(stats.InUse)
	})
	metrics.NewGauge("forgejo_db_connections_idle", func() float64 {
		stats := db.GetEngine(db.DefaultContext).DB().Stats()
		return float64(stats.Idle)
	})
	metrics.NewGauge("forgejo_db_wait_count", func() float64 {
		stats := db.GetEngine(db.DefaultContext).DB().Stats()
		return float64(stats.WaitCount)
	})
	metrics.NewGauge("forgejo_db_wait_duration", func() float64 {
		stats := db.GetEngine(db.DefaultContext).DB().Stats()
		return float64(stats.WaitDuration.Milliseconds())
	})
}
