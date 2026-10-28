// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	user_model "forgejo.org/models/user"
	"forgejo.org/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selectAccountAuthorizeURL targets the confidential fixture app, for which
// user1 already has a grant.
const selectAccountAuthorizeURL = "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=a&response_type=code&state=thestate"

// chooserRedirectTo returns the redirect_to value embedded in the chooser forms.
func chooserRedirectTo(t *testing.T, htmlDoc *HTMLDoc) string {
	t.Helper()
	redirectTo, ok := htmlDoc.Find("form[action='/login/oauth/select_account'] input[name='redirect_to']").First().Attr("value")
	require.True(t, ok)
	return redirectTo
}

func TestAuthorizeSelectAccountShowsChooser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	for _, prompt := range []string{"select_account", "consent select_account"} {
		t.Run(prompt, func(t *testing.T) {
			req := NewRequest(t, "GET", selectAccountAuthorizeURL+"&prompt="+url.QueryEscape(prompt))
			resp := session.MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			htmlDoc.AssertElement(t, "form[action='/login/oauth/select_account'] input[name='uid'][value='1']", true)
			htmlDoc.AssertElement(t, "#select-account-add", true)

			redirectTo := chooserRedirectTo(t, htmlDoc)
			assert.True(t, strings.HasPrefix(redirectTo, "/login/oauth/authorize?"), redirectTo)
			u, err := url.Parse(redirectTo)
			require.NoError(t, err)
			assert.False(t, u.Query().Has("prompt"))
			assert.Equal(t, "da7da3ba-9a13-4167-856f-3899de0b0138", u.Query().Get("client_id"))
			assert.Equal(t, "thestate", u.Query().Get("state"))
		})
	}
}

func TestAuthorizeSelectAccountPost(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequestWithValues(t, "POST", "/login/oauth/authorize", map[string]string{
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"redirect_uri":  "a",
		"response_type": "code",
		"state":         "thestate",
		"prompt":        "select_account",
	})
	resp := session.MakeRequest(t, req, http.StatusOK)

	u, err := url.Parse(chooserRedirectTo(t, NewHTMLParser(t, resp.Body)))
	require.NoError(t, err)
	assert.Equal(t, "da7da3ba-9a13-4167-856f-3899de0b0138", u.Query().Get("client_id"))
	assert.Equal(t, "a", u.Query().Get("redirect_uri"))
}

func TestAuthorizeOtherPromptKeepsAutoGrant(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", selectAccountAuthorizeURL+"&prompt=login")
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	require.NoError(t, err)
	assert.NotEmpty(t, u.Query().Get("code"))
}

func TestAuthorizeSelectAccountStillValidatesClient(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=UNREGISTERED&response_type=code&state=thestate&prompt=select_account")
	resp := session.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Unregistered Redirect URI")
}

// addSwitchAccount signs the second account in through the switcher's
// add-account flow, leaving it as the active account.
func addSwitchAccount(t *testing.T, session *TestSession, username string) {
	t.Helper()
	req := NewRequest(t, "POST", "/user/account_switch/add")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": username,
		"password":  userPassword,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func TestSelectAccountSwitchesAndIssuesCode(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")
	addSwitchAccount(t, session, "user2")
	require.Equal(t, "user2", signedInUsername(t, session))

	// Both accounts are offered.
	req := NewRequest(t, "GET", selectAccountAuthorizeURL+"&prompt=select_account")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "input[name='uid'][value='1']", true)
	htmlDoc.AssertElement(t, "input[name='uid'][value='2']", true)
	redirectTo := chooserRedirectTo(t, htmlDoc)

	// Choosing user1 switches the session and resumes the authorize request.
	req = NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
		"redirect_to": redirectTo,
		"uid":         "1",
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, redirectTo, resp.Header().Get("Location"))
	assert.Equal(t, "user1", signedInUsername(t, session))

	// user1 has a grant for the app, so the code is issued for user1.
	req = NewRequest(t, "GET", redirectTo)
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	require.NoError(t, err)
	code, err := auth_model.GetOAuth2AuthorizationByCode(db.DefaultContext, u.Query().Get("code"))
	require.NoError(t, err)
	require.NotNil(t, code)
	assert.EqualValues(t, 1, code.Grant.UserID)
}

func TestSelectAccountCurrentUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
		"redirect_to": selectAccountAuthorizeURL,
		"uid":         "1",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, selectAccountAuthorizeURL, resp.Header().Get("Location"))
	assert.Equal(t, "user1", signedInUsername(t, session))
}

func TestSelectAccountRejectsUnlistedAccount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	for _, uid := range []string{"1", "0", ""} {
		req := NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
			"redirect_to": selectAccountAuthorizeURL,
			"uid":         uid,
		})
		session.MakeRequest(t, req, http.StatusBadRequest)
	}
	assert.Equal(t, "user2", signedInUsername(t, session))
}

func TestSelectAccountRejectsForeignRedirect(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	for _, redirectTo := range []string{
		"",
		"https://evil.example/login/oauth/authorize?client_id=x",
		"//evil.example/login/oauth/authorize?client_id=x",
		"/user/settings",
		"/login/oauth/authorize/../../user/settings",
	} {
		req := NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
			"redirect_to": redirectTo,
			"uid":         "1",
		})
		session.MakeRequest(t, req, http.StatusBadRequest)

		req = NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
			"redirect_to": redirectTo,
			"add":         "1",
		})
		session.MakeRequest(t, req, http.StatusBadRequest)
	}
	assert.Equal(t, "user1", signedInUsername(t, session))
}

func TestSelectAccountProhibitedAccount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")
	addSwitchAccount(t, session, "user2")

	// user1 is disabled after it was added to the switcher.
	_, err := db.GetEngine(db.DefaultContext).ID(1).Cols("prohibit_login").Update(&user_model.User{ProhibitLogin: true})
	require.NoError(t, err)

	req := NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
		"redirect_to": selectAccountAuthorizeURL,
		"uid":         "1",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	require.NoError(t, err)
	assert.Equal(t, "/login/oauth/authorize", u.Path)
	assert.Equal(t, "select_account", u.Query().Get("prompt"))
	assert.Equal(t, "user2", signedInUsername(t, session))
}

func TestSelectAccountAddAnotherAccount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	req := NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
		"redirect_to": selectAccountAuthorizeURL,
		"add":         "1",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user/login?add_account=1", resp.Header().Get("Location"))

	// Signing in the other account returns to the authorize request.
	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": "user1",
		"password":  userPassword,
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, selectAccountAuthorizeURL, resp.Header().Get("Location"))
	assert.Equal(t, "user1", signedInUsername(t, session))
}

func TestSelectAccountChooserCancel(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", selectAccountAuthorizeURL+"&prompt=select_account&scope=openid")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "#select-account-cancel", true)

	// Submit the values of the cancel form as the browser would.
	values := map[string]string{}
	htmlDoc.Find("form[action='/login/oauth/grant'] input[type='hidden']").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		value, _ := s.Attr("value")
		if name != "_csrf" {
			values[name] = value
		}
	})
	assert.Equal(t, "da7da3ba-9a13-4167-856f-3899de0b0138", values["client_id"])
	assert.Equal(t, "thestate", values["state"])
	assert.Equal(t, "a", values["redirect_uri"])
	assert.Equal(t, "openid", values["scope"])
	values["granted"] = "false"

	req = NewRequestWithValues(t, "POST", "/login/oauth/grant", values)
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	// The fixture's redirect URI is the relative "a", which the redirect
	// resolves against the grant endpoint's path.
	u, err := url.Parse(resp.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/login/oauth/a", u.Path)
	assert.Equal(t, "access_denied", u.Query().Get("error"))
	assert.Equal(t, "thestate", u.Query().Get("state"))
	assert.Empty(t, u.Query().Get("code"))
}

func TestSelectAccountChooserHidesProhibitedAccounts(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	addSwitchAccount(t, session, "user1")
	require.Equal(t, "user1", signedInUsername(t, session))

	// user2 is disabled after it was added to the switcher.
	_, err := db.GetEngine(db.DefaultContext).ID(2).Cols("prohibit_login").Update(&user_model.User{ProhibitLogin: true})
	require.NoError(t, err)

	req := NewRequest(t, "GET", selectAccountAuthorizeURL+"&prompt=select_account")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "input[name='uid'][value='1']", true)
	htmlDoc.AssertElement(t, "input[name='uid'][value='2']", false)
}

func TestSelectAccountChooserStaleRedirectCookie(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	// Start "use another account" from the chooser and abandon the login page.
	req := NewRequestWithValues(t, "POST", "/login/oauth/select_account", map[string]string{
		"redirect_to": selectAccountAuthorizeURL,
		"add":         "1",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// A later add from the navbar switcher must not resume the OAuth flow.
	req = NewRequest(t, "POST", "/user/account_switch/add")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": "user1",
		"password":  userPassword,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.NotContains(t, resp.Header().Get("Location"), "/login/oauth/authorize")
	assert.Equal(t, "user1", signedInUsername(t, session))
}
