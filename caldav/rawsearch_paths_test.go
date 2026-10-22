package caldav

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	extcaldav "github.com/emersion/go-webdav/caldav"
)

func reportPathsClient(hrefs ...string) *Client {
	var body strings.Builder
	body.WriteString(`<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)
	for _, href := range hrefs {
		body.WriteString(`<response><href>` + xmlEscape(href) + `</href><propstat><prop><C:calendar-data>` + xmlEscape(richFixture) + `</C:calendar-data></prop></propstat></response>`)
	}
	body.WriteString(`</multistatus>`)
	return &Client{httpDoer: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusMultiStatus, Body: io.NopCloser(strings.NewReader(body.String()))}, nil
	})}}
}

func TestRawReportNormalizesResourcePaths(t *testing.T) {
	for _, tc := range []struct{ name, href, want string }{
		{"absolute", iCloudBaseURL + "/cal/fixture.ics", "/cal/fixture.ics"},
		{"default-port", "https://CALDAV.ICLOUD.COM:443/cal/fixture.ics", "/cal/fixture.ics"},
		{"root", "/cal/fixture.ics", "/cal/fixture.ics"},
		{"relative", "fixture.ics", "/cal/fixture.ics"},
		{"escaped-space", iCloudBaseURL + "/cal/team%20meeting.ics", "/cal/team meeting.ics"},
		{"escaped-unicode", "/cal/caf%C3%A9.ics", "/cal/café.ics"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := reportPathsClient(tc.href)
			objects, err := c.rawQueryCalendar(context.Background(), "/cal", nil, nil)
			if err != nil || len(objects) != 1 || objects[0].Path != tc.want {
				t.Fatalf("raw REPORT: objects=%+v err=%v; want path %q", objects, err, tc.want)
			}
			events, err := c.SearchEvents(context.Background(), "/cal/", nil, nil)
			if err != nil || len(events) != 1 || events[0].Path != tc.want {
				t.Fatalf("SearchEvents: events=%+v err=%v; want path %q", events, err, tc.want)
			}
		})
	}
}

func TestRawReportSkipsUnsafeResourcePaths(t *testing.T) {
	for _, href := range []string{
		"https://attacker.example/cal/fixture.ics", "http://caldav.icloud.com/cal/fixture.ics",
		"https://user@caldav.icloud.com/cal/fixture.ics", "//attacker.example/cal/fixture.ics",
		"/other/fixture.ics", "/calendar/fixture.ics", "/cal/../other/fixture.ics", "../other/fixture.ics",
		"/cal/./fixture.ics", "/cal/%2e/fixture.ics", "/cal/%2e%2e/fixture.ics", "/cal/nested%2Ffixture.ics", "/cal/nested%5cfixture.ics",
		"/cal/%252e%252e.ics", "/cal/fixture.ics?x=1", "/cal/fixture.ics?", "/cal/fixture.ics#fragment", "/cal/fixture.ics#",
		"/cal/fixture%3F.ics", "/cal/fixture%23.ics", "/cal/nested/fixture.ics", "/cal/", "", "/cal/invalid%xx.ics",
	} {
		t.Run(href, func(t *testing.T) {
			// Invalid entries are omitted, like malformed calendar data; valid
			// siblings in a multistatus response must remain available.
			c := reportPathsClient(href, "/cal/fixture.ics")
			events, err := c.SearchEvents(context.Background(), "/cal/", nil, nil)
			if err != nil || len(events) != 1 || events[0].Path != "/cal/fixture.ics" {
				t.Fatalf("unsafe href %q was exposed or valid sibling lost: events=%+v err=%v", href, events, err)
			}
		})
	}
}

func TestRawReportPathRoundTripsThroughReadAndUpdate(t *testing.T) {
	c := reportPathsClient(iCloudBaseURL + "/cal/team%20meeting.ics")
	events, err := c.SearchEvents(context.Background(), "/cal/", nil, nil)
	if err != nil || len(events) != 1 {
		t.Fatalf("search: events=%+v err=%v", events, err)
	}
	var methods []string
	h := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		methods = append(methods, r.Method)
		if r.URL.Scheme != "https" || r.URL.Host != "caldav.icloud.com" || r.URL.EscapedPath() != "/cal/team%20meeting.ics" {
			t.Errorf("resource request did not roundtrip: %s", r.URL)
		}
		status, body := http.StatusOK, richFixture
		if r.Method == http.MethodPut {
			status, body = http.StatusCreated, ""
		}
		return &http.Response{Request: r, StatusCode: status, Header: http.Header{"Content-Type": {"text/calendar"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	b, err := extcaldav.NewClient(h, iCloudBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	reader := NewClientWithBackend(b)
	if _, err := reader.GetEvent(context.Background(), events[0].Path); err != nil {
		t.Fatalf("read returned path: %v", err)
	}
	title := "Changed"
	if err := reader.UpdateEvent(context.Background(), events[0].Path, &EventUpdate{Title: &title}); err != nil {
		t.Fatalf("update returned path: %v", err)
	}
	if strings.Join(methods, ",") != "GET,GET,PUT" {
		t.Fatalf("read/update methods: %v", methods)
	}
}
