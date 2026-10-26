// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnouncementsSchedule(t *testing.T) {
	items := Announcements{
		{ID: "always", Enabled: true},
		{ID: "scheduled", Enabled: true, StartsAt: 100, EndsAt: 200},
		{ID: "disabled", Enabled: false},
	}
	assert.Len(t, items.ActiveAt(99), 1)
	assert.Len(t, items.ActiveAt(100), 2)
	assert.Len(t, items.ActiveAt(199), 2)
	assert.Len(t, items.ActiveAt(200), 1)
	assert.Equal(t, "scheduled", items[1].StatusAt(99))
	assert.Equal(t, "active", items[1].StatusAt(100))
	assert.Equal(t, "expired", items[1].StatusAt(200))
	assert.Equal(t, "disabled", items[2].StatusAt(100))
}
