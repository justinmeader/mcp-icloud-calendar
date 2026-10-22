package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"time"

	"github.com/emersion/go-ical"
	extcaldav "github.com/emersion/go-webdav/caldav"
)

// rawQueryCalendar performs a calendar-query REPORT with an EMPTY
// <C:calendar-data/> projection. iCloud rejects (412 Precondition Failed)
// any projection that names nested components, and returns component-less
// VCALENDAR shells for allprop+allcomp projections; only the empty
// projection returns complete calendar objects. go-webdav v0.7.0 cannot
// express an empty projection, so the request is built by hand.
// Verified live against caldav.icloud.com on 2026-07-17.
func (c *Client) rawQueryCalendar(ctx context.Context, calendarPath string, startTime, endTime *time.Time) ([]extcaldav.CalendarObject, error) {
	var timeRange string
	if startTime != nil || endTime != nil {
		attrs := ""
		if startTime != nil {
			attrs += fmt.Sprintf(` start="%s"`, startTime.UTC().Format("20060102T150405Z"))
		}
		if endTime != nil {
			attrs += fmt.Sprintf(` end="%s"`, endTime.UTC().Format("20060102T150405Z"))
		}
		timeRange = "<C:time-range" + attrs + "/>"
	}

	body := `<?xml version="1.0" encoding="utf-8" ?>` +
		`<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">` +
		`<D:prop><D:getetag/><C:calendar-data/></D:prop>` +
		`<C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT">` +
		timeRange +
		`</C:comp-filter></C:comp-filter></C:filter>` +
		`</C:calendar-query>`

	req, err := http.NewRequestWithContext(ctx, "REPORT", iCloudBaseURL+calendarPath, bytes.NewBufferString(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build calendar-query request: %w", err)
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)

	resp, err := c.httpDoer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calendar-query request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // The read result is authoritative; closing is cleanup.
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	var ms struct {
		Responses []struct {
			Href      string `xml:"href"`
			Propstats []struct {
				Status string `xml:"status"`
				Prop   struct {
					CalendarData string `xml:"calendar-data"`
				} `xml:"prop"`
			} `xml:"propstat"`
		} `xml:"response"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("failed to parse multistatus response: %w", err)
	}

	objects := make([]extcaldav.CalendarObject, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		resourcePath, ok := reportResourcePath(req.URL, r.Href)
		if !ok {
			continue // Omit unusable hrefs without hiding valid sibling objects.
		}
		for _, ps := range r.Propstats {
			data := strings.TrimSpace(ps.Prop.CalendarData)
			if data == "" {
				continue
			}
			cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
			if err != nil {
				continue // parseCalendarObject-level skip semantics; leave malformed objects out
			}
			objects = append(objects, extcaldav.CalendarObject{Path: resourcePath, Data: cal})
		}
	}
	return objects, nil
}

// reportResourcePath converts a REPORT href into the plain calendar-scoped path
// accepted by event tools. Check before resolving: ResolveReference would erase
// traversal, and decoding first would turn encoded separators into real ones.
func reportResourcePath(calendarURL *url.URL, href string) (string, bool) {
	u, err := url.Parse(href)
	if err != nil || u.Opaque != "" || u.User != nil || strings.ContainsAny(href, "?#") {
		return "", false
	}
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.ContainsAny(u.Path, "%?#\\") || ValidateCalendarPath(u.Path) != nil {
		return "", false
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return "", false
		}
	}
	base := *calendarURL
	base.Path = strings.TrimSuffix(base.Path, "/") + "/"
	base.RawPath = ""
	resolved := base.ResolveReference(u)
	port := func(v *url.URL) string {
		if p := v.Port(); p != "" {
			return p
		}
		if strings.EqualFold(v.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	if !strings.EqualFold(resolved.Scheme, base.Scheme) || !strings.EqualFold(resolved.Hostname(), base.Hostname()) || port(resolved) != port(&base) {
		return "", false
	}
	if !strings.HasPrefix(resolved.Path, base.Path) {
		return "", false
	}
	resource := strings.TrimPrefix(resolved.Path, base.Path)
	if ValidateEventID(resource) != nil || resource == "." {
		return "", false
	}
	return resolved.Path, true
}
