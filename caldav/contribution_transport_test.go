package caldav

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestContributionRawReport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
		fail   bool
	}{
		{"complete", 207, `<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><response><href>/cal/fixture.ics</href><propstat><prop><C:calendar-data>` + xmlEscape(richFixture) + `</C:calendar-data></prop></propstat></response></multistatus>`, 1, false},
		{"empty", 207, `<multistatus xmlns="DAV:"><response><href>/cal/empty.ics</href><propstat><prop/></propstat></response></multistatus>`, 0, false},
		{"invalid-calendar", 207, `<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><response><propstat><prop><C:calendar-data>invalid</C:calendar-data></prop></propstat></response></multistatus>`, 0, false},
		{"invalid-xml", 207, `not XML`, 0, true},
		{"status-error", 412, `rejected`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "REPORT" || r.URL.Path != "/cal/" || r.Header.Get("Depth") != "1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				if !strings.Contains(string(body), "<C:calendar-data/>") || !strings.Contains(string(body), `start="20200101T000000Z"`) || !strings.Contains(string(body), `end="20200103T000000Z"`) {
					t.Errorf("wrong REPORT projection/window: %s", body)
				}
				w.WriteHeader(tc.status)
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Error(err)
				}
			}))
			defer s.Close()
			h := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				proxy, err := http.NewRequestWithContext(r.Context(), r.Method, s.URL+"/cal/", r.Body)
				if err != nil {
					return nil, err
				}
				proxy.Header = r.Header.Clone()
				return s.Client().Transport.RoundTrip(proxy)
			})}
			c := &Client{httpDoer: h}
			start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			end := start.Add(48 * time.Hour)
			objects, err := c.rawQueryCalendar(context.Background(), "/cal/", &start, &end)
			if (err != nil) != tc.fail || len(objects) != tc.want {
				t.Fatalf("got count=%d err=%v", len(objects), err)
			}
			if tc.want == 1 {
				event, err := c.parseCalendarObject(&objects[0])
				if err != nil || event.ID != "fixture" || len(event.Alerts) != 1 {
					t.Fatalf("REPORT lost event/alarm: %+v %v", event, err)
				}
			}
		})
	}
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func TestContributionRawReportFailures(t *testing.T) {
	c := &Client{httpDoer: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("transport unavailable") })}}
	if _, err := c.rawQueryCalendar(context.Background(), "/cal/", nil, nil); err == nil {
		t.Fatal("transport failure ignored")
	}
	if _, err := c.rawQueryCalendar(context.Background(), "\n", nil, nil); err == nil {
		t.Fatal("invalid URL accepted")
	}
}

func TestContributionReadDecorators(t *testing.T) {
	c, b := fixtureClient(t)
	reader := NewRetryClient(NewRateLimitedClient(c, 100, 10), 1, time.Millisecond)
	e, err := reader.GetEvent(context.Background(), "/cal/fixture.ics")
	if err != nil || e.ID != "fixture" {
		t.Fatalf("decorated read: %v %v", e, err)
	}
	b.object = nil
	if _, err = reader.GetEvent(context.Background(), "/cal/fixture.ics"); err == nil {
		t.Fatal("empty response accepted")
	}
	if _, err = readService(context.Background(), &MockClient{}, "/cal/fixture.ics"); err == nil {
		t.Fatal("unsupported reader accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	limited := NewRateLimitedClient(c, 1, 0)
	if _, err = limited.GetEvent(ctx, "/cal/fixture.ics"); err == nil {
		t.Fatal("cancelled rate limit ignored")
	}
}

func TestContributionClientConstruction(t *testing.T) {
	for _, opt := range []ClientOptions{{}, {Timeout: time.Second, MaxConnsPerHost: 2}} {
		c, err := NewClient("fixture@example.com", "dummy", opt)
		if err != nil || c.backend == nil || c.httpDoer == nil {
			t.Fatalf("construct: %v", err)
		}
	}
	if _, err := NewClient("fixture@example.com", "dummy", ClientOptions{TLSCertFile: "missing-cert", TLSKeyFile: "missing-key"}); err == nil {
		t.Fatal("missing certificate accepted")
	}
}
