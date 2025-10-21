// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
)

func JobParser(workflow []byte, options ...jobparser.ParseOption) ([]*jobparser.SingleWorkflow, error) {
	singleWorkflows, err := jobparser.Parse(workflow, false, options...)
	if err != nil {
		return nil, err
	}
	return singleWorkflows, nil
}
