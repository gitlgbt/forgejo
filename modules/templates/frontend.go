// Copyright 2026 The Gitlgbt Authors. All rights reserved.
// Copyright 2026 Fryrvo All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"forgejo.org/modules/assetfs"
	"forgejo.org/modules/frontend"
	"forgejo.org/modules/log"
	"forgejo.org/modules/public"
	"forgejo.org/modules/setting"
)

const (
	DefaultFrontend    = frontend.DefaultName
	FrontendSettingKey = frontend.SettingKey
)

var (
	frontends   = map[string]*HTMLRender{}
	frontendsMu sync.RWMutex
)

func (h *HTMLRender) assetFS() *assetfs.LayeredFS {
	return assetfs.Layered(public.FrontendLayer(h.name, "templates"), CustomAssets(), BuiltinAssets())
}

func loadFrontends() {
	loaded := map[string]*HTMLRender{DefaultFrontend: htmlRender}
	entries, err := os.ReadDir(filepath.Join(setting.CustomPath, frontend.Folder))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error("Unable to discover frontends: %v", err)
		}
		storeFrontends(loaded)
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !ValidFrontendName(name) || name == DefaultFrontend {
			continue
		}
		root := filepath.Join(setting.CustomPath, frontend.Folder, name)
		if !directoryExists(filepath.Join(root, "templates")) && !directoryExists(filepath.Join(root, "public")) {
			continue
		}
		r := &HTMLRender{name: name}
		if err := r.CompileTemplates(); err != nil || r.templates.Load() == nil {
			log.Error("Frontend %q disabled, template error: %v", name, err)
			continue
		}
		loaded[name] = r
	}
	storeFrontends(loaded)
}

func storeFrontends(loaded map[string]*HTMLRender) {
	frontendsMu.Lock()
	frontends = loaded
	frontendsMu.Unlock()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func Frontends() []string {
	frontendsMu.RLock()
	defer frontendsMu.RUnlock()
	names := slices.Sorted(maps.Keys(frontends))
	slices.SortStableFunc(names, func(a, b string) int {
		if a == DefaultFrontend {
			return -1
		}
		if b == DefaultFrontend {
			return 1
		}
		return 0
	})
	return names
}

func ResolveFrontend(name string) string {
	frontendsMu.RLock()
	defer frontendsMu.RUnlock()
	if _, ok := frontends[name]; ok {
		return name
	}
	return DefaultFrontend
}

func FrontendRenderer(name string) *HTMLRender {
	frontendsMu.RLock()
	defer frontendsMu.RUnlock()
	if r, ok := frontends[name]; ok {
		return r
	}
	return HTMLRenderer()
}

func ValidFrontendName(name string) bool {
	return frontend.ValidName(name)
}
