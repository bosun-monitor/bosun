// Copyright 2017 Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package env

import (
	"os"
	"strconv"
	"time"
)

type envGetter func(string) string

// getter allows mocking os.Getenv in tests.
var getter envGetter = os.Getenv

// String inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func String(defaultValue string, envvars ...string) string {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			return s
		}
	}
	return defaultValue
}

// Int inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Int(defaultValue int, envvars ...string) int {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if i, err := strconv.Atoi(s); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// Int64 inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Int64(defaultValue int64, envvars ...string) int64 {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// Float32 inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Float32(defaultValue float32, envvars ...string) float32 {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if f, err := strconv.ParseFloat(s, 32); err == nil {
				return float32(f)
			}
		}
	}
	return defaultValue
}

// Float64 inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Float64(defaultValue float64, envvars ...string) float64 {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		}
	}
	return defaultValue
}

// Bool inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Bool(defaultValue bool, envvars ...string) bool {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if flag, err := strconv.ParseBool(s); err == nil {
				return flag
			}
		}
	}
	return defaultValue
}

// Time inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Time(defaultValue time.Time, layout string, envvars ...string) time.Time {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if d, err := time.Parse(layout, s); err == nil {
				return d
			}
		}
	}
	return defaultValue
}

// Duration inspects the environment variables specified in envvars.
// If all of these environment variables are empty, it returns defaultValue.
func Duration(defaultValue time.Duration, envvars ...string) time.Duration {
	for _, envvar := range envvars {
		if s := getter(envvar); s != "" {
			if d, err := time.ParseDuration(s); err == nil {
				return d
			}
		}
	}
	return defaultValue
}
