package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time span written in a mesh file. It accepts s, m, h, d, and y
// as units. One year is 365 days.
type Duration struct {
	time.Duration
	raw string
}

// UnmarshalYAML reads a duration from a mesh file.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := ParseDuration(value.Value)
	if err != nil {
		return err
	}
	d.Duration = parsed
	d.raw = value.Value
	return nil
}

// MarshalYAML writes the duration back in its original form.
func (d Duration) MarshalYAML() (any, error) {
	if d.raw != "" {
		return d.raw, nil
	}
	return FormatDuration(d.Duration), nil
}

// String returns the duration in mesh file form.
func (d Duration) String() string {
	if d.raw != "" {
		return d.raw
	}
	return FormatDuration(d.Duration)
}

// ParseDuration reads a duration such as "20y", "1y", or "60d".
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	unit := s[len(s)-1]
	number := s
	scale := time.Second

	switch unit {
	case 'y':
		scale = 365 * 24 * time.Hour
		number = s[:len(s)-1]
	case 'd':
		scale = 24 * time.Hour
		number = s[:len(s)-1]
	case 'h':
		scale = time.Hour
		number = s[:len(s)-1]
	case 'm':
		scale = time.Minute
		number = s[:len(s)-1]
	case 's':
		scale = time.Second
		number = s[:len(s)-1]
	default:
		return 0, fmt.Errorf("duration %q has no unit, use s, m, h, d, or y", s)
	}

	count, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, fmt.Errorf("duration %q has a bad number: %w", s, err)
	}
	if count <= 0 {
		return 0, fmt.Errorf("duration %q must be more than zero", s)
	}

	return time.Duration(count * float64(scale)), nil
}

// FormatDuration writes a duration in the shortest unit that keeps it exact.
func FormatDuration(d time.Duration) string {
	year := 365 * 24 * time.Hour
	day := 24 * time.Hour
	switch {
	case d%(year) == 0:
		return strconv.Itoa(int(d/year)) + "y"
	case d%day == 0:
		return strconv.Itoa(int(d/day)) + "d"
	case d%time.Hour == 0:
		return strconv.Itoa(int(d/time.Hour)) + "h"
	case d%time.Minute == 0:
		return strconv.Itoa(int(d/time.Minute)) + "m"
	default:
		return strconv.Itoa(int(d/time.Second)) + "s"
	}
}
