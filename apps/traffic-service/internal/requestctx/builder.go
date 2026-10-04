package requestctx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/clientip"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Builder struct {
	ipResolver *clientip.Resolver
}

type Input struct {
	OwnerID       string
	ClickID       string
	CampaignID    string
	StreamID      string
	DestinationID string
	SourceID      string
}

func NewBuilder(ipResolver *clientip.Resolver) *Builder {
	if ipResolver == nil {
		ipResolver = clientip.NewResolver(
			nil,
			nil,
		)
	}
	return &Builder{ipResolver: ipResolver}
}

func (b *Builder) Build(
	r *http.Request,
	input Input,
) models.RequestContext {
	q := r.URL.Query()
	query := make(
		map[string]string,
		len(q),
	)
	for key, values := range q {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}

	var subIDs [10]string
	for i := 0; i < 10; i++ {
		subIDs[i] = q.Get("sub" + strconv.Itoa(i+1))
	}

	ip := ""
	if addr, ok := b.ipResolver.Resolve(r); ok {
		ip = addr.String()
	}

	return models.RequestContext{
		ClickID:       input.ClickID,
		OwnerID:       input.OwnerID,
		CampaignID:    input.CampaignID,
		StreamID:      input.StreamID,
		DestinationID: input.DestinationID,
		SourceID: first(
			input.SourceID,
			q.Get("source_id"),
			q.Get("source"),
		),
		SourceClickID: first(
			q.Get("clickid"),
			q.Get("click_id"),
			q.Get("external_click_id"),
			q.Get("source_click_id"),
			q.Get("cid"),
			q.Get("sclid"),
			q.Get("fbclid"),
			q.Get("gclid"),
			q.Get("ttclid"),
			q.Get("yclid"),
			q.Get("msclkid"),
		),
		IP:        ip,
		UserAgent: r.UserAgent(),
		Referrer:  r.Referer(),
		GeoCountry: first(
			q.Get("geo_country"),
			q.Get("country"),
			q.Get("country_code"),
			q.Get("geo"),
		),
		GeoRegion: first(
			q.Get("geo_region"),
			q.Get("region"),
			q.Get("state"),
		),
		City: first(
			q.Get("city"),
			q.Get("geo_city"),
		),
		ASN: first(
			q.Get("asn"),
			q.Get("geo_asn"),
		),
		ISP: first(
			q.Get("isp"),
			q.Get("carrier"),
		),
		DeviceType: first(
			q.Get("device_type"),
			q.Get("device"),
		),
		OS: first(
			q.Get("os"),
			q.Get("platform"),
		),
		Browser: first(
			q.Get("browser"),
			q.Get("ua_browser"),
		),
		SubIDs: subIDs,
		UTM: models.UTM{
			Source:   q.Get("utm_source"),
			Medium:   q.Get("utm_medium"),
			Campaign: q.Get("utm_campaign"),
			Content:  q.Get("utm_content"),
			Term:     q.Get("utm_term"),
		},
		Cost: parseCost(first(
			q.Get("cost"),
			q.Get("cpc"),
			q.Get("price"),
			q.Get("bid"),
			q.Get("spend"),
		)),
		Currency:         q.Get("currency"),
		RawQuery:         r.URL.RawQuery,
		Query:            query,
		TrafficbackDepth: parseNonNegativeInt(q.Get("trafficback_depth")),
	}
}

func Values(ctx models.RequestContext) map[string]string {
	values := make(
		map[string]string,
		len(ctx.Query)+32,
	)
	for key, value := range ctx.Query {
		values[key] = value
		values["query."+key] = value
	}

	values["click_id"] = ctx.ClickID
	values["campaign_id"] = ctx.CampaignID
	values["stream_id"] = ctx.StreamID
	values["destination_id"] = ctx.DestinationID
	values["source_id"] = ctx.SourceID
	values["source_click_id"] = ctx.SourceClickID
	values["ip"] = ctx.IP
	values["user_agent"] = ctx.UserAgent
	values["referrer"] = ctx.Referrer
	values["geo_country"] = ctx.GeoCountry
	values["geo_region"] = ctx.GeoRegion
	values["city"] = ctx.City
	values["asn"] = ctx.ASN
	values["isp"] = ctx.ISP
	values["device_type"] = ctx.DeviceType
	values["device"] = ctx.DeviceType
	values["os"] = ctx.OS
	values["browser"] = ctx.Browser
	values["utm_source"] = ctx.UTM.Source
	values["utm_medium"] = ctx.UTM.Medium
	values["utm_campaign"] = ctx.UTM.Campaign
	values["utm_content"] = ctx.UTM.Content
	values["utm_term"] = ctx.UTM.Term
	values["currency"] = ctx.Currency
	values["cost"] = formatCost(ctx.Cost)
	values["trafficback_depth"] = strconv.Itoa(ctx.TrafficbackDepth)

	for i, value := range ctx.SubIDs {
		values["sub"+strconv.Itoa(i+1)] = value
	}
	return values
}

func ClickEvent(
	ctx models.RequestContext,
	createdAt time.Time,
) models.ClickEvent {
	return models.ClickEvent{
		ClickID:       ctx.ClickID,
		OwnerID:       ctx.OwnerID,
		CampaignID:    ctx.CampaignID,
		StreamID:      ctx.StreamID,
		DestinationID: ctx.DestinationID,
		SourceID:      ctx.SourceID,
		SourceClickID: ctx.SourceClickID,
		UserAgent:     ctx.UserAgent,
		GeoCountry:    ctx.GeoCountry,
		GeoRegion:     ctx.GeoRegion,
		City:          ctx.City,
		ASN:           ctx.ASN,
		ISP:           ctx.ISP,
		DeviceType:    ctx.DeviceType,
		OS:            ctx.OS,
		Browser:       ctx.Browser,
		Referrer:      ctx.Referrer,
		SubIDs:        ctx.SubIDs,
		UTM:           ctx.UTM,
		Cost:          ctx.Cost,
		Currency:      ctx.Currency,
		RawQuery:      ctx.RawQuery,
		Query:         ctx.Query,
		CreatedAt:     createdAt,
	}
}

func parseCost(value string) float64 {
	if value == "" {
		return 0
	}
	cost, err := strconv.ParseFloat(
		value,
		64,
	)
	if err != nil {
		return 0
	}
	return cost
}

func parseNonNegativeInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func formatCost(cost float64) string {
	if cost == 0 {
		return ""
	}
	return strconv.FormatFloat(
		cost,
		'f',
		-1,
		64,
	)
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
