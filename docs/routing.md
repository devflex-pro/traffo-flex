# Routing

`traffic-service` selects destinations from the in-memory campaign cache. MongoDB is not queried in the redirect hot path.

Destination availability is evaluated in this order:

1. `manual_status` must be `active`.
2. `health_status` must not be `unhealthy`.
3. If `schedule.enabled=true`, current local time in `schedule.timezone` must match at least one schedule window.
4. If `caps.enabled=true`, no cap rule can be reached.
5. If stream `distribution.unique_policy.enabled=true`, destinations already used by the same user key inside the history window are skipped.

Schedule windows use weekdays and `HH:MM` local clock values:

```json
{
  "enabled": true,
  "timezone": "Europe/Moscow",
  "windows": [
    {
      "weekdays": ["mon", "tue", "wed", "thu", "fri"],
      "start_time": "09:00",
      "end_time": "18:00"
    }
  ]
}
```

Overnight windows are supported. For example, `22:00` to `02:00` on `mon` means Monday 22:00 through Tuesday 02:00 in the configured timezone.

Caps are configured per destination with custom windows in hours:

```json
{
  "enabled": true,
  "rules": [
    {
      "metric": "clicks",
      "window_hours": 1,
      "limit": 1000
    },
    {
      "metric": "conversions",
      "window_hours": 24,
      "limit": 300
    },
    {
      "metric": "revenue",
      "window_hours": 24,
      "limit": 5000
    }
  ]
}
```

Supported cap metrics:

- `clicks`: count from `click_events`.
- `cost`: sum of `cost` from `click_events`.
- `conversions`: count from `conversion_events`.
- `revenue`: sum of `payout` from `conversion_events`.

`traffic-service` checks caps through ClickHouse and keeps a short TTL cache controlled by `DESTINATION_CAP_CACHE_TTL_MS`. If ClickHouse cap checking fails, the destination remains available and a warning is logged; this keeps the redirect hot path fail-open for analytics dependency issues.

## Stream rules

Streams are evaluated by priority. The first active stream whose conditions match the request handles the click.

Supported operators:

- `eq`, `neq`
- `in`, `not_in`
- `contains`, `not_contains`
- `starts_with`, `ends_with`
- `gt`, `gte`, `lt`, `lte`
- `exists`, `not_exists`
- `regex`

Rule fields come from the request context. Supported normalized fields:

- source: `source_id`, `source_click_id`
- IDs: `campaign_id`, `stream_id`, `destination_id`, `click_id`
- network/client: `ip`, `user_agent`, `referrer`
- geo: `geo_country`, `geo_region`, `city`, `asn`, `isp`
- device: `device_type`, `device`, `os`, `browser`
- sub IDs: `sub1` through `sub10`
- UTM: `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term`
- economics: `cost`, `currency`
- trafficback: `trafficback_depth`
- raw query params: both `<param>` and `query.<param>`

Common query aliases are normalized before rule matching:

- `country`, `country_code` and `geo` fill `geo_country`.
- `region` and `state` fill `geo_region`.
- `geo_city` fills `city`.
- `geo_asn` fills `asn`.
- `carrier` fills `isp`.
- `device` fills `device_type`.
- `platform` fills `os`.
- `ua_browser` fills `browser`.

Examples:

```json
[
  {
    "field": "source_id",
    "operator": "eq",
    "value": "facebook"
  },
  {
    "field": "geo_country",
    "operator": "in",
    "values": ["US", "CA"]
  },
  {
    "field": "browser",
    "operator": "neq",
    "value": "Safari"
  },
  {
    "field": "user_agent",
    "operator": "contains",
    "value": "Android"
  },
  {
    "field": "query.zone",
    "operator": "eq",
    "value": "native_top"
  }
]
```

## Anti-repeat destination routing

Anti-repeat routing prevents sending the same user key to the same destination repeatedly while other suitable destinations are still available.

Example:

```json
{
  "mode": "weighted",
  "destinations": [
    { "destination_id": "dst_a", "weight": 100 },
    { "destination_id": "dst_b", "weight": 100 },
    { "destination_id": "dst_c", "weight": 100 }
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

Supported `user_key` values:

- `source_click_id`
- `user_agent`
- `sub1` through `sub10`
- `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term`
- `query.<param>`

Routing order:

1. Match the first active stream whose conditions match the request.
2. Filter destinations by manual status, health, schedule and caps.
3. Load used destinations for the configured user key from ClickHouse `click_events`.
4. Exclude already used destinations inside `history_window_hours`.
5. If every destination was already used, `exhausted_mode=allow_repeat` returns to the full available list, while `no_destination` leaves the list empty.
6. Select from the remaining candidates using `selection_strategy`.

Selection strategies:

- `waterfall`: first configured available destination.
- `round_robin`: hash-based rotation over available candidates for the current click.
- `weighted`: current weighted distribution.
- `best_roi`: candidates are ranked by ROI from ClickHouse over `roi_window_hours`; rows with fewer than `min_clicks` are ignored. If there is not enough ROI data, `fallback_strategy` is used.

The history and ROI lookups use the same short TTL cache as destination cap checks. If the lookup fails, routing fails open and continues with the current available candidates.
