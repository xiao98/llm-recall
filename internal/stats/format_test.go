package stats

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{-1, "—"},
		{0, "—"},
		{30 * time.Second, "<1m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h 0m"},
		{1*time.Hour + 5*time.Minute, "1h 5m"},
		{23*time.Hour + 59*time.Minute, "23h 59m"},
		{24 * time.Hour, "1d 0h 0m"},
		{24*time.Hour + 30*time.Minute, "1d 0h 30m"},
		{11*24*time.Hour + 2*time.Hour + 36*time.Minute, "11d 2h 36m"},
		{365 * 24 * time.Hour, "365d 0h 0m"},
	}
	for _, c := range cases {
		got := FormatDuration(c.in)
		if got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatDate(t *testing.T) {
	if got := FormatDate(time.Time{}); got != "—" {
		t.Errorf("zero date = %q, want —", got)
	}
	d := time.Date(2026, time.April, 27, 0, 0, 0, 0, time.UTC)
	if got := FormatDate(d); got != "Apr 27" {
		t.Errorf("FormatDate = %q, want Apr 27", got)
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{-1, "—"},
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1_000, "1k"},
		{1_500, "1.5k"},
		{14_500_000, "14.5m"},
		{2_000_000, "2m"},
		{1_000_000_000, "1b"},
	}
	for _, c := range cases {
		got := FormatTokens(c.in)
		if got != c.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatStreak(t *testing.T) {
	if got := FormatStreak(0); got != "0 days" {
		t.Errorf("0 = %q", got)
	}
	if got := FormatStreak(1); got != "1 day" {
		t.Errorf("1 = %q", got)
	}
	if got := FormatStreak(8); got != "8 days" {
		t.Errorf("8 = %q", got)
	}
}

func TestFormatActiveDays(t *testing.T) {
	if got := FormatActiveDays(23, 79); got != "23/79" {
		t.Errorf("got %q", got)
	}
	if got := FormatActiveDays(5, 0); got != "1/1" {
		t.Errorf("zero total handled? got %q", got)
	}
	if got := FormatActiveDays(50, 30); got != "30/30" {
		t.Errorf("clamping: got %q", got)
	}
}
