// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/gob"
	"errors"
	"fmt"
	"slices"
	"strings"

	user_model "forgejo.org/models/user"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/web/middleware"
	"forgejo.org/services/context"
	user_service "forgejo.org/services/user"
)

const (
	// sessionKeySwitchAccounts stores the list of user IDs ([]int64) that the
	// current browser session is allowed to switch between. The currently
	// signed-in user is always part of this list.
	sessionKeySwitchAccounts = "switch_accounts"

	// sessionKeySwitchAdd is a transient flag (bool) that is set while the user
	// is authenticating an additional account to add to the switcher. It allows
	// the still-signed-in user to reach the sign-in flow.
	sessionKeySwitchAdd = "switch_add"
)

func init() {
	// The session backends (db, redis) serialize values with encoding/gob, so
	// the concrete type stored behind the "switch_accounts" key must be known.
	gob.Register([]int64{})
}

// getSwitchAccountIDs returns the user IDs the current session may switch
// between, including the currently signed-in user.
func getSwitchAccountIDs(ctx *context.Context) []int64 {
	ids, _ := ctx.Session.Get(sessionKeySwitchAccounts).([]int64)
	return ids
}

// addSwitchAccountID returns ids with id appended if it is not already present.
func addSwitchAccountID(ids []int64, id int64) []int64 {
	if slices.Contains(ids, id) {
		return ids
	}
	return append(slices.Clone(ids), id)
}

// switchAccountAddParam is the query parameter that carries the "add an account"
// intent through the sign-in flow. It is used in addition to the session flag so
// the flow does not depend on a session write surviving the redirect to the
// login page (which can be unreliable across multiple backend nodes or session
// backends).
const switchAccountAddParam = "add_account"

// IsSwitchAccountAddMode reports whether the request is authenticating an
// additional account. In this mode a signed-in user is intentionally allowed to
// reach the sign-in routes that otherwise require being signed out.
//
// The intent is detected from the URL query first (which is carried explicitly
// through the sign-in flow) and falls back to the session flag, which covers the
// steps that have no query parameter, such as the 2FA continuation. Only the URL
// query is inspected (not the POST body) so this is safe to call from middleware
// without consuming the request body.
func IsSwitchAccountAddMode(ctx *context.Context) bool {
	if ctx.Req != nil && ctx.Req.URL.Query().Get(switchAccountAddParam) == "1" {
		return true
	}
	if ctx.Session == nil {
		return false
	}
	add, _ := ctx.Session.Get(sessionKeySwitchAdd).(bool)
	return add
}

// switchAccountSignInLink returns the sign-in form action, carrying the
// add-account intent as a query parameter so the form submission (and its error
// re-renders) stays in add mode without relying on the session.
func switchAccountSignInLink(addAccount bool) string {
	link := setting.AppSubURL + "/user/login"
	if addAccount {
		link += "?" + switchAccountAddParam + "=1"
	}
	return link
}

// LoadSwitchAccounts populates the template data with a lazily-evaluated list of
// the other accounts the current session can switch to. It is used by the
// navbar user dropdown.
func LoadSwitchAccounts(ctx *context.Context) {
	if strings.HasPrefix(ctx.Req.URL.Path, "/api") {
		return
	}
	if !ctx.IsSigned {
		return
	}

	ids := getSwitchAccountIDs(ctx)
	// SwitchAccounts is called from head_navbar.tmpl. Resolve the users lazily so
	// non-HTML requests never hit the database.
	ctx.Data["SwitchAccounts"] = func() []*user_model.User {
		otherIDs := make([]int64, 0, len(ids))
		for _, id := range ids {
			if id != ctx.Doer.ID {
				otherIDs = append(otherIDs, id)
			}
		}
		if len(otherIDs) == 0 {
			return nil
		}
		users, err := user_model.GetUsersByIDs(ctx, otherIDs)
		if err != nil {
			log.Error("Unable to load switch accounts for user %-v: %v", ctx.Doer, err)
			return nil
		}
		return users
	}
}

// beginAddAccount remembers the current account in the switcher and marks the
// session as adding another account, so the sign-in routes accept a signed-in
// user.
func beginAddAccount(ctx *context.Context) error {
	ids := addSwitchAccountID(getSwitchAccountIDs(ctx), ctx.Doer.ID)
	if err := ctx.Session.Set(sessionKeySwitchAccounts, ids); err != nil {
		return fmt.Errorf("Session.Set: %w", err)
	}
	if err := ctx.Session.Set(sessionKeySwitchAdd, true); err != nil {
		return fmt.Errorf("Session.Set: %w", err)
	}
	if err := ctx.Session.Release(); err != nil {
		return fmt.Errorf("Session.Release: %w", err)
	}
	return nil
}

// AccountSwitchAdd remembers the current account and redirects to the sign-in
// page so the user can authenticate an additional account.
func AccountSwitchAdd(ctx *context.Context) {
	if err := beginAddAccount(ctx); err != nil {
		ctx.ServerError("beginAddAccount", err)
		return
	}
	// A redirect_to left behind by an abandoned select_account flow must not
	// send this sign-in to the OAuth client.
	middleware.DeleteRedirectToCookie(ctx.Resp)
	// The query parameter carries the intent even if the session flag above does
	// not survive the redirect (e.g. across backend nodes).
	ctx.JSONRedirect(setting.AppSubURL + "/user/login?" + switchAccountAddParam + "=1")
}

// moveRememberCookie keeps an existing long-term "remember me" cookie pointing
// at u, the account that is becoming active. Without this the cookie would
// later sign the browser back in as whichever account set it.
func moveRememberCookie(ctx *context.Context, u *user_model.User) error {
	if ctx.GetSiteCookie(setting.CookieRememberName) == "" {
		return nil
	}
	if err := ctx.SetLTACookie(u); err != nil {
		return fmt.Errorf("SetLTACookie: %w", err)
	}
	return nil
}

// errSwitchAccountProhibited is returned by switchToAccount for accounts that
// may not sign in.
var errSwitchAccountProhibited = errors.New("account is not allowed to sign in")

// switchToAccount makes u the signed-in account of the session, keeping ids as
// the session's switcher list. The caller must have checked that u is in ids.
func switchToAccount(ctx *context.Context, u *user_model.User, ids []int64) error {
	if !u.IsActive || u.ProhibitLogin {
		return errSwitchAccountProhibited
	}

	if err := updateSession(ctx, []string{sessionKeySwitchAdd}, map[string]any{
		"uid":                    u.ID,
		sessionKeySwitchAccounts: ids,
	}); err != nil {
		return fmt.Errorf("updateSession: %w", err)
	}

	if err := moveRememberCookie(ctx, u); err != nil {
		return err
	}

	if err := user_service.UpdateUser(ctx, u, &user_service.UpdateOptions{SetLastLogin: true}); err != nil {
		return fmt.Errorf("UpdateUser: %w", err)
	}

	if err := resetLocale(ctx, u); err != nil {
		return fmt.Errorf("resetLocale: %w", err)
	}
	return nil
}

// AccountSwitchTo switches the session to another account that this browser has
// already authenticated during the session.
func AccountSwitchTo(ctx *context.Context) {
	targetID := ctx.ParamsInt64("uid")
	ids := getSwitchAccountIDs(ctx)

	// Only accounts previously authenticated in this session (tracked
	// server-side) may be switched to; this cannot be forged by the client.
	if targetID == ctx.Doer.ID || !slices.Contains(ids, targetID) {
		ctx.JSONRedirect(setting.AppSubURL + "/")
		return
	}

	u, err := user_model.GetUserByID(ctx, targetID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			// The account no longer exists: drop it from the switcher.
			removeSwitchAccount(ctx, ids, targetID)
			ctx.JSONRedirect(setting.AppSubURL + "/")
			return
		}
		ctx.ServerError("GetUserByID", err)
		return
	}

	if err := switchToAccount(ctx, u, ids); err != nil {
		if errors.Is(err, errSwitchAccountProhibited) {
			ctx.Flash.Error(ctx.Tr("auth.prohibit_login"))
			ctx.JSONRedirect("")
			return
		}
		ctx.ServerError("switchToAccount", err)
		return
	}

	ctx.JSONRedirect(setting.AppSubURL + "/")
}

// AccountSwitchRemove drops another account from the switcher. The currently
// signed-in account cannot be removed this way; use sign out instead.
func AccountSwitchRemove(ctx *context.Context) {
	targetID := ctx.ParamsInt64("uid")
	if targetID == ctx.Doer.ID {
		ctx.JSONRedirect(setting.AppSubURL + "/")
		return
	}
	removeSwitchAccount(ctx, getSwitchAccountIDs(ctx), targetID)
	ctx.JSONRedirect(setting.AppSubURL + "/")
}

// removeSwitchAccount removes id from the session's switch-account list without
// regenerating the session (the signed-in identity does not change).
func removeSwitchAccount(ctx *context.Context, ids []int64, id int64) {
	newIDs := slices.DeleteFunc(slices.Clone(ids), func(v int64) bool { return v == id })
	if err := ctx.Session.Set(sessionKeySwitchAccounts, newIDs); err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		ctx.ServerError("Session.Release", err)
		return
	}
}
