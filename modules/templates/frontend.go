// Copyright 2026 The Gitlgbt Authors. All rights reserved.
// Copyright 2026 Fryrvo All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"forgejo.org/modules/assetfs"
	"forgejo.org/modules/log"
	"forgejo.org/modules/public"
	"forgejo.org/modules/setting"
)

const (
	DefaultFrontend    = "gitlgbt" // default fornted / themplate name is hardcoded here
	FrontendSettingKey = "frontend"
)

var frontends = map[string]*HTMLRender{}

func (h *HTMLRender) assetFS() *assetfs.LayeredFS {
	return assetfs.Layered(public.FrontendLayer(h.name, "templates"), CustomAssets(), BuiltinAssets())
}

func loadFrontends() {
	frontends = map[string]*HTMLRender{DefaultFrontend: htmlRender}
	entries, _ := os.ReadDir(filepath.Join(setting.CustomPath, public.FrontendFolder))
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || name == DefaultFrontend {
			continue
		}
		r := &HTMLRender{name: name}
		if err := r.CompileTemplates(); err != nil || r.templates.Load() == nil {
			log.Error("Frontend %q disabled, template error: %v", name, err)
			continue
		}
		frontends[name] = r
	}
}

func Frontends() []string {
	return slices.Sorted(maps.Keys(frontends))
}

func ResolveFrontend(name string) string {
	if _, ok := frontends[name]; ok {
		return name
	}
	return DefaultFrontend
}

func FrontendRenderer(name string) *HTMLRender {
	if r, ok := frontends[name]; ok {
		return r
	}
	return HTMLRenderer()
}
