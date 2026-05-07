// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"context"
	"errors"
	"strings"
	"time"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/modules/log"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"

	"connectrpc.com/connect"
)

const (
	uuidHeaderKey       = "x-runner-uuid"
	tokenHeaderKey      = "x-runner-token"
	requestKeyHeaderKey = "x-runner-request-key"
)

var withRunner = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		methodName := getMethodName(request)
		if methodName == "Register" {
			return unaryFunc(ctx, request)
		}
		uuid := request.Header().Get(uuidHeaderKey)
		token := request.Header().Get(tokenHeaderKey)

		runner, err := actions_model.GetRunnerByUUID(ctx, uuid)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unregistered runner"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !runner.VerifyToken(token) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unregistered runner"))
		}

		now := timeutil.TimeStampNow()
		cols := []string{}
		if !runner.LastOnline.AsLocalTime().Truncate(time.Minute).Equal(now.AsLocalTime().Truncate(time.Minute)) {
			runner.LastOnline = now
			cols = append(cols, "last_online")
		}
		if runner.TokenSalt != "" {
			runner.UpdateSecret(runner.Token)
			cols = append(cols, "token_hash", "token_salt")
		}
		if methodName == "UpdateTask" || methodName == "UpdateLog" {
			if !runner.LastActive.AsLocalTime().Truncate(time.Minute).Equal(now.AsLocalTime().Truncate(time.Minute)) {
				runner.LastActive = now
				cols = append(cols, "last_active")
			}
		}
		if err := actions_model.UpdateRunner(ctx, runner, cols...); err != nil {
			log.Error("can't update runner status: %v", err)
		}

		ctx = context.WithValue(ctx, runnerCtxKey{}, runner)

		requestKey := request.Header().Get(requestKeyHeaderKey)
		if requestKey != "" {
			ctx = context.WithValue(ctx, runnerRequestKeyCtxKey{}, requestKey)
		}

		return unaryFunc(ctx, request)
	}
}))

func getMethodName(req connect.AnyRequest) string {
	splits := strings.Split(req.Spec().Procedure, "/")
	if len(splits) > 0 {
		return splits[len(splits)-1]
	}
	return ""
}

type runnerCtxKey struct{}

func GetRunner(ctx context.Context) *actions_model.ActionRunner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*actions_model.ActionRunner); ok {
			return r
		}
	}
	return nil
}

type runnerRequestKeyCtxKey struct{}

func getRequestKey(ctx context.Context) *string {
	if v := ctx.Value(runnerRequestKeyCtxKey{}); v != nil {
		if r, ok := v.(string); ok {
			return &r
		}
	}
	return nil
}
