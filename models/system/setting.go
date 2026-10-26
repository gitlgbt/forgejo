// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting/config"
	"forgejo.org/modules/timeutil"

	"xorm.io/builder"
)

type Setting struct {
	ID           int64  `xorm:"pk autoincr"`
	SettingKey   string `xorm:"varchar(255) unique"` // key should be lowercase
	SettingValue string `xorm:"text"`
	Version      int
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "system_setting"
}

func init() {
	db.RegisterModel(new(Setting))
}

const keyRevision = "revision"

func GetRevision(ctx context.Context) int {
	revision, exist, err := db.Get[Setting](ctx, builder.Eq{"setting_key": keyRevision})
	if err != nil {
		return 0
	} else if !exist {
		err = db.Insert(ctx, &Setting{SettingKey: keyRevision, Version: 1})
		if err != nil {
			return 0
		}
		return 1
	}
	if revision.Version <= 0 || revision.Version >= math.MaxInt-1 {
		_, err = db.Exec(ctx, "UPDATE system_setting SET version=1 WHERE setting_key=?", keyRevision)
		if err != nil {
			return 0
		}
		return 1
	}
	return revision.Version
}

func GetAllSettings(ctx context.Context) (revision int, res map[string]string, err error) {
	_ = GetRevision(ctx) // prepare the "revision" key ahead
	var settings []*Setting
	if err := db.GetEngine(ctx).
		Find(&settings); err != nil {
		return 0, nil, err
	}
	res = make(map[string]string)
	for _, s := range settings {
		if s.SettingKey == keyRevision {
			revision = s.Version
		}
		res[s.SettingKey] = s.SettingValue
	}
	return revision, res, nil
}

// bumpSettingsRevision also serializes setting writers on the shared revision row.
// Call only within a transaction, after preparing the row with GetRevision.
func bumpSettingsRevision(ctx context.Context) error {
	_, err := db.Exec(ctx, "UPDATE system_setting SET version=version+1 WHERE setting_key=?", keyRevision)
	return err
}

func upsertSetting(ctx context.Context, key, value string) error {
	res, err := db.Exec(ctx, "UPDATE system_setting SET version=version+1, setting_value=? WHERE setting_key=?", value, key)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return db.Insert(ctx, &Setting{SettingKey: key, SettingValue: value})
	}
	return nil
}

func SetSettings(ctx context.Context, settings map[string]string) error {
	_ = GetRevision(ctx)
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := bumpSettingsRevision(ctx); err != nil {
			return err
		}
		for key, value := range settings {
			if err := upsertSetting(ctx, key, value); err != nil {
				return err
			}
		}
		return nil
	})
}

var ErrSettingChanged = errors.New("setting changed since the form was opened")

// SetSettingIfVersion writes a setting only if its version still matches.
// A version of -1 means the setting must not yet exist.
func SetSettingIfVersion(ctx context.Context, key string, version int, value string) error {
	_ = GetRevision(ctx)
	return db.WithTx(ctx, func(ctx context.Context) error {
		// Taking the revision write lock before reading also serializes concurrent first saves.
		if err := bumpSettingsRevision(ctx); err != nil {
			return err
		}
		row, exists, err := db.Get[Setting](ctx, builder.Eq{"setting_key": key})
		if err != nil {
			return err
		}
		if (!exists && version != -1) || (exists && row.Version != version) {
			return ErrSettingChanged
		}
		return upsertSetting(ctx, key, value)
	})
}

type dbConfigCachedGetter struct {
	mu sync.RWMutex

	cacheTime time.Time
	revision  int
	settings  map[string]string
}

var _ config.DynKeyGetter = (*dbConfigCachedGetter)(nil)

func (d *dbConfigCachedGetter) GetValue(ctx context.Context, key string) (v string, has bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, has = d.settings[key]
	return v, has
}

func (d *dbConfigCachedGetter) GetRevision(ctx context.Context) int {
	d.mu.RLock()
	cachedDuration := time.Since(d.cacheTime)
	cachedRevision := d.revision
	d.mu.RUnlock()

	if cachedDuration < time.Second {
		return cachedRevision
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if GetRevision(ctx) != d.revision {
		rev, set, err := GetAllSettings(ctx)
		if err != nil {
			log.Error("Unable to get all settings: %v", err)
		} else {
			d.revision = rev
			d.settings = set
		}
	}
	d.cacheTime = time.Now()
	return d.revision
}

func (d *dbConfigCachedGetter) InvalidateCache() {
	d.mu.Lock()
	d.cacheTime = time.Time{}
	d.mu.Unlock()
}

func NewDatabaseDynKeyGetter() config.DynKeyGetter {
	return &dbConfigCachedGetter{}
}
