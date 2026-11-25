// Copyright 2026 The Gitlgbt Authors. All rights reserved.
// Copyright 2026 Fryrvo All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"forgejo.org/modules/assetfs"
	"forgejo.org/modules/frontend"
	"forgejo.org/modules/setting"
)

const FrontendFolder = frontend.Folder

func FrontendLayer(name, sub string) *assetfs.Layer {
	return assetfs.Local("frontend", setting.CustomPath, FrontendFolder, name, sub)
}

var (
	frontendAssetsMu sync.RWMutex
	frontendAssets   = map[string]*assetfs.LayeredFS{}
)

func frontendAssetFS(name string) *assetfs.LayeredFS {
	frontendAssetsMu.RLock()
	fs := frontendAssets[name]
	frontendAssetsMu.RUnlock()
	if fs != nil {
		return fs
	}

	fs = assetfs.Layered(FrontendLayer(name, "public"), CustomAssets(), BuiltinAssets())
	frontendAssetsMu.Lock()
	frontendAssets[name] = fs
	frontendAssetsMu.Unlock()
	return fs
}

func FrontendFileHandlerFunc() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" && req.Method != "HEAD" {
			resp.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name, file, _ := strings.Cut(strings.TrimPrefix(req.URL.Path, "/assets/_fe/"), "/")
		if !frontend.ValidName(name) {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		root := filepath.Join(setting.CustomPath, FrontendFolder, name)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		publicDir, publicErr := os.Stat(filepath.Join(root, "public"))
		templateDir, templateErr := os.Stat(filepath.Join(root, "templates"))
		if (publicErr != nil || !publicDir.IsDir()) && (templateErr != nil || !templateDir.IsDir()) {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		handleRequest(resp, req, frontendAssetFS(name), "assets/"+file)
	}
}
