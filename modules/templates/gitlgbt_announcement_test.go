// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRenderAnnouncement(t *testing.T) {
	defer test.MockVariableValue(&setting.AppSubURL, "/forge")()
	content := "[status](status) <script>alert(1)</script>"
	first := RenderAnnouncement(t.Context(), "render-test", content)
	assert.Contains(t, string(first), `href="/forge/status"`)
	assert.NotContains(t, string(first), "<script>")
	assert.Equal(t, first, RenderAnnouncement(t.Context(), "render-test", content))
	assert.Contains(t, string(RenderAnnouncement(t.Context(), "render-test", "**Updated**")), "<strong>Updated</strong>")
}
