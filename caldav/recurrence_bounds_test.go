package caldav

import (
	"strings"
	"testing"
	"time"
)

func TestRecurrenceExpansionBoundsHistoricalWork(t *testing.T) {
	start := time.Date(1900, 1, 1, 10, 0, 0, 0, time.UTC)
	event := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=DAILY"}
	window := start.AddDate(0, 0, 10001)
	_, err := ExpandRecurrence(event, window, window.Add(24*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "work limit") {
		t.Fatalf("historical iteration must fail with an explicit work limit: %v", err)
	}
}

func TestRecurrenceExpansionRejectsDenseTimeset(t *testing.T) {
	start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	event := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=DAILY;COUNT=1;BYHOUR=0,1,2,3,4,5,6,7,8,9,10,11,12;BYMINUTE=0,30"}
	_, err := ExpandRecurrence(event, start, start.Add(24*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "dense") {
		t.Fatalf("dense timesets must be rejected before expansion: %v", err)
	}
}

func TestRecurrenceExpansionRejectsOversizedSelectors(t *testing.T) {
	start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	event := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=MONTHLY;COUNT=1;BYSETPOS=" + strings.Repeat("1,", 366) + "1"}
	_, err := ExpandRecurrence(event, start, start.Add(24*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "selector") {
		t.Fatalf("oversized selector must be rejected before expansion: %v", err)
	}
}

func TestRecurrenceExpansionPreservesMonthlySetPosition(t *testing.T) {
	start := time.Date(2020, 1, 31, 10, 0, 0, 0, time.UTC)
	event := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1;COUNT=3"}
	events, err := ExpandRecurrence(event, start, start.AddDate(0, 3, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2020-01-31", "2020-02-28", "2020-03-31"}
	if len(events) != len(want) {
		t.Fatalf("got %d occurrences, want %d", len(events), len(want))
	}
	for i, event := range events {
		if got := event.StartTime.Format("2006-01-02"); got != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got, want[i])
		}
	}
}

func TestRecurrenceExpansionRejectsEasterOffsets(t *testing.T) {
	start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	event := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=YEARLY;BYEASTER=1000000"}
	_, err := ExpandRecurrence(event, start, start.AddDate(1, 0, 0))
	if err == nil || !strings.Contains(err.Error(), "BYEASTER") {
		t.Fatalf("unbounded Easter offsets must be rejected before expansion: %v", err)
	}
}

func TestExpandRecurrenceRejectsExtremeStoredInterval(t *testing.T) {
	for _, rule := range []string{"FREQ=MONTHLY;INTERVAL=9223372036854775807;COUNT=1", "FREQ=DAILY;INTERVAL=10001;COUNT=1"} {
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		event := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: rule}
		if _, err := ExpandRecurrence(event, start, start.AddDate(1, 0, 0)); err == nil {
			t.Fatalf("unsafe stored interval accepted: %s", rule)
		}
	}
}
