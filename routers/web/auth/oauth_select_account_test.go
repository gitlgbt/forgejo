// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/url"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/forms"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectAccountAuthorizeURLHelpers(t *testing.T) {
	for _, subURL := range []string{"", "/sub"} {
		t.Run("AppSubURL="+subURL, func(t *testing.T) {
			defer test.MockVariableValue(&setting.AppSubURL, subURL)()

			wrongPrefix := "/sub/login/oauth/authorize?x=1"
			if subURL == "/sub" {
				wrongPrefix = "/login/oauth/authorize?x=1"
			}

			assert.True(t, isAuthorizeRedirect(subURL+"/login/oauth/authorize?x=1"))
			assert.False(t, isAuthorizeRedirect(wrongPrefix))
			assert.False(t, isAuthorizeRedirect(subURL+"/login/oauth/authorize/../x"))
			assert.False(t, isAuthorizeRedirect(setting.AppURL+"login/oauth/authorize"))
			assert.False(t, isAuthorizeRedirect("/\\evil.example/login/oauth/authorize"))
			assert.False(t, isAuthorizeRedirect(""))

			got := authorizeURLWithoutPrompt(&forms.AuthorizationForm{
				ResponseType: "code",
				ClientID:     "client",
				RedirectURI:  "https://client.example/cb",
				State:        "s",
				Prompt:       "select_account",
			})
			assert.Contains(t, got, subURL+"/login/oauth/authorize?")
			assert.True(t, isAuthorizeRedirect(got), got)
			u, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, subURL+"/login/oauth/authorize", u.Path)
			assert.False(t, u.Query().Has("prompt"))
			assert.Equal(t, "client", u.Query().Get("client_id"))
			assert.Equal(t, "https://client.example/cb", u.Query().Get("redirect_uri"))
		})
	}
}
