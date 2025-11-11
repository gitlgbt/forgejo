// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"forgejo.org/modules/log"
	"forgejo.org/modules/util"
)

// Moderation settings
var Moderation = struct {
	Enabled                bool          `ini:"ENABLED"`
	KeepResolvedReportsFor time.Duration `ini:"KEEP_RESOLVED_REPORTS_FOR"`
	ContentPatternListFile string        `ini:"CONTENT_PATTERN_LIST_FILE"`
}{
	Enabled: false,
}

func loadModerationFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("moderation")
	err := sec.MapTo(&Moderation)
	if err != nil {
		return fmt.Errorf("failed to map Moderation settings: %v", err)
	}

	// keep reports for one week by default. Since time.Duration stops at the unit of an hour
	// we are using the value of 24 (hours) * 7 (days) which gives us the value of 168
	Moderation.KeepResolvedReportsFor, err = sec.Key("KEEP_RESOLVED_REPORTS_FOR").MustDuration(168 * time.Hour)
	if err != nil {
		return fmt.Errorf("failed to parse duration for [moderation].KEEP_RESOLVED_REPORTS_FOR: %w", err)
	}
	// Make sure it exists on startup.
	if Moderation.ContentPatternListFile != "" {
		exists, err := util.IsExist(Moderation.ContentPatternListFile)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("CONTENT_PATTERN_LIST_FILE %q does not exists", Moderation.ContentPatternListFile)
		}
	}
	return nil
}

var (
	contentPatternListMu sync.RWMutex
	contentPatternList   *strings.Replacer
	lastLoaded           time.Time
)

// ContentPatterns returns which patterns content should be searched for.
func ContentPatterns() *strings.Replacer {
	if Moderation.ContentPatternListFile == "" {
		return nil
	}

	// Fast path
	contentPatternListMu.RLock()
	canUseCache := time.Now().Add(-time.Minute).Before(lastLoaded)
	contentPatternListMu.RUnlock()
	if canUseCache {
		return contentPatternList
	}

	// Slow path :(
	contentPatternListMu.Lock()
	defer contentPatternListMu.Unlock()

	f, err := os.Open(Moderation.ContentPatternListFile)
	if err != nil {
		log.Error("Not able to open %q: %v", Moderation.ContentPatternListFile, err)
		return nil
	}
	defer f.Close()

	patternList := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}

		patternList = append(patternList, line, line)
	}

	if scanner.Err() != nil {
		log.Error("Reading file failed: %v", err)
		return nil
	}

	contentPatternList = strings.NewReplacer(patternList...)
	lastLoaded = time.Now()
	return contentPatternList
}

type RememberSecondWriteWriter struct {
	pos        int
	Idx        int
	End        int
	Writecount int
}

func (n *RememberSecondWriteWriter) Write(p []byte) (int, error) {
	n.Writecount++
	if n.Writecount == 2 {
		n.Idx = n.pos
		n.End = n.pos + len(p)
		n.pos += len(p)
		return len(p), io.EOF
	}
	n.pos += len(p)
	return len(p), nil
}

func (n *RememberSecondWriteWriter) WriteString(s string) (int, error) {
	n.Writecount++
	if n.Writecount == 2 {
		n.Idx = n.pos
		n.End = n.pos + len(s)
		n.pos += len(s)
		return len(s), io.EOF
	}
	n.pos += len(s)
	return len(s), nil
}
