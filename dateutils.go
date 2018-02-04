package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	SecondsInMinute = 60
	SecondsInHour   = 60 * SecondsInMinute
	SecondsInDay    = 24 * SecondsInHour
	SecondsInMonth  = 30 * SecondsInDay
	SecondsInYear   = 365 * SecondsInDay
)

func secondsToTimeString(seconds float64) string {
	s := int(seconds)
	years := s / SecondsInYear
	s -= years * SecondsInYear
	months := s / SecondsInMonth
	s -= months * SecondsInMonth
	days := s / SecondsInDay
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
	return int(secondsSince(dt)) / 86400
}

func getDate(dt string) string {
	return strings.Split(dt, "T")[0]
}
