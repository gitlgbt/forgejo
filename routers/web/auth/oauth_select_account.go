// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	auth_model "forgejo.org/models/auth"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/modules/httplib"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/web/middleware"
	"forgejo.org/services/context"
	"forgejo.org/services/forms"
)

const tplSelectAccount base.TplName = "user/auth/oauth_select_account"

// promptSelectAccount reports whether the OpenID Connect prompt parameter asks
// the user to choose an account (OpenID Connect Core 1.0, section 3.1.2.1).
func promptSelectAccount(prompt string) bool {
	return slices.Contains(strings.Fields(prompt), "select_account")
}

// authorizeURLWithoutPrompt rebuilds the authorize request from the bound form,
// without the prompt, so the chooser can resume the normal authorize flow. It
// uses the form rather than the request URL so POST requests work too.
func authorizeURLWithoutPrompt(form *forms.AuthorizationForm) string {
	q := url.Values{}
	for k, v := range map[string]string{
		"response_type":         form.ResponseType,
		"client_id":             form.ClientID,
		"redirect_uri":          form.RedirectURI,
		"state":                 form.State,
		"scope":                 form.Scope,
		"nonce":                 form.Nonce,
		"code_challenge_method": form.CodeChallengeMethod,
		"code_challenge":        form.CodeChallenge,
	} {
		if v != "" {
			q.Set(k, v)
		}
	}
	return setting.AppSubURL + "/login/oauth/authorize?" + q.Encode()
}

// renderSelectAccount shows the accounts of this session the user can
// authorize app as: the signed-in account first, then the other accounts in
// the switcher.
func renderSelectAccount(ctx *context.Context, app *auth_model.OAuth2Application, form *forms.AuthorizationForm) {
	accounts := []*user_model.User{ctx.Doer}
	otherIDs := slices.DeleteFunc(slices.Clone(getSwitchAccountIDs(ctx)), func(id int64) bool {
		return id == ctx.Doer.ID
	})
	if len(otherIDs) > 0 {
		others, err := user_model.GetUsersByIDs(ctx, otherIDs)
		if err != nil {
			ctx.ServerError("GetUsersByIDs", err)
			return
		}
		// Accounts that can no longer sign in would only produce an error when picked.
		others = slices.DeleteFunc(others, func(u *user_model.User) bool {
			return !u.IsActive || u.ProhibitLogin
		})
		accounts = append(accounts, others...)
	}

	// Cancel posts to the grant endpoint, which checks these session values,
	// exactly as it does for the grant page.
	for k, v := range map[string]string{
		"client_id":    app.ClientID,
		"redirect_uri": form.RedirectURI,
		"state":        form.State,
	} {
		if err := ctx.Session.Set(k, v); err != nil {
			handleServerError(ctx, form.State, form.RedirectURI, err)
			log.Error(err.Error())
			return
		}
	}
	// Try to release the session early, as the grant page does.
	if err := ctx.Session.Release(); err != nil {
		log.Error("Unable to save changes to the session: %v", err)
	}

	ctx.Data["Title"] = ctx.Tr("auth.oauth_select_account_title", app.Name)
	ctx.Data["Application"] = app
	ctx.Data["Accounts"] = accounts
	ctx.Data["RedirectURI"] = form.RedirectURI
	ctx.Data["State"] = form.State
	ctx.Data["Scope"] = form.Scope
	ctx.Data["Nonce"] = form.Nonce
	ctx.Data["RedirectTo"] = authorizeURLWithoutPrompt(form)
	ctx.HTML(http.StatusOK, tplSelectAccount)
}

// isAuthorizeRedirect reports whether redirectTo is a local URL of the OAuth2
// authorize endpoint, the only place the chooser may send the browser back to.
func isAuthorizeRedirect(redirectTo string) bool {
	if redirectTo == "" || httplib.IsRiskyRedirectURL(redirectTo) {
		return false
	}
	u, err := url.Parse(redirectTo)
	if err != nil {
		return false
	}
	return u.Scheme == "" && u.Host == "" && u.Path == setting.AppSubURL+"/login/oauth/authorize"
}

// selectAccountURL returns redirectTo with prompt=select_account set again, to
// show the chooser once more.
func selectAccountURL(redirectTo string) string {
	u, _ := url.Parse(redirectTo) // validated by isAuthorizeRedirect
	q := u.Query()
	q.Set("prompt", "select_account")
	u.RawQuery = q.Encode()
	return u.String()
}

// SelectAccountOAuth handles a choice made on the prompt=select_account
// chooser: continue as an account of this session, or sign in another one.
func SelectAccountOAuth(ctx *context.Context) {
	redirectTo := ctx.FormString("redirect_to")
	if !isAuthorizeRedirect(redirectTo) {
		ctx.Error(http.StatusBadRequest)
		return
	}

	if ctx.FormString("add") == "1" {
		if err := beginAddAccount(ctx); err != nil {
			ctx.ServerError("beginAddAccount", err)
			return
		}
		// The sign-in handlers send the browser here once the account is added.
		middleware.SetRedirectToCookie(ctx.Resp, redirectTo)
		ctx.Redirect(setting.AppSubURL + "/user/login?" + switchAccountAddParam + "=1")
		return
	}

	targetID := ctx.FormInt64("uid")
	if targetID == ctx.Doer.ID {
		ctx.Redirect(redirectTo)
		return
	}

	// Only accounts previously authenticated in this session may be chosen.
	ids := getSwitchAccountIDs(ctx)
	if !slices.Contains(ids, targetID) {
		ctx.Error(http.StatusBadRequest)
		return
	}

	u, err := user_model.GetUserByID(ctx, targetID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusBadRequest)
			return
		}
		ctx.ServerError("GetUserByID", err)
		return
	}

	if err := switchToAccount(ctx, u, ids); err != nil {
		if errors.Is(err, errSwitchAccountProhibited) {
			ctx.Flash.Error(ctx.Tr("auth.prohibit_login"))
			ctx.Redirect(selectAccountURL(redirectTo))
			return
		}
		ctx.ServerError("switchToAccount", err)
		return
	}

	ctx.Redirect(redirectTo)
}
