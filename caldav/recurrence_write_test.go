package caldav

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRecurrenceCreateAndClock(t *testing.T) {
	c, b := fixtureClient(t)
	var e Event
	if err := json.Unmarshal([]byte(`{"title":"Repeat","startTime":"2020-03-07T14:00:00Z","endTime":"2020-03-07T15:00:00Z","timezone":"America/New_York","recurrence":"FREQ=DAILY;COUNT=3"}`), &e); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateEvent(context.Background(), "/cal/", &e); err != nil {
		t.Fatal(err)
	}
	prop := b.object.Data.Children[0].Props.Get("RRULE")
	if prop == nil || prop.Value != "FREQ=DAILY;COUNT=3" {
		t.Fatal("recurrence not stored")
	}
	start := b.object.Data.Children[0].Props.Get("DTSTART")
	if start.Params.Get("TZID") != "America/New_York" || start.Value != "20200307T090000" {
		t.Fatalf("calendar clock not stored: %+v", start)
	}
	read, err := c.parseCalendarObject(b.object)
	if err != nil {
		t.Fatal(err)
	}
	events, err := ExpandRecurrence(*read, time.Date(2020, 3, 7, 0, 0, 0, 0, time.UTC), time.Date(2020, 3, 10, 0, 0, 0, 0, time.UTC))
	if err != nil || len(events) != 3 {
		t.Fatalf("expand %d %v", len(events), err)
	}
	zone, _ := time.LoadLocation("America/New_York")
	for _, event := range events {
		if event.StartTime.In(zone).Hour() != 9 {
			t.Fatalf("DST changed local hour: %v", event.StartTime)
		}
	}
	if events[1].StartTime.Sub(events[0].StartTime) != 23*time.Hour {
		t.Fatal("DST not reflected in timestamps")
	}
}
func TestRecurrenceUpdateAndClear(t *testing.T) {
	for _, input := range []string{`{"Recurrence":"FREQ=WEEKLY;COUNT=3"}`, `{"Recurrence":""}`} {
		c, b := fixtureClient(t)
		var u EventUpdate
		if err := json.Unmarshal([]byte(input), &u); err != nil {
			t.Fatal(err)
		}
		if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &u); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(u)
		_ = raw
		prop := b.object.Data.Children[0].Props.Get("RRULE")
		if strings.Contains(input, "WEEKLY") {
			if prop == nil || prop.Value != "FREQ=WEEKLY;COUNT=3" {
				t.Fatal("rule not changed")
			}
		} else if prop != nil {
			t.Fatal("rule not cleared")
		}
		if b.object.Data.Children[0].Props.Get("X-KEEP") == nil || len(b.object.Data.Children[0].Children) != 1 {
			t.Fatal("unrelated fields/reminder lost")
		}
	}
}
func TestRecurrenceRejectUnsafeRuleEdit(t *testing.T) {
	c, b := fixtureClient(t)
	b.object.Data.Children[0].Props.SetText("EXDATE", "20200103")
	var u EventUpdate
	if err := json.Unmarshal([]byte(`{"Recurrence":"FREQ=WEEKLY;COUNT=3"}`), &u); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &u); err == nil {
		t.Fatal("exception series edit accepted")
	}
	if b.object.Data.Children[0].Props.Get("RRULE").Value != "FREQ=DAILY;COUNT=2" {
		t.Fatal("rejected write mutated rule")
	}
}
func TestRecurrenceRejectInvalidCreation(t *testing.T) {
	for _, rule := range []string{"FREQ=SECONDLY", "FREQ=DAILY;COUNT=0", "FREQ=DAILY;INTERVAL=0", "FREQ=DAILY;COUNT=3;UNTIL=20250101T000000Z", "FREQ=DAILY;FREQ=WEEKLY", "FREQ=WEEKLY;BYDAY=BOGUS", "FREQ=DAILY\r\nSUMMARY:injected"} {
		c, _ := fixtureClient(t)
		e := Event{Title: "Test", StartTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC), Recurrence: rule}
		if _, err := c.CreateEvent(context.Background(), "/cal/", &e); err == nil {
			t.Errorf("invalid rule accepted: %q", rule)
		}
	}
}
func TestRecurrenceExclusiveWindow(t *testing.T) {
	start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	e := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=DAILY;COUNT=3"}
	events, err := ExpandRecurrence(e, start, start.Add(24*time.Hour))
	if err != nil || len(events) != 1 {
		t.Fatalf("end boundary included: %d %v", len(events), err)
	}
}
func TestRecurrenceUpdatedStartValidated(t *testing.T) {
	c, b := fixtureClient(t)
	b.object.Data.Children[0].Props.Get("DTSTART").SetDateTime(time.Date(2020, 1, 2, 10, 0, 0, 0, time.UTC))
	var u EventUpdate
	if err := json.Unmarshal([]byte(`{"StartTime":"2020-02-01T10:00:00Z","Recurrence":"FREQ=DAILY;UNTIL=20200110T000000Z"}`), &u); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &u); err == nil {
		t.Fatal("UNTIL before effective start accepted")
	}
	if b.object.Data.Children[0].Props.Get("DTSTART").Value != "20200102T100000Z" {
		t.Fatal("rejected combined edit mutated start")
	}
}
func TestRecurrenceStartMatchesRule(t *testing.T) {
	c, _ := fixtureClient(t)
	e := Event{Title: "Mismatch", StartTime: time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC), EndTime: time.Date(2020, 1, 1, 11, 0, 0, 0, time.UTC), Recurrence: "FREQ=WEEKLY;BYDAY=MO"}
	if _, err := c.CreateEvent(context.Background(), "/cal/", &e); err == nil {
		t.Fatal("Wednesday DTSTART accepted for Monday-only rule")
	}
}
func TestRecurrenceExpansionBounded(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	e := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=DAILY;BYHOUR=0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23"}
	if _, err := ExpandRecurrence(e, start, start.AddDate(1, 0, 0)); err == nil {
		t.Fatal("more than5000 occurrences accepted")
	}
}
func TestRecurrenceSubDailyExpansionRejected(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	e := Event{StartTime: start, EndTime: start.Add(time.Hour), Recurrence: "FREQ=HOURLY;COUNT=2"}
	if _, err := ExpandRecurrence(e, start, start.Add(3*time.Hour)); err == nil {
		t.Fatal("sub-daily expansion accepted")
	}
}
func TestRecurrenceMonthlyAndYearly(t *testing.T) {
	for _, tc := range []struct {
		rule  string
		start time.Time
		dates []string
	}{{"FREQ=MONTHLY;COUNT=3", time.Date(2020, 1, 31, 10, 0, 0, 0, time.UTC), []string{"2020-01-31", "2020-03-31", "2020-05-31"}}, {"FREQ=YEARLY;COUNT=3", time.Date(2020, 2, 29, 10, 0, 0, 0, time.UTC), []string{"2020-02-29", "2024-02-29", "2028-02-29"}}} {
		c, b := fixtureClient(t)
		e := Event{Title: "Repeat", StartTime: tc.start, EndTime: tc.start.Add(time.Hour), Recurrence: tc.rule}
		if _, err := c.CreateEvent(context.Background(), "/cal/", &e); err != nil {
			t.Fatal(err)
		}
		stored, err := c.parseCalendarObject(b.object)
		if err != nil {
			t.Fatal(err)
		}
		events, err := ExpandRecurrence(*stored, tc.start, tc.start.AddDate(9, 0, 0))
		if err != nil || len(events) != 3 {
			t.Fatalf("%s %d %v", tc.rule, len(events), err)
		}
		for i, event := range events {
			if event.StartTime.Format("2006-01-02") != tc.dates[i] {
				t.Fatalf("wrong occurrence %v", event.StartTime)
			}
		}
	}
}
