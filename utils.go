package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
)

const (
	secondsInMinute = 60
	secondsInHour   = 60 * secondsInMinute
	secondsInDay    = 24 * secondsInHour
	secondsInWeek   = 7 * secondsInDay
	secondsInMonth  = 30 * secondsInDay
	secondsInYear   = 365 * secondsInDay
)

func secondsToTimeString(seconds float64) string {
	s := int(seconds)
	years := s / secondsInYear
	s -= years * secondsInYear
	months := s / secondsInMonth
	s -= months * secondsInMonth
	days := s / secondsInDay
	ts := ""
	if years > 0 {
		ts += fmt.Sprint(years) + "y"
	}
	if months > 0 {
		ts += fmt.Sprint(months) + "m"
	}
	if days > 0 {
		ts += fmt.Sprint(days) + "d"
	}
	if ts == "" {
		ts = "today"
	}
	return ts
}

func secondsSince(dt string) float64 {
	t, _ := time.Parse("2006-01-02T15:04:05Z", dt)
	duration := time.Since(t)

	return duration.Seconds()
}

func daysSince(dt string) int {
	return int(secondsSince(dt)) / secondsInDay
}

func getDate(dt string) string {
	return strings.Split(dt, "T")[0]
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}
