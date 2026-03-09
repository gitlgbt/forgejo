// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package frccaptcha

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"
)

type VerifyOpts struct {
	Secret   string `json:"secret"`
	Solution string `json:"solution"`
	Sitekey  string `json:"sitekey"`
}

// Response is the structure of JSON returned from API
type Response struct {
	Success bool         `json:"success"`
	Errors  *[]ErrorCode `json:"errors"`
}

const verifyURL = "https://api.friendlycaptcha.com/api/v1/siteverify"

// Verify calls Google Recaptcha API to verify token
func Verify(ctx context.Context, response string) (bool, error) {
	post := &VerifyOpts{
		Secret:   setting.Service.FrcCaptchaSecret,
		Solution: response,
		Sitekey:  setting.Service.FrcCaptchaSitekey,
	}
	reqbody, err := json.Marshal(post)
	if err != nil {
		return false, fmt.Errorf("Failed to marshal CAPTCHA request: %w", err)
	}
	// Basically a copy of http.PostForm, but with a context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		verifyURL, bytes.NewReader(reqbody))
	if err != nil {
		return false, fmt.Errorf("Failed to create CAPTCHA request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("Failed to send CAPTCHA response: %s", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("Failed to read CAPTCHA response: %s", err)
	}

	var jsonResponse Response
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return false, fmt.Errorf("Failed to parse CAPTCHA response: %s", err)
	}
	var respErr error
	if jsonResponse.Errors != nil && len(*jsonResponse.Errors) > 0 {
		respErr = (*jsonResponse.Errors)[0]
	}
	return jsonResponse.Success, respErr
}

const (
	ErrSecretMissing   ErrorCode = "secret_missing"
	ErrSecretInvalid   ErrorCode = "secret_invalid"
	ErrSolutionMissing ErrorCode = "solution_missing"
	ErrSolutionInvalid ErrorCode = "solution_invalid"
	ErrBadRequest      ErrorCode = "bad_request"
	ErrSolutionTimeout ErrorCode = "solution_timeout_or_duplicate"
)

// ErrorCode is a reCaptcha error
type ErrorCode string

// String fulfills the Stringer interface
func (e ErrorCode) String() string {
	switch e {
	case ErrSecretMissing:
		return "The secret parameter is missing."
	case ErrSecretInvalid:
		return "The secret parameter is invalid or malformed."
	case ErrSolutionMissing:
		return "The solution parameter is missing."
	case ErrSolutionInvalid:
		return "The solution parameter is invalid or malformed."
	case ErrBadRequest:
		return "The request is invalid or malformed."
	case ErrSolutionTimeout:
		return "The solution is no longer valid: either is too old or has been used previously."
	}
	return string(e)
}

// Error fulfills the error interface
func (e ErrorCode) Error() string {
	return e.String()
}
