package models

import "time"

type RequestContext struct {
	ClickID          string            `json:"click_id"`
	OwnerID          string            `json:"owner_id,omitempty"`
	CampaignID       string            `json:"campaign_id"`
	StreamID         string            `json:"stream_id,omitempty"`
	DestinationID    string            `json:"destination_id,omitempty"`
	SourceID         string            `json:"source_id,omitempty"`
	SourceClickID    string            `json:"source_click_id,omitempty"`
	IP               string            `json:"ip,omitempty"`
	UserAgent        string            `json:"user_agent,omitempty"`
	Referrer         string            `json:"referrer,omitempty"`
	GeoCountry       string            `json:"geo_country,omitempty"`
	GeoRegion        string            `json:"geo_region,omitempty"`
	City             string            `json:"city,omitempty"`
	ASN              string            `json:"asn,omitempty"`
	ISP              string            `json:"isp,omitempty"`
	DeviceType       string            `json:"device_type,omitempty"`
	OS               string            `json:"os,omitempty"`
	Browser          string            `json:"browser,omitempty"`
	SubIDs           [10]string        `json:"sub_ids"`
	UTM              UTM               `json:"utm"`
	Cost             float64           `json:"cost,omitempty"`
	Currency         string            `json:"currency,omitempty"`
	RawQuery         string            `json:"raw_query"`
	Query            map[string]string `json:"query"`
	TrafficbackDepth int               `json:"trafficback_depth"`
}

type UTM struct {
	Source   string `json:"source,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Campaign string `json:"campaign,omitempty"`
	Content  string `json:"content,omitempty"`
	Term     string `json:"term,omitempty"`
}

type ClickEvent struct {
	ClickID       string            `json:"click_id"`
	OwnerID       string            `json:"owner_id,omitempty"`
	CampaignID    string            `json:"campaign_id"`
	StreamID      string            `json:"stream_id,omitempty"`
	DestinationID string            `json:"destination_id,omitempty"`
	SourceID      string            `json:"source_id,omitempty"`
	SourceClickID string            `json:"source_click_id,omitempty"`
	IPHash        string            `json:"ip_hash,omitempty"`
	IPPrefix      string            `json:"ip_prefix,omitempty"`
	UserAgent     string            `json:"user_agent,omitempty"`
	UAHash        string            `json:"ua_hash,omitempty"`
	GeoCountry    string            `json:"geo_country,omitempty"`
	GeoRegion     string            `json:"geo_region,omitempty"`
	City          string            `json:"city,omitempty"`
	ASN           string            `json:"asn,omitempty"`
	ISP           string            `json:"isp,omitempty"`
	DeviceType    string            `json:"device_type,omitempty"`
	OS            string            `json:"os,omitempty"`
	Browser       string            `json:"browser,omitempty"`
	Referrer      string            `json:"referrer,omitempty"`
	SubIDs        [10]string        `json:"sub_ids"`
	UTM           UTM               `json:"utm"`
	Cost          float64           `json:"cost,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	IsBot         bool              `json:"is_bot"`
	BotScore      float64           `json:"bot_score,omitempty"`
	RawQuery      string            `json:"raw_query"`
	Query         map[string]string `json:"query"`
	CreatedAt     time.Time         `json:"created_at"`
}

type ConversionEvent struct {
	ConversionID  string            `json:"conversion_id"`
	OwnerID       string            `json:"owner_id,omitempty"`
	ClickID       string            `json:"click_id"`
	TransactionID string            `json:"transaction_id"`
	CampaignID    string            `json:"campaign_id,omitempty"`
	StreamID      string            `json:"stream_id,omitempty"`
	DestinationID string            `json:"destination_id,omitempty"`
	SourceID      string            `json:"source_id,omitempty"`
	OfferID       string            `json:"offer_id,omitempty"`
	EventType     string            `json:"event_type"`
	Status        string            `json:"status"`
	Payout        float64           `json:"payout,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	NetworkID     string            `json:"network_id"`
	RawPayload    map[string]string `json:"raw_payload"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type PostbackLogEvent struct {
	PostbackID    string            `json:"postback_id"`
	OwnerID       string            `json:"owner_id,omitempty"`
	NetworkID     string            `json:"network_id"`
	ClickID       string            `json:"click_id,omitempty"`
	TransactionID string            `json:"transaction_id,omitempty"`
	Status        string            `json:"status"`
	Error         string            `json:"error,omitempty"`
	RawPayload    map[string]string `json:"raw_payload"`
	CreatedAt     time.Time         `json:"created_at"`
}

type TrafficbackEvent struct {
	ClickID             string            `json:"click_id"`
	OwnerID             string            `json:"owner_id,omitempty"`
	CampaignID          string            `json:"campaign_id"`
	StreamID            string            `json:"stream_id,omitempty"`
	DestinationID       string            `json:"destination_id,omitempty"`
	Reason              TrafficbackReason `json:"reason"`
	Depth               int               `json:"depth"`
	VisitedDestinations []string          `json:"visited_destinations"`
	CreatedAt           time.Time         `json:"created_at"`
}

type DestinationHealthEvent struct {
	DestinationID string       `json:"destination_id"`
	OwnerID       string       `json:"owner_id,omitempty"`
	Previous      HealthStatus `json:"previous"`
	Current       HealthStatus `json:"current"`
	Error         string       `json:"error,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
}
