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
	// Stored rules bypass the write validator; cap the interval here too so
	// the dependency cannot overflow its month/year arithmetic.
	if rOption.Interval < 0 || rOption.Interval > 10000 {
		return nil, fmt.Errorf("recurrence interval exceeds supported bounds; read the stored series instead")
	}
	if rOption.Freq > rrule.DAILY {
		return nil, fmt.Errorf("sub-daily recurrence expansion is unsupported; read the stored series instead")
	}
	// This nonstandard extension is not bounds-checked by the dependency and
	// arbitrary stored offsets can panic while constructing its day masks.
	if len(rOption.Byeaster) != 0 {
		return nil, fmt.Errorf("BYEASTER recurrence expansion is unsupported; read the stored series instead")
	}
	// Repeated selectors can otherwise multiply the library's per-period
	// filtering and BYSETPOS work without adding useful occurrences.
	for _, field := range []struct{ size, limit int }{
		{len(rOption.Bysetpos), 366}, {len(rOption.Bymonth), 12},
		{len(rOption.Bymonthday), 62}, {len(rOption.Byyearday), 732},
		{len(rOption.Byweekno), 106}, {len(rOption.Byweekday), 749},
	} {
		if field.size > field.limit {
			return nil, fmt.Errorf("recurrence selector limit exceeded; read the stored series instead")
		}
	}
	// NewRRule builds the full Cartesian product of these fields before an
	// iterator is available. Bound that allocation independently of COUNT and
	// the requested window (which may both be very small).
	timesPerDay := 1
	for _, size := range []int{len(rOption.Byhour), len(rOption.Byminute), len(rOption.Bysecond)} {
		if size > 0 {
			if size > 24/timesPerDay {
				return nil, fmt.Errorf("dense recurrence expansion is unsupported; read the stored series instead")
			}
			timesPerDay *= size
		}
	}
	// Stop ordinary lookahead at the requested horizon. The dependency checks
	// UNTIL only when it finds a candidate, so this does not bound its internal
	// scan through empty periods in an impossible or very sparse rule.
	if rOption.Until.IsZero() || rOption.Until.After(rangeEnd) {
		rOption.Until = rangeEnd
	}

	rule, err := rrule.NewRRule(*rOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create recurrence rule: %w", err)
	}

	var occurrences []time.Time
	next := rule.Iterator()
	for work := 0; ; work++ {
		// Charge every iterator call, including occurrences before rangeStart.
		// The output limit alone cannot bound work for an old stored DTSTART.
		// This caps yielded-candidate work, not the dependency's internal scans.
		if work >= 10000 {
			return nil, fmt.Errorf("recurrence expansion work limit exceeded; read the stored series instead")
		}
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
