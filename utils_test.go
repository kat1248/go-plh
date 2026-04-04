package main

import (
	"math"
	"testing"
	"time"
)

func TestSecondsToTimeString(t *testing.T) {
	cases := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"zero", 0, "today"},
		{"days only", float64(5 * secondsInDay), "5d"},
		{"months only", float64(secondsInMonth), "1m"},
		{"years months days", float64(secondsInYear + 2*secondsInMonth + 3*secondsInDay), "1y2m3d"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := secondsToTimeString(tc.seconds)
			if got != tc.want {
				t.Fatalf("secondsToTimeString(%v) = %q; want %q", tc.seconds, got, tc.want)
			}
		})
	}
}

func TestSecondsSince_Approx(t *testing.T) {
	now := time.Now().UTC()
	dur := 3*time.Hour + 15*time.Minute + 10*time.Second
	dt := now.Add(-dur).Format("2006-01-02T15:04:05Z")
	sec := secondsSince(dt)
	if math.Abs(sec-dur.Seconds()) > 2.0 {
		t.Fatalf("secondsSince(%s) = %v; want approx %v", dt, sec, dur.Seconds())
	}
}

func TestGetDate(t *testing.T) {
	in := "2020-02-03T12:34:56Z"
	if d := getDate(in); d != "2020-02-03" {
		t.Fatalf("getDate(%q) = %q; want %q", in, d, "2020-02-03")
	}
}
