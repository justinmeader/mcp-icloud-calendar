package caldav

import (
	"fmt"
	"time"

	"github.com/teambition/rrule-go"
)

// ExpandRecurrence expands a recurrence rule into individual event occurrences
// within the given time range.
func ExpandRecurrence(event Event, rangeStart, rangeEnd time.Time) ([]Event, error) {
	if event.Recurrence == "" {
		return []Event{event}, nil
	}

	if event.HasRecurrenceExceptions {
		return nil, fmt.Errorf("expansion of a series with exceptions is unsupported; read the stored series instead")
	}
	if !rangeEnd.After(rangeStart) || rangeEnd.Sub(rangeStart) > 10*366*24*time.Hour {
		return nil, fmt.Errorf("recurrence expansion requires a positive window of at most10 years")
	}
	rOption, err := rrule.StrToROption(event.Recurrence)
	if err != nil {
		return nil, fmt.Errorf("failed to parse recurrence rule: %w", err)
	}
	// Keeping the original location preserves wall-clock time through DST changes.
	rOption.Dtstart = event.StartTime
	if rOption.Freq > rrule.DAILY {
		return nil, fmt.Errorf("sub-daily recurrence expansion is unsupported; read the stored series instead")
	}

	rule, err := rrule.NewRRule(*rOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create recurrence rule: %w", err)
	}

	var occurrences []time.Time
	next := rule.Iterator()
	for {
		occurrence, ok := next()
		if !ok || !occurrence.Before(rangeEnd) {
			break
		}
		if occurrence.Before(rangeStart) {
			continue
		}
		if len(occurrences) >= 5000 {
			return nil, fmt.Errorf("recurrence expansion exceeds5000 occurrences; use a smaller window")
		}
		occurrences = append(occurrences, occurrence)
	}
	duration := event.EndTime.Sub(event.StartTime)

	events := make([]Event, 0, len(occurrences))
	for _, occ := range occurrences {
		e := event
		e.StartTime = occ
		e.EndTime = occ.Add(duration)
		if e.AllDay {
			e.StartDate = e.StartTime.Format("20060102")
			e.EndDate = e.EndTime.Format("20060102")
		}

		events = append(events, e)
	}

	return events, nil
}
