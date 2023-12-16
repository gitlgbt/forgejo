// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArchiveType archive types
type ArchiveType int

const (
	// ZIP zip archive type
	ZIP ArchiveType = iota + 1
	// TARGZ tar gz archive type
	TARGZ
	// BUNDLE bundle archive type
	BUNDLE
)

// String converts an ArchiveType to string
func (a ArchiveType) String() string {
	switch a {
	case ZIP:
		return "zip"
	case TARGZ:
		return "tar.gz"
	case BUNDLE:
		return "bundle"
	}
	return "unknown"
}

func ToArchiveType(s string) ArchiveType {
	switch s {
	case "zip":
		return ZIP
	case "tar.gz":
		return TARGZ
	case "bundle":
		return BUNDLE
	}
	return 0
}

var codebergSwitchedToNewChecksumAlgorithm = time.Date(2024, time.January, 12, 23, 0o0, 0o0, 0o0, time.UTC)

// CreateArchive create archive content to the target path
func (repo *Repository) CreateArchive(ctx context.Context, format ArchiveType, target io.Writer, usePrefix bool, commitID string) error {
	if format.String() == "unknown" {
		return fmt.Errorf("unknown format: %v", format)
	}

	cmd := NewCommandContextNoGlobals(ctx)
	if format == TARGZ {
		commit, err := repo.GetCommit(commitID)
		if err != nil && !IsErrNotExist(err) {
			return fmt.Errorf("repo.GetCommit: %v", err)
		}
		if !IsErrNotExist(err) && commit.Author.When.Before(codebergSwitchedToNewChecksumAlgorithm) {
			cmd.AddOptionValues("-c", "tar.tar.gz.command=gzip -cn")
		}
	}

	cmd.AddArguments("archive")
	if usePrefix {
		cmd.AddOptionFormat("--prefix=%s", filepath.Base(strings.TrimSuffix(repo.Path, ".git"))+"/")
	}
	cmd.AddOptionFormat("--format=%s", format.String())
	cmd.AddDynamicArguments(commitID)

	// Avoid LFS hooks getting installed because of /etc/gitconfig, which can break pull requests.
	env := append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")

	var stderr strings.Builder
	err := cmd.Run(&RunOpts{
		Dir:    repo.Path,
		Stdout: target,
		Stderr: &stderr,
		Env:    env,
	})
	if err != nil {
		return ConcatenateError(err, stderr.String())
	}
	return nil
}
