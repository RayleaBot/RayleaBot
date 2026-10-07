package release

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var isoDate = regexp.MustCompile(`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2}|[0-9]{8}|[0-9]{4}-W[0-9]{2}(?:-[1-7])?|[0-9]{4}W[0-9]{2}[1-7]?)`)
var basicClock = regexp.MustCompile(`^[0-9]{2}(?:[0-9]{2}(?:[0-9]{2})?)?(?:[.,][0-9]+)?$`)
var extendedClock = regexp.MustCompile(`^[0-9]{2}(?::[0-9]{2}(?::[0-9]{2})?)?(?:[.,][0-9]+)?$`)

// Release timestamps retain ISO calendar/week dates, basic/extended clocks and
// timezone offsets, then normalize to UTC seconds as the metadata writer does.
func ParseReleaseTime(value string) (time.Time, error) {
	invalid := errors.New("release timestamp must include a timezone and use ISO 8601")
	value = strings.ReplaceAll(strings.TrimSpace(value), "Z", "+00:00")
	date := isoDate.FindString(value)
	if date == "" || len(value) <= len(date) {
		return time.Time{}, invalid
	}
	compact := strings.ReplaceAll(date, "-", "")
	year, _ := strconv.Atoi(compact[:4])
	if year < 1 {
		return time.Time{}, invalid
	}
	var day time.Time
	if compact[4] == 'W' {
		week, _ := strconv.Atoi(compact[5:7])
		weekday := 1
		if len(compact) == 8 {
			weekday = int(compact[7] - '0')
		}
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		day = jan4.AddDate(0, 0, -(int(jan4.Weekday())+6)%7+(week-1)*7+weekday-1)
		y, w := day.ISOWeek()
		if week < 1 || y != year || w != week {
			return time.Time{}, invalid
		}
	} else {
		month, _ := strconv.Atoi(compact[4:6])
		number, _ := strconv.Atoi(compact[6:8])
		day = time.Date(year, time.Month(month), number, 0, 0, 0, 0, time.UTC)
		if day.Year() != year || int(day.Month()) != month || day.Day() != number {
			return time.Time{}, invalid
		}
	}
	clock := value[len(date):]
	_, separatorBytes := utf8.DecodeRuneInString(clock)
	clock = clock[separatorBytes:]
	offsetIndex := strings.IndexAny(clock, "+-")
	if offsetIndex < 0 {
		return time.Time{}, invalid
	}
	local, offset := clock[:offsetIndex], clock[offsetIndex+1:]
	h, m, s, micro, ok := parseClock(local)
	if !ok || h > 23 || m > 59 || s > 59 {
		return time.Time{}, invalid
	}
	oh, om, os, omicro, ok := parseClock(offset)
	if !ok {
		return time.Time{}, invalid
	}
	seconds := oh*3600 + om*60 + os
	if seconds >= 24*3600 {
		return time.Time{}, invalid
	}
	offsetDuration := time.Duration(seconds)*time.Second + time.Duration(omicro)*time.Microsecond
	if clock[offsetIndex] == '-' {
		offsetDuration = -offsetDuration
	}
	result := day.Add(time.Duration(h*3600+m*60+s)*time.Second + time.Duration(micro)*time.Microsecond - offsetDuration)
	if result.Year() < 1 || result.Year() > 9999 {
		return time.Time{}, invalid
	}
	return result.Truncate(time.Second), nil
}
func parseClock(value string) (hour, minute, second, microsecond int, ok bool) {
	if !basicClock.MatchString(value) && !extendedClock.MatchString(value) {
		return
	}
	whole, fraction, _ := strings.Cut(strings.ReplaceAll(value, ",", "."), ".")
	whole = strings.ReplaceAll(whole, ":", "")
	hour, _ = strconv.Atoi(whole[:2])
	if len(whole) >= 4 {
		minute, _ = strconv.Atoi(whole[2:4])
	}
	if len(whole) == 6 {
		second, _ = strconv.Atoi(whole[4:6])
	}
	if fraction != "" {
		fraction = (fraction + "000000")[:6]
		microsecond, _ = strconv.Atoi(fraction)
	}
	ok = true
	return
}
