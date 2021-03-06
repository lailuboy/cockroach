// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package settings

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// EncodeDuration encodes a duration in the format parseRaw expects.
func EncodeDuration(d time.Duration) string {
	return d.String()
}

// EncodeBool encodes a bool in the format parseRaw expects.
func EncodeBool(b bool) string {
	return strconv.FormatBool(b)
}

// EncodeInt encodes an int in the format parseRaw expects.
func EncodeInt(i int64) string {
	return strconv.FormatInt(i, 10)
}

// EncodeFloat encodes a bool in the format parseRaw expects.
func EncodeFloat(f float64) string {
	return strconv.FormatFloat(f, 'G', -1, 64)
}

type updater struct {
	sv *Values
	m  map[string]struct{}
}

// Updater is a helper for updating the in-memory settings.
//
// RefreshSettings passes the serialized representations of all individual
// settings -- e.g. the rows read from the system.settings table. We update the
// wrapped atomic settings values as we go and note which settings were updated,
// then set the rest to default in ResetRemaining().
type Updater interface {
	Set(k, rawValue, valType string) error
	ResetRemaining()
}

// A NoopUpdater ignores all updates.
type NoopUpdater struct{}

// Set implements Updater. It is a no-op.
func (u NoopUpdater) Set(_, _, _ string) error { return nil }

// ResetRemaining implements Updater. It is a no-op.
func (u NoopUpdater) ResetRemaining() {}

// NewUpdater makes an Updater.
func NewUpdater(sv *Values) Updater {
	return updater{
		m:  make(map[string]struct{}, len(Registry)),
		sv: sv,
	}
}

// Set attempts to parse and update a setting and notes that it was updated.
func (u updater) Set(key, rawValue string, vt string) error {
	d, ok := Registry[key]
	if !ok {
		if _, ok := retiredSettings[key]; ok {
			return nil
		}
		// Likely a new setting this old node doesn't know about.
		return errors.Errorf("unknown setting '%s'", key)
	}

	u.m[key] = struct{}{}

	if expected := d.Typ(); vt != expected {
		return errors.Errorf("setting '%s' defined as type %s, not %s", key, expected, vt)
	}

	switch setting := d.(type) {
	case *StringSetting:
		return setting.set(u.sv, rawValue)
	case *BoolSetting:
		b, err := strconv.ParseBool(rawValue)
		if err != nil {
			return err
		}
		setting.set(u.sv, b)
		return nil
	case numericSetting: // includes *EnumSetting
		i, err := strconv.Atoi(rawValue)
		if err != nil {
			return err
		}
		return setting.set(u.sv, int64(i))
	case *FloatSetting:
		f, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return err
		}
		return setting.set(u.sv, f)
	case *DurationSetting:
		d, err := time.ParseDuration(rawValue)
		if err != nil {
			return err
		}
		return setting.set(u.sv, d)
	case *StateMachineSetting:
		return setting.set(u.sv, []byte(rawValue))
	}
	return nil
}

// ResetRemaining sets all settings not updated by the updater to their default values.
func (u updater) ResetRemaining() {
	for k, v := range Registry {
		if _, ok := u.m[k]; !ok {
			v.setToDefault(u.sv)
		}
	}
}
