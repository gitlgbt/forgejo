// Copyright 2026 The Gitlgbt Authors. All rights reserved.
// Copyright 2026 Fryrvo All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"net/http"
	"strings"

	"forgejo.org/modules/assetfs"
	"forgejo.org/modules/setting"
)

const FrontendFolder = "fryrvo_template_switcher" // switch folder name here

func FrontendLayer(name, sub string) *assetfs.Layer {
	return assetfs.Local("frontend", setting.CustomPath, FrontendFolder, name, sub)
}

func FrontendFileHandlerFunc() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" && req.Method != "HEAD" {
			resp.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name, file, _ := strings.Cut(strings.TrimPrefix(req.URL.Path, "/assets/_fe/"), "/")
		if name == "" || strings.HasPrefix(name, ".") || strings.Contains(name, "\\") {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		handleRequest(resp, req, assetfs.Layered(FrontendLayer(name, "public"), CustomAssets(), BuiltinAssets()), "assets/"+file)
	}
}
