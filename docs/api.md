# API

## api-service

```http
GET /healthz
GET /readyz
GET /api/campaigns
GET /api/reports/overview
```

Admin list endpoints accept `limit` and `offset` query parameters. Paginated list responses use:

```json
{
  "items": [],
  "limit": 100,
  "offset": 0,
  "total": 0
}
```

This applies to campaign, stream, destination, traffic source, affiliate network, postback template and user lists, plus postback logs and destination health history.

Destination create/update payloads support availability policy fields:

```json
{
  "name": "Offer A",
  "type": "offer",
  "url": "https://offer.example/?subid={click_id}",
  "manual_status": "active",
  "health_status": "unknown",
  "redirect": {
    "mode": "http_302"
  },
  "schedule": {
    "enabled": true,
    "timezone": "Europe/Moscow",
    "windows": [
      {
        "weekdays": ["mon", "tue", "wed", "thu", "fri"],
        "start_time": "09:00",
        "end_time": "18:00"
      }
    ]
  },
  "caps": {
    "enabled": true,
    "rules": [
      {
        "metric": "clicks",
        "window_hours": 24,
        "limit": 10000
      },
      {
        "metric": "revenue",
        "window_hours": 24,
        "limit": 5000
      }
    ]
  }
}
```

Supported cap metrics are `clicks`, `cost`, `conversions` and `revenue`. `window_hours` is custom, so hourly, daily and weekly caps are represented as `1`, `24` and `168`.

Stream distributions support anti-repeat routing through `unique_policy`:

```json
{
  "mode": "weighted",
  "destinations": [
    {
      "destination_id": "dst_a",
      "weight": 100
    },
    {
      "destination_id": "dst_b",
      "weight": 100
    }
  ],
  "unique_policy": {
    "enabled": true,
    "user_key": "source_click_id",
    "history_window_hours": 168,
    "exhausted_mode": "allow_repeat",
    "selection_strategy": "best_roi",
    "roi_window_hours": 24,
    "min_clicks": 100,
    "fallback_strategy": "round_robin"
  }
}
```

Supported distribution modes are `direct`, `weighted`, `fallback`, `waterfall`, `round_robin` and `best_roi`. Supported unique user keys are `source_click_id`, `user_agent`, `sub1` through `sub10`, `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term` and `query.<param>`.

## traffic-service

```http
GET /c/{campaignSlug}
GET /tb/{campaignSlug}
GET /healthz
GET /readyz
```

## postback-service

```http
GET /pb/{network}
POST /api/postbacks
GET /healthz
GET /readyz
```

## Report explorer

`GET /api/reports/grouped?group_by=geo_country` supports campaign/stream/destination/source and 15 GEO/device/placement dimensions. It returns paginated rows, matching totals and row count. Filters, sorting, row conditions and timezone/date semantics are documented in [Analytics](analytics.md#api). Existing report routes remain available.
