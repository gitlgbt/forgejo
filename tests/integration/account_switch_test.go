// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/keying"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/translation"
	"forgejo.org/tests"

	"github.com/markbates/goth"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonRedirect returns the "redirect" target of a JSONRedirect response.
func jsonRedirect(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Redirect string `json:"redirect"`
	}
	DecodeJSON(t, resp, &body)
	return body.Redirect
}

// signedInUsername returns the username shown in the navbar user dropdown.
func signedInUsername(t *testing.T, session *TestSession) string {
	t.Helper()
	req := NewRequest(t, "GET", "/")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	return strings.TrimSpace(htmlDoc.Find(".navbar-right .header strong").First().Text())
}

func TestAccountSwitch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	locale := translation.NewLocale("en-US")

	// Sign in as user2 (the initial account).
	session := loginUser(t, "user2")
	assert.Equal(t, "user2", signedInUsername(t, session))

	// The navbar exposes the "add an account" action but no switch targets yet.
	req := NewRequest(t, "GET", "/")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/add']", true)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/1']", false)

	// Start adding another account: keeps user2 signed in and redirects to login
	// with the add-account intent carried in the query string.
	req = NewRequest(t, "POST", "/user/account_switch/add")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/user/login?add_account=1", jsonRedirect(t, resp))

	// While in add mode the login form is shown even though we are signed in.
	req = NewRequest(t, "GET", "/user/login?add_account=1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "form[action='/user/login?add_account=1']", true)
	assert.Contains(t, htmlDoc.Find(".ui.info.message").Text(), locale.TrString("auth.switch_account_prompt"))

	// Authenticate the second account (admin user1). The form posts back with the
	// add-account query parameter.
	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": "user1",
		"password":  userPassword,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// We are now user1, and user2 is offered as a switch target.
	assert.Equal(t, "user1", signedInUsername(t, session))
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/2']", true)

	// Switch back to user2.
	req = NewRequest(t, "POST", "/user/account_switch/2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/", jsonRedirect(t, resp))
	assert.Equal(t, "user2", signedInUsername(t, session))

	// user1 is still available to switch to.
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/1']", true)

	// Remove user1 from the switcher; it can no longer be switched to.
	req = NewRequest(t, "POST", "/user/account_switch/1/remove")
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/1']", false)

	// A switch attempt to the removed (or any non-listed) account is rejected
	// and keeps the current session unchanged.
	req = NewRequest(t, "POST", "/user/account_switch/1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/", jsonRedirect(t, resp))
	assert.Equal(t, "user2", signedInUsername(t, session))
}

// TestAccountSwitchAddBrowserFlow replicates the exact sequence the browser
// performs: the "Add an account" link-action POSTs to the endpoint, then the
// JS submits the returned redirect through /-/fetch-redirect, and only then
// does the browser GET /user/login. This proves the switch-add session flag
// survives that extra hop and the login form is actually shown.
func TestAccountSwitchAddBrowserFlow(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// 1. link-action POST → JSON {redirect: /user/login?add_account=1}
	req := NewRequest(t, "POST", "/user/account_switch/add")
	resp := session.MakeRequest(t, req, http.StatusOK)
	redirect := jsonRedirect(t, resp)
	assert.Equal(t, "/user/login?add_account=1", redirect)

	// 2. the JS submits the redirect through the fetch-redirect delegate
	req = NewRequestWithValues(t, "POST", "/-/fetch-redirect", map[string]string{
		"redirect": redirect,
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user/login?add_account=1", resp.Header().Get("Location"))

	// 3. the browser follows the redirect to the login form, still signed in
	req = NewRequest(t, "GET", "/user/login?add_account=1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "form[action='/user/login?add_account=1']", true)
}

// TestAccountSwitchAddQueryParamOnly verifies the add-account intent works from
// the URL query alone, without any pre-existing session flag. This mirrors an
// environment where the session write of the flag does not survive the redirect
// (e.g. multiple backend nodes): the login form must still be shown to a
// signed-in user, and authenticating must switch to the new account.
func TestAccountSwitchAddQueryParamOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Sign in as user2, then hit the login page in add mode via the query
	// parameter directly (no POST to /user/account_switch/add, so no flag set).
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user/login?add_account=1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "form[action='/user/login?add_account=1']", true)

	// Authenticating posts back with the query parameter and switches to user1.
	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": "user1",
		"password":  userPassword,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "user1", signedInUsername(t, session))

	// user2 remains available as a switch target.
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/2']", true)
}

// TestAccountSwitchWith2FA verifies that adding an account which requires 2FA
// keeps the switcher state across the session regenerations of the 2FA flow.
func TestAccountSwitchWith2FA(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Enroll TOTP for user4 so the account being added requires 2FA.
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	loginUser(t, user4.Name).EnrollTOTP(t)
	twoFactor, err := auth.GetTwoFactorByUID(db.DefaultContext, user4.ID)
	require.NoError(t, err)
	secret, err := keying.TOTP.Decrypt(twoFactor.Secret, keying.ColumnAndID("secret", twoFactor.ID))
	require.NoError(t, err)

	// Sign in as user2 and start adding the second account.
	session := loginUser(t, "user2")
	req := NewRequest(t, "POST", "/user/account_switch/add")
	session.MakeRequest(t, req, http.StatusOK)

	// Authenticating user4 stops at the 2FA challenge, still in add mode.
	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": user4.Name,
		"password":  userPassword,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user/two_factor", resp.Header().Get("Location"))

	// Complete the 2FA challenge.
	passcode, err := totp.GenerateCode(string(secret), time.Now())
	require.NoError(t, err)
	req = NewRequestWithValues(t, "POST", "/user/two_factor", map[string]string{
		"passcode": passcode,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// We are now user4, and user2 survived the 2FA flow as a switch target.
	assert.Equal(t, user4.Name, signedInUsername(t, session))
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/2']", true)

	// Switching back to user2 works without re-authenticating.
	req = NewRequest(t, "POST", "/user/account_switch/2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/", jsonRedirect(t, resp))
	assert.Equal(t, "user2", signedInUsername(t, session))
}

// TestAccountSwitchWith2FARemembered verifies that adding an account which
// requires 2FA works while the signed-in account has a "remember me" cookie.
// The 2FA pages must not use that cookie to sign the current account back in.
func TestAccountSwitchWith2FARemembered(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Enroll TOTP for user4 so the account being added requires 2FA.
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	loginUser(t, user4.Name).EnrollTOTP(t)
	twoFactor, err := auth.GetTwoFactorByUID(db.DefaultContext, user4.ID)
	require.NoError(t, err)
	secret, err := keying.TOTP.Decrypt(twoFactor.Secret, keying.ColumnAndID("secret", twoFactor.ID))
	require.NoError(t, err)

	// Sign in as user2 with "remember me" and start adding the second account.
	session := loginUserWithPasswordRemember(t, "user2", userPassword, true)
	require.NotEmpty(t, session.GetCookie(setting.CookieRememberName))
	req := NewRequest(t, "POST", "/user/account_switch/add")
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": user4.Name,
		"password":  userPassword,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user/two_factor", resp.Header().Get("Location"))

	// The 2FA pages are shown instead of signing user2 back in.
	for _, page := range []string{"/user/two_factor", "/user/two_factor/scratch", "/user/webauthn"} {
		req = NewRequest(t, "GET", page)
		session.MakeRequest(t, req, http.StatusOK)
	}

	passcode, err := totp.GenerateCode(string(secret), time.Now())
	require.NoError(t, err)
	req = NewRequestWithValues(t, "POST", "/user/two_factor", map[string]string{
		"passcode": passcode,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// We are now user4, and user2 is offered as a switch target.
	assert.Equal(t, user4.Name, signedInUsername(t, session))
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/2']", true)
}

// TestAccountSwitchAddMovesRememberCookie verifies that an existing "remember
// me" cookie follows the account that was just added, even when that sign-in
// did not ask to be remembered, so the browser is not later signed back in as
// the previous account.
func TestAccountSwitchAddMovesRememberCookie(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUserWithPasswordRemember(t, "user2", userPassword, true)
	req := NewRequest(t, "POST", "/user/account_switch/add")
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithValues(t, "POST", "/user/login?add_account=1", map[string]string{
		"user_name": "user1",
		"password":  userPassword,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "user1", signedInUsername(t, session))

	// Once the session is gone, the remember-me cookie signs user1 back in.
	rememberCookie := session.GetCookie(setting.CookieRememberName)
	require.NotNil(t, rememberCookie)
	ltaSession := GetSessionForLTACookie(t, rememberCookie)
	req = NewRequest(t, "GET", "/user/login")
	ltaSession.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "user1", signedInUsername(t, ltaSession))
}

// TestAccountSwitchAddOAuth adds an additional account by authenticating it
// through an OAuth2 login source (as opposed to the internal username/password
// form). It proves the OAuth2 sign-in path records the account into the switcher
// and keeps the previously signed-in account available to switch back to.
func TestAccountSwitchAddOAuth(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// An OAuth2 authentication source (GitLab, which also stands in for a remote
	// Forgejo/Gitea instance) and a user that belongs to it.
	gitlabName := "gitlab"
	gitlab := addAuthSource(t, authSourcePayloadGitLabCustom(gitlabName))

	gitlabUserID := "5678"
	gitlabUser := &user_model.User{
		Name:        "gitlabuser",
		Email:       "gitlabuser@example.com",
		Passwd:      "gitlabuserpassword",
		Type:        user_model.UserTypeIndividual,
		LoginType:   auth.OAuth2,
		LoginSource: gitlab.ID,
		LoginName:   gitlabUserID,
		IsActive:    true,
	}
	defer createUser(t.Context(), t, gitlabUser)()

	// A request to Goth returns a goth.User matching the account above.
	defer mockCompleteUserAuth(func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
		return goth.User{
			Provider: gitlabName,
			UserID:   gitlabUserID,
			Email:    gitlabUser.Email,
		}, nil
	})()

	// Sign in as user2 (the initial account).
	session := loginUser(t, "user2")
	assert.Equal(t, "user2", signedInUsername(t, session))

	// Start adding another account: keeps user2 signed in and sets the add-account
	// flag that must survive the OAuth2 round-trip.
	req := NewRequest(t, "POST", "/user/account_switch/add")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/user/login?add_account=1", jsonRedirect(t, resp))

	// Complete the OAuth2 callback in the still-signed-in session, as the browser
	// would after being redirected back from the provider.
	req = NewRequest(t, "GET", fmt.Sprintf("/user/oauth2/%s/callback?code=XYZ&state=XYZ", gitlabName))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/", test.RedirectURL(resp))

	// We are now the OAuth2 account, and user2 is offered as a switch target.
	assert.Equal(t, gitlabUser.Name, signedInUsername(t, session))
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "a[data-url='/user/account_switch/2']", true)

	// Switching back to user2 works without re-authenticating.
	req = NewRequest(t, "POST", "/user/account_switch/2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/", jsonRedirect(t, resp))
	assert.Equal(t, "user2", signedInUsername(t, session))

	// The OAuth2 account is now a switch target from user2.
	req = NewRequest(t, "GET", "/")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, fmt.Sprintf("a[data-url='/user/account_switch/%d']", gitlabUser.ID), true)
}
