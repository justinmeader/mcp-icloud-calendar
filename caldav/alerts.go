package caldav

import (
	"fmt"
	"github.com/emersion/go-ical"
	"regexp"
	"strings"
)

// Alert exposes stored VALARM properties. Writes support DISPLAY reminders only.
// Trigger is an iCalendar duration (e.g. -PT15M), related to START by default.
type Alert struct {
	Action      string `json:"action"`
	Trigger     string `json:"trigger"`
	Related     string `json:"related,omitempty"`
	Description string `json:"description,omitempty"`
	ValueType   string `json:"valueType,omitempty"`
	Repeat      string `json:"repeat,omitempty"`
	Duration    string `json:"duration,omitempty"`
}
type Attachment struct {
	Value      string `json:"value"`
	ValueType  string `json:"valueType,omitempty"`
	FormatType string `json:"formatType,omitempty"`
	Encoding   string `json:"encoding,omitempty"`
}

var alertDuration = regexp.MustCompile(`^[+-]?P([0-9]+W|([0-9]+D)?(T([0-9]+H)?([0-9]+M)?([0-9]+S)?)?)$`)

// ValidateAlerts rejects unsupported alarms rather than silently changing them.
func ValidateAlerts(alerts []Alert) error {
	if len(alerts) > 10 {
		return fmt.Errorf("at most 10 alerts are supported")
	}
	for i, a := range alerts {
		if a.Action != "" && a.Action != "DISPLAY" {
			return fmt.Errorf("alert %d: only DISPLAY writes are supported", i)
		}
		if !alertDuration.MatchString(a.Trigger) || a.Trigger == "P" || strings.HasSuffix(a.Trigger, "T") {
			return fmt.Errorf("alert %d: trigger must be an iCalendar duration such as -PT15M", i)
		}
		p := ical.Prop{Name: "TRIGGER", Value: a.Trigger}
		if _, err := p.Duration(); err != nil {
			return fmt.Errorf("alert %d: invalid trigger: %w", i, err)
		}
		if a.Related != "" && a.Related != "START" && a.Related != "END" {
			return fmt.Errorf("alert %d: related must be START or END", i)
		}
		if a.ValueType != "" && a.ValueType != "DURATION" || a.Repeat != "" || a.Duration != "" {
			return fmt.Errorf("alert %d: absolute or repeating alarm writes are not supported", i)
		}
	}
	return nil
}

func replaceAlerts(event *ical.Component, alerts []Alert) {
	kept := make([]*ical.Component, 0, len(event.Children)+len(alerts))
	for _, c := range event.Children {
		if c.Name != "VALARM" {
			kept = append(kept, c)
		}
	}
	for _, a := range alerts {
		c := ical.NewComponent("VALARM")
		c.Props.SetText("ACTION", "DISPLAY")
		p := ical.Prop{Name: "TRIGGER", Value: a.Trigger, Params: ical.Params{}}
		if a.Related != "" {
			p.Params.Set("RELATED", a.Related)
		}
		c.Props.Set(&p)
		desc := a.Description
		if desc == "" {
			desc = "Reminder"
		}
		c.Props.SetText("DESCRIPTION", desc)
		kept = append(kept, c)
	}
	event.Children = kept
}

func readExtraFields(event *Event, c *ical.Component) {
	value := func(name string) string {
		if p := c.Props.Get(name); p != nil {
			return p.Value
		}
		return ""
	}
	event.URL = value("URL")
	event.Status = value("STATUS")
	event.Transparency = value("TRANSP")
	event.Organizer = value("ORGANIZER")
	event.RecurrenceID = value("RECURRENCE-ID")
	if p := c.Props.Get("DTSTART"); p != nil {
		event.AllDay = p.Params.Get("VALUE") == "DATE"
		event.Floating = !event.AllDay && p.Params.Get("TZID") == "" && !strings.HasSuffix(p.Value, "Z")
		if event.AllDay {
			event.StartDate = p.Value
		}
	}
	if p := c.Props.Get("DTEND"); p != nil && event.AllDay {
		event.EndDate = p.Value
	}
	event.Alerts = make([]Alert, 0)
	for _, child := range c.Children {
		if child.Name != "VALARM" {
			continue
		}
		a := Alert{}
		if p := child.Props.Get("ACTION"); p != nil {
			a.Action = p.Value
		}
		if p := child.Props.Get("TRIGGER"); p != nil {
			a.Trigger = p.Value
			a.Related = p.Params.Get("RELATED")
			a.ValueType = p.Params.Get("VALUE")
		}
		if p := child.Props.Get("DESCRIPTION"); p != nil {
			a.Description, _ = p.Text()
		}
		if p := child.Props.Get("REPEAT"); p != nil {
			a.Repeat = p.Value
		}
		if p := child.Props.Get("DURATION"); p != nil {
			a.Duration = p.Value
		}
		event.Alerts = append(event.Alerts, a)
	}
	for _, p := range c.Props["ATTACH"] {
		event.Attachments = append(event.Attachments, Attachment{Value: p.Value, ValueType: p.Params.Get("VALUE"), FormatType: p.Params.Get("FMTTYPE"), Encoding: p.Params.Get("ENCODING")})
	}
}
