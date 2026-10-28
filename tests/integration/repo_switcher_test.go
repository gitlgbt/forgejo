// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

type testRepoSwitcherEntry struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Link     string `json:"link"`
	Private  bool   `json:"private"`
	Archived bool   `json:"archived"`
}

type testRepoSwitcherResponse struct {
	Results []testRepoSwitcherEntry `json:"results"`
	More    bool                    `json:"more"`
}

func TestRepoSwitcherList(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 owns several public repos in the fixtures, including repo1.
	t.Run("anonymous sees only public repos", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/repo-switcher/list")
		resp := MakeRequest(t, req, http.StatusOK)
		var body testRepoSwitcherResponse
		DecodeJSON(t, resp, &body)

		assert.NotEmpty(t, body.Results)
		for _, r := range body.Results {
			assert.False(t, r.Private, "anonymous response must not contain private repos: %s", r.FullName)
		}
		// every returned repo belongs to user2
		for _, r := range body.Results {
			assert.Contains(t, r.FullName, "user2/")
		}
	})

	t.Run("owner sees their private repos", func(t *testing.T) {
		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user2/repo1/repo-switcher/list")
		resp := session.MakeRequest(t, req, http.StatusOK)
		var body testRepoSwitcherResponse
		DecodeJSON(t, resp, &body)

		hasPrivate := false
		for _, r := range body.Results {
			if r.Private {
				hasPrivate = true
			}
		}
		assert.True(t, hasPrivate, "owner should see at least one private repo")
	})

	t.Run("third-party user cannot see owner private repos", func(t *testing.T) {
		// user4 is a regular non-admin user with no collaboration or team access
		// to any of user2's private repos (repo2, repo15); verified via fixtures.
		session := loginUser(t, "user4")
		req := NewRequest(t, "GET", "/user2/repo1/repo-switcher/list")
		resp := session.MakeRequest(t, req, http.StatusOK)
		var body testRepoSwitcherResponse
		DecodeJSON(t, resp, &body)

		assert.NotEmpty(t, body.Results)
		for _, r := range body.Results {
			assert.False(t, r.Private, "third-party user must not see private repos: %s", r.FullName)
		}
		// every returned repo must belong to user2
		for _, r := range body.Results {
			assert.Contains(t, r.FullName, "user2/")
		}
	})
}
