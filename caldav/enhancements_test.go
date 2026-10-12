package caldav

import (
	"context"
	"encoding/json"
	"github.com/emersion/go-ical"
	ext "github.com/emersion/go-webdav/caldav"
	"strings"
	"testing"
	"time"
)

const richFixture = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//fixture//EN\r\nBEGIN:VEVENT\r\nUID:fixture\r\nDTSTAMP:20200101T000000Z\r\nDTSTART;VALUE=DATE:20200102\r\nDTEND;VALUE=DATE:20200103\r\nSUMMARY:Fixture\r\nURL:https://example.com/event\r\nATTACH:https://example.com/file.pdf\r\nSTATUS:CONFIRMED\r\nTRANSP:TRANSPARENT\r\nRRULE:FREQ=DAILY;COUNT=2\r\nX-KEEP:unchanged\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER;RELATED=END:-PT15M\r\nDESCRIPTION:Fixture reminder\r\nX-ALARM-KEEP:unchanged\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

type memoryBackend struct {
	backend
	object *ext.CalendarObject
}

func (b *memoryBackend) GetCalendarObject(context.Context, string) (*ext.CalendarObject, error) {
	return b.object, nil
}
func (b *memoryBackend) PutCalendarObject(_ context.Context, path string, c *ical.Calendar) (*ext.CalendarObject, error) {
	var encoded strings.Builder
	if err := ical.NewEncoder(&encoded).Encode(c); err != nil {
		return nil, err
	}
	decoded, err := ical.NewDecoder(strings.NewReader(encoded.String())).Decode()
	b.object = &ext.CalendarObject{Path: path, Data: decoded}
	return b.object, err
}
func fixtureClient(t *testing.T) (*Client, *memoryBackend) {
	t.Helper()
	cal, err := ical.NewDecoder(strings.NewReader(richFixture)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	b := &memoryBackend{object: &ext.CalendarObject{Path: "/cal/fixture.ics", Data: cal}}
	return NewClientWithBackend(b), b
}
func eventJSON(t *testing.T, c *Client, b *memoryBackend) map[string]interface{} {
	t.Helper()
	e, err := c.parseCalendarObject(b.object)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestEnhancementReadFields(t *testing.T) {
	c, b := fixtureClient(t)
	v := eventJSON(t, c, b)
	for k, w := range map[string]interface{}{"allDay": true, "url": "https://example.com/event", "status": "CONFIRMED", "transparency": "TRANSPARENT"} {
		if v[k] != w {
			t.Errorf("%s=%v want %v", k, v[k], w)
		}
	}
	a, ok := v["alerts"].([]interface{})
	if !ok || len(a) != 1 {
		t.Fatalf("alerts missing: %v", v["alerts"])
	}
	alert := a[0].(map[string]interface{})
	if alert["trigger"] != "-PT15M" || alert["related"] != "END" {
		t.Fatalf("wrong alert: %v", alert)
	}
	if a, ok := v["attachments"].([]interface{}); !ok || len(a) != 1 {
		t.Fatalf("attachments absent: %v", v["attachments"])
	}
}
func TestEnhancementCreateAlert(t *testing.T) {
	c, b := fixtureClient(t)
	var e Event
	if err := json.Unmarshal([]byte(`{"title":"New","startTime":"2020-01-02T10:00:00Z","endTime":"2020-01-02T11:00:00Z","alerts":[{"action":"DISPLAY","trigger":"-PT10M","description":"Reminder"}]}`), &e); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateEvent(context.Background(), "/cal/", &e); err != nil {
		t.Fatal(err)
	}
	v := eventJSON(t, c, b)
	a, ok := v["alerts"].([]interface{})
	if !ok || len(a) != 1 {
		t.Fatalf("created alert absent: %v", v["alerts"])
	}
}
func TestEnhancementDirectRead(t *testing.T) {
	c, _ := fixtureClient(t)
	reader, ok := interface{}(c).(interface {
		GetEvent(context.Context, string) (*Event, error)
	})
	if !ok {
		t.Fatal("direct event read unavailable")
	}
	e, err := reader.GetEvent(context.Background(), "/cal/fixture.ics")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(e)
	if !strings.Contains(string(raw), `"trigger":"-PT15M"`) {
		t.Fatal("direct read lost alert")
	}
}
func TestEnhancementExpandedFieldsPreserved(t *testing.T) {
	c, b := fixtureClient(t)
	e, err := c.parseCalendarObject(b.object)
	if err != nil {
		t.Fatal(err)
	}
	e.StartTime = time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	e.EndTime = e.StartTime.Add(24 * time.Hour)
	events, err := ExpandRecurrence(*e, e.StartTime, e.StartTime.Add(72*time.Hour))
	if err != nil || len(events) != 2 {
		t.Fatalf("expand %v %d", err, len(events))
	}
	raw, _ := json.Marshal(events[0])
	if !strings.Contains(string(raw), `"trigger":"-PT15M"`) || !strings.Contains(string(raw), `"allDay":true`) {
		t.Fatal("expanded recurrence lost alert/date fields")
	}
}
func TestEnhancementMasterSelectedBeforeDetached(t *testing.T) {
	c, b := fixtureClient(t)
	detached := ical.NewEvent()
	detached.Props.SetText("UID", "fixture")
	detached.Props.SetDateTime("DTSTAMP", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	detached.Props.SetDateTime("DTSTART", time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC))
	detached.Props.SetText("RECURRENCE-ID", "20200103")
	detached.Props.SetText("SUMMARY", "Detached")
	b.object.Data.Children = append([]*ical.Component{detached.Component}, b.object.Data.Children...)
	e, err := c.parseCalendarObject(b.object)
	if err != nil {
		t.Fatal(err)
	}
	if e.Title != "Fixture" {
		t.Fatalf("read selected detached component: %s", e.Title)
	}
	title := "Changed"
	if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &EventUpdate{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if b.object.Data.Children[0].Props.Get("SUMMARY").Value != "Detached" {
		t.Fatal("master edit changed detached occurrence")
	}
}
func TestEnhancementRejectImplicitDateTypeConversion(t *testing.T) {
	for _, kind := range []string{"all-day", "floating", "named-zone"} {
		t.Run(kind, func(t *testing.T) {
			c, b := fixtureClient(t)
			p := b.object.Data.Children[0].Props.Get("DTSTART")
			if kind != "all-day" {
				p.Value = "20200102T100000"
				p.Params = ical.Params{}
				if kind == "named-zone" {
					p.Params.Set("TZID", "America/New_York")
				}
			}
			original := p.Value
			start := time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC)
			if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &EventUpdate{StartTime: &start}); err == nil {
				t.Fatal("implicit conversion accepted")
			}
			if b.object.Data.Children[0].Props.Get("DTSTART").Value != original {
				t.Fatal("rejected edit mutated date")
			}
		})
	}
}
func TestEnhancementUpdateAlertSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        int
	}{{"omitted", `{"Title":"Changed"}`, 1}, {"clear", `{"Alerts":[]}`, 0}, {"replace", `{"Alerts":[{"action":"DISPLAY","trigger":"-PT5M","description":"New"}]}`, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			c, b := fixtureClient(t)
			var u EventUpdate
			if err := json.Unmarshal([]byte(tc.input), &u); err != nil {
				t.Fatal(err)
			}
			if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &u); err != nil {
				t.Fatal(err)
			}
			child := b.object.Data.Children[0]
			n := 0
			for _, a := range child.Children {
				if a.Name == "VALARM" {
					n++
				}
			}
			if n != tc.want {
				t.Fatalf("alarm count %d want %d", n, tc.want)
			}
			if child.Props.Get("X-KEEP").Value != "unchanged" || child.Props.Get("RRULE").Value != "FREQ=DAILY;COUNT=2" || child.Props.Get("ATTACH") == nil {
				t.Fatal("unrelated fields lost")
			}
			if tc.name == "omitted" && child.Children[0].Props.Get("X-ALARM-KEEP") == nil {
				t.Fatal("unknown alarm property lost")
			}
			if tc.name == "replace" && child.Children[0].Props.Get("TRIGGER").Value != "-PT5M" {
				t.Fatal("alert not replaced")
			}
		})
	}
}
