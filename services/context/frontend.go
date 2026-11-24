// Copyright 2026 The Gitlgbt Authors. All rights reserved.
// Copyright 2026 Fryrvo All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/templates"
)

func (ctx *Context) frontendRenderer() *templates.HTMLRender {
	name, ok := ctx.Data["Frontend"].(string)
	if !ok {
		if ctx.Doer != nil {
			name, _ = user_model.GetUserSetting(ctx, ctx.Doer.ID, templates.FrontendSettingKey)
		}
		name = templates.ResolveFrontend(name)
		ctx.Data["Frontend"] = name
	}
	return templates.FrontendRenderer(name)
}
