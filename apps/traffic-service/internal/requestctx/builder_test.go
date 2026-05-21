package requestctx

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/clientip"
)

func TestBuildNormalizesKnownFields(t *testing.T) {
	resolver := clientip.NewResolver(
		[]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		[]string{"X-Forwarded-For"},
	)
	builder := NewBuilder(resolver)
	req := httptest.NewRequest(
		"GET",
		"/c/demo?clickid=ext_1&sub1=zone_a&utm_source=meta&cost=1.25&currency=USD&trafficback_depth=2&country=US&device=mobile&browser=Chrome&os=Android",
		nil,
	)
	req.RemoteAddr = "10.1.2.3:12345"
	req.Header.Set(
		"X-Forwarded-For",
		"198.51.100.20",
	)
	req.Header.Set(
		"User-Agent",
		"test-agent",
	)
	req.Header.Set(
		"Referer",
		"https://ref.example",
	)

	ctx := builder.Build(
		req,
		Input{ClickID: "clk_1", CampaignID: "cmp_1"},
	)

	if ctx.SourceClickID != "ext_1" {
		t.Fatalf(
			"SourceClickID = %q, want ext_1",
			ctx.SourceClickID,
		)
	}
	if ctx.SubIDs[0] != "zone_a" {
		t.Fatalf(
			"sub1 = %q, want zone_a",
			ctx.SubIDs[0],
		)
	}
	if ctx.UTM.Source != "meta" {
		t.Fatalf(
			"UTM.Source = %q, want meta",
			ctx.UTM.Source,
		)
	}
	if ctx.Cost != 1.25 {
		t.Fatalf(
			"Cost = %v, want 1.25",
			ctx.Cost,
		)
	}
	if ctx.IP != "198.51.100.20" {
		t.Fatalf(
			"IP = %q, want 198.51.100.20",
			ctx.IP,
		)
	}
	if ctx.TrafficbackDepth != 2 {
		t.Fatalf(
			"TrafficbackDepth = %d, want 2",
			ctx.TrafficbackDepth,
		)
	}
	if ctx.GeoCountry != "US" {
		t.Fatalf(
			"GeoCountry = %q, want US",
			ctx.GeoCountry,
		)
	}
	if ctx.DeviceType != "mobile" {
		t.Fatalf(
			"DeviceType = %q, want mobile",
			ctx.DeviceType,
		)
	}
	if ctx.Browser != "Chrome" {
		t.Fatalf(
			"Browser = %q, want Chrome",
			ctx.Browser,
		)
	}
	if ctx.OS != "Android" {
		t.Fatalf(
			"OS = %q, want Android",
			ctx.OS,
		)
	}
}

func TestClickEventCopiesRequestContextFields(t *testing.T) {
	builder := NewBuilder(nil)
	req := httptest.NewRequest(
		"GET",
		"/c/demo?clickid=ext_1&sub1=zone_a&geo_country=US&device_type=mobile&browser=Chrome",
		nil,
	)
	createdAt := time.Now().UTC()

	event := ClickEvent(
		builder.Build(
			req,
			Input{ClickID: "clk_1", CampaignID: "cmp_1", StreamID: "str_1", DestinationID: "dst_1"},
		),
		createdAt,
	)

	if event.ClickID != "clk_1" {
		t.Fatalf(
			"ClickID = %q, want clk_1",
			event.ClickID,
		)
	}
	if event.SourceClickID != "ext_1" {
		t.Fatalf(
			"SourceClickID = %q, want ext_1",
			event.SourceClickID,
		)
	}
	if event.SubIDs[0] != "zone_a" {
		t.Fatalf(
			"sub1 = %q, want zone_a",
			event.SubIDs[0],
		)
	}
	if event.GeoCountry != "US" {
		t.Fatalf(
			"GeoCountry = %q, want US",
			event.GeoCountry,
		)
	}
	if event.DeviceType != "mobile" {
		t.Fatalf(
			"DeviceType = %q, want mobile",
			event.DeviceType,
		)
	}
	if event.Browser != "Chrome" {
		t.Fatalf(
			"Browser = %q, want Chrome",
			event.Browser,
		)
	}
	if !event.CreatedAt.Equal(createdAt) {
		t.Fatalf(
			"CreatedAt = %v, want %v",
			event.CreatedAt,
			createdAt,
		)
	}
}

func TestValuesIncludesQueryAndMacroValues(t *testing.T) {
	builder := NewBuilder(nil)
	req := httptest.NewRequest(
		"GET",
		"/c/demo?sub1=zone_a&currency=USD&country=US&device=mobile",
		nil,
	)

	values := Values(builder.Build(
		req,
		Input{ClickID: "clk_1", CampaignID: "cmp_1", StreamID: "str_1", DestinationID: "dst_1"},
	))

	if values["click_id"] != "clk_1" {
		t.Fatalf(
			"click_id = %q, want clk_1",
			values["click_id"],
		)
	}
	if values["sub1"] != "zone_a" {
		t.Fatalf(
			"sub1 = %q, want zone_a",
			values["sub1"],
		)
	}
	if values["query.currency"] != "USD" {
		t.Fatalf(
			"query.currency = %q, want USD",
			values["query.currency"],
		)
	}
	if values["geo_country"] != "US" {
		t.Fatalf(
			"geo_country = %q, want US",
			values["geo_country"],
		)
	}
	if values["device_type"] != "mobile" {
		t.Fatalf(
			"device_type = %q, want mobile",
			values["device_type"],
		)
	}
}
