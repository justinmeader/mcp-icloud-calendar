package caldav

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func TestUpdateEventRejectsStartChangesWithExceptions(t *testing.T) {
	for _, exception := range []string{"EXDATE", "RDATE", "detached"} {
		t.Run(exception, func(t *testing.T) {
			c, b := fixtureClient(t)
			master := b.object.Data.Children[0]
			start := time.Date(2020, 1, 2, 9, 0, 0, 0, time.UTC)
			master.Props.SetDateTime(ical.PropDateTimeStart, start)
			master.Props.SetDateTime(ical.PropDateTimeEnd, start.Add(time.Hour))
			if exception == "detached" {
				detached := ical.NewEvent()
				detached.Props.SetText(ical.PropUID, "fixture")
				detached.Props.SetDateTime(ical.PropDateTimeStamp, start)
				detached.Props.SetDateTime("RECURRENCE-ID", start.AddDate(0, 0, 1))
				detached.Props.SetDateTime(ical.PropDateTimeStart, start.AddDate(0, 0, 1))
				b.object.Data.Children = append(b.object.Data.Children, detached.Component)
			} else {
				master.Props.Set(&ical.Prop{Name: exception, Value: "20200103T090000Z"})
			}
			var before strings.Builder
			if err := ical.NewEncoder(&before).Encode(b.object.Data); err != nil {
				t.Fatal(err)
			}
			shifted := start.Add(time.Hour)
			err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &EventUpdate{StartTime: &shifted})
			if err == nil || !strings.Contains(err.Error(), "exceptions") {
				t.Fatalf("expected exception-schedule rejection, got %v", err)
			}
			var after strings.Builder
			if err := ical.NewEncoder(&after).Encode(b.object.Data); err != nil {
				t.Fatal(err)
			}
			if before.String() != after.String() {
				t.Fatal("rejected schedule update changed the resource")
			}
		})
	}
}

func TestUpdateEventPreservesExceptionAnchorsOnTitleChange(t *testing.T) {
	c, b := fixtureClient(t)
	master := b.object.Data.Children[0]
	master.Props.Set(&ical.Prop{Name: "EXDATE", Value: "20200103"})
	title := "Updated title"
	if err := c.UpdateEvent(context.Background(), "/cal/fixture.ics", &EventUpdate{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if got := b.object.Data.Children[0].Props.Get("EXDATE").Value; got != "20200103" {
		t.Fatalf("exception changed: %s", got)
	}
}
