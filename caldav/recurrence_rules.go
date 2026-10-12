package caldav

import (
	"fmt"
	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
	"strconv"
	"strings"
	"time"
)

// ValidateRecurrence bounds the supported human-scale schedules before any write.
func ValidateRecurrence(rule string, start time.Time) error {
	if rule == "" {
		return nil
	}
	if len(rule) > 512 || strings.ContainsAny(rule, "\r\n\t ") {
		return fmt.Errorf("recurrence must be one RRULE value without whitespace")
	}
	seen := map[string]string{}
	for _, part := range strings.Split(rule, ";") {
		bits := strings.SplitN(part, "=", 2)
		if len(bits) != 2 || bits[1] == "" {
			return fmt.Errorf("invalid recurrence part")
		}
		if _, ok := seen[bits[0]]; ok {
			return fmt.Errorf("duplicate recurrence field %s", bits[0])
		}
		seen[bits[0]] = bits[1]
		switch bits[0] {
		case "FREQ", "INTERVAL", "COUNT", "UNTIL", "BYDAY", "BYMONTHDAY", "BYMONTH", "BYSETPOS", "WKST":
		default:
			return fmt.Errorf("unsupported recurrence field %s", bits[0])
		}
	}
	switch seen["FREQ"] {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	default:
		return fmt.Errorf("recurrence frequency must be DAILY, WEEKLY, MONTHLY or YEARLY")
	}
	for _, key := range []string{"COUNT", "INTERVAL"} {
		if value, ok := seen[key]; ok {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 10000 {
				return fmt.Errorf("%s must be an integer from1 to10000", key)
			}
		}
	}
	if seen["COUNT"] != "" && seen["UNTIL"] != "" {
		return fmt.Errorf("recurrence cannot specify both COUNT and UNTIL")
	}
	opt, err := rrule.StrToROption(rule)
	if err != nil {
		return fmt.Errorf("invalid recurrence: %w", err)
	}
	opt.Dtstart = start
	if !opt.Until.IsZero() && opt.Until.Before(start) {
		return fmt.Errorf("recurrence UNTIL precedes the event start")
	}
	r, err := rrule.NewRRule(*opt)
	if err != nil {
		return err
	}
	first := r.After(start.Truncate(time.Second).Add(-time.Nanosecond), true)
	if !first.Equal(start.Truncate(time.Second)) {
		return fmt.Errorf("event start must match its recurrence pattern")
	}
	return nil
}

func hasRecurrenceExceptions(cal *ical.Calendar) bool {
	count := 0
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		count++
		for _, name := range []string{"RECURRENCE-ID", "EXDATE", "RDATE", "EXRULE"} {
			if len(component.Props[name]) > 0 {
				return true
			}
		}
	}
	return count > 1
}
