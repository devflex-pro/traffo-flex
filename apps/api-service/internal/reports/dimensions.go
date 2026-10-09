package reports

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var reportDimensions = []string{"geo_country", "geo_region", "city", "device_type", "os", "browser", "isp", "zone_id", "publisher_id", "site_id", "creative_id", "carrier", "connection_type", "source_campaign_id", "source_campaign_name"}

func isReportDimension(field string) bool {
	for _, dimension := range reportDimensions {
		if dimension == field {
			return true
		}
	}
	return false
}

func parseReportGroup(raw string) (GroupBy, error) {
	switch raw {
	case "", "campaign", "campaign_id", "campaigns":
		return GroupByCampaign, nil
	case "stream", "stream_id", "streams":
		return GroupByStream, nil
	case "destination", "destination_id", "destinations":
		return GroupByDestination, nil
	case "source", "source_id", "sources":
		return GroupBySource, nil
	case "trafficback":
		return GroupByTrafficback, nil
	case "health":
		return GroupByHealth, nil
	default:
		if isReportDimension(raw) {
			return GroupBy(raw), nil
		}
		return "", errors.Join(
			ErrInvalidQuery,
			errors.New("invalid group_by"),
		)
	}
}

func (h *Handler) Grouped(
	w http.ResponseWriter,
	r *http.Request,
) {
	group, err := parseReportGroup(r.URL.Query().Get("group_by"))
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
		return
	}
	h.respondGrouped(
		w,
		r,
		group,
	)
}

// Dimension values can contain country names, spaces and Unicode, but never
// control characters. SQL identifiers always come from the static whitelist.
func validDimensionValue(value string) bool {
	if len(value) > 256 || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func rawDimensionExpression(field string) string {
	switch field {
	case "zone_id":
		return "if(notEmpty(JSONExtractString(query, 'zone_id')), JSONExtractString(query, 'zone_id'), sub1)"
	case "publisher_id":
		return "if(notEmpty(JSONExtractString(query, 'publisher_id')), JSONExtractString(query, 'publisher_id'), sub2)"
	case "site_id":
		return "if(notEmpty(JSONExtractString(query, 'site_id')), JSONExtractString(query, 'site_id'), sub3)"
	case "creative_id":
		return "if(notEmpty(JSONExtractString(query, 'creative_id')), JSONExtractString(query, 'creative_id'), sub4)"
	case "carrier":
		return "JSONExtractString(query, 'carrier')"
	case "connection_type":
		return "JSONExtractString(query, 'source_connection_type')"
	case "source_campaign_id":
		return "JSONExtractString(query, 'source_campaign_id')"
	case "source_campaign_name":
		return "JSONExtractString(query, 'source_campaign_name')"

	default:
		return field
	}
}

func reportDimensionExpression(
	table string,
	field string,
) string {
	if table == clickEventsTable && isReportDimension(field) {
		return rawDimensionExpression(field)
	}
	return field
}

func hasDimensionFilters(query Query) bool {
	if len(query.Dimensions) > 0 {
		return true
	}
	for _, field := range query.EmptyFields {
		if isReportDimension(field) {
			return true
		}
	}
	return false
}

func reportRowOrder(query Query) string {
	field := query.Sort
	switch field {
	case "name":
		field = "id"
	case "cr":
		field = "if(clicks = 0, 0, conversions / clicks)"
	case "epc":
		field = "if(clicks = 0, 0, revenue / clicks)"
	case "cpc":
		field = "if(clicks = 0, 0, cost / clicks)"
	case "cpa":
		field = "if(conversions = 0, 0, cost / conversions)"
	case "clicks", "conversions", "revenue", "cost", "profit", "roi":
	default:
		field = "clicks"
	}
	order := "DESC"
	if query.Order == "asc" {
		order = "ASC"
	}
	return field + " " + order + ", id ASC"
}

func groupedRowFilters(query Query) string {
	conditions := []string{}
	if query.MinClicks > 0 {
		conditions = append(
			conditions,
			"clicks >= "+strconv.Itoa(query.MinClicks),
		)
	}
	if query.Profit == "positive" {
		conditions = append(
			conditions,
			"profit > 0",
		)
	}
	if query.Profit == "negative" {
		conditions = append(
			conditions,
			"profit < 0",
		)
	}
	if len(conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(
		conditions,
		" AND ",
	)
}
