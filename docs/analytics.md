# Analytics and report explorer

The admin frontend calls `api-service` only. ClickHouse is the source of truth for analytics; campaign configuration and saved tracking parameters live in MongoDB.

## Finding combinations in Reports

Choose a period and timezone, apply campaign/stream/destination/source filters, then select **Group by**, **Then by**, and an optional third level. For example: Country → Device → Site. Click a row to constrain that segment and move to the next group. Breadcrumbs return to earlier levels. The last-level row applies an exact filter. Missing values appear as **Unknown** and can also be drilled into.

Available groups: campaign, stream, destination, traffic source, country, region, city, device, OS, browser, ISP, zone, publisher, site, creative, carrier, connection type, source campaign ID and source campaign name. Entity groups display configuration names when available.

The GEO/devices/placements filter panel accepts exact values. Table headers sort on the server; pagination supports 25–500 rows. Search searches the displayed page. **Profitable / Unprofitable** and **Minimum clicks** restrict the table rows; the table footer totals all matching rows across pages. Top summary cards and the daily chart describe the entire selected segment, before these row conditions. CR, CPC, EPC and CPA are derived from segment clicks, conversions, revenue and cost. Zero-denominator cost-per-action metrics display a dash.

Grouping, filters, drill path, dates, timezone and table settings are kept in the page URL, so an analysis can be bookmarked or shared with a user who has access to the same workspace. Trafficback and health event reports keep their supported operational filters.

## Full URL-builder mapping

Default fields are zone, country and external click ID (also copied into `utm_content`). The rest are optional. Optional macro values are editable: use only macros actually supported by the selected source. TraffoFlex stores received strings; it does not resolve `[MACRO]` placeholders or infer device/GEO from absent source values.

In the table, `query.x` means `click_events.query`, a JSON string containing normalized query parameters. Every parameter is also preserved in `click_events.raw_query`. **Report dimension** means a field in both `click_stats_1h` and `click_stats_1d` and the compact `click_attribution_lookup`; it can be grouped, filtered and drilled into in Reports. Production physical tables have `_local` suffixes; public names are Distributed facades.

| Builder field | URL parameter / default macro | Saved click value | Where to analyze |
| --- | --- | --- | --- |
| Zone ID | `zone_id=[ZONE_ID]` | `query.zone_id` | Report dimension `zone_id` (Zone) |
| Country | `geo_country=[COUNTRY]` | `geo_country` column | Report dimension `geo_country` (Country) |
| Ad network click ID | `clickid=[CLICK_ID]` | `source_click_id` column | Raw click events; not a report group |
| Click ID compatibility copy | `utm_content=[CLICK_ID]` | `utm_content` column | Raw click events; not a report group |
| Region | `geo_region=[REGION]` | `geo_region` column | Report dimension `geo_region` (Region) |
| City | `city=[CITY]` | `city` column | Report dimension `city` (City) |
| Publisher ID | `publisher_id=[PUBLISHER_ID]` | `query.publisher_id` | Report dimension `publisher_id` (Publisher) |
| Site ID | `site_id=[SITE_ID]` | `query.site_id` | Report dimension `site_id` (Site) |
| Creative ID | `creative_id=[CREATIVE_ID]` | `query.creative_id` | Report dimension `creative_id` (Creative) |
| Device | `device_type=[DEVICE]` | `device_type` column | Report dimension `device_type` (Device) |
| Browser | `browser=[BROWSER]` | `browser` column | Report dimension `browser` (Browser) |
| OS | `os=[OS]` | `os` column | Report dimension `os` (OS) |
| ISP | `isp=[ISP]` | `isp` column | Report dimension `isp` (ISP) |
| Carrier | `carrier=[CARRIER]` | `query.carrier`; also the ISP fallback if `isp` is absent | Report dimension `carrier` (Carrier) |
| Connection type | `source_connection_type=[CONNECTION_TYPE]` | `query.source_connection_type` | Report dimension `connection_type` (Connection type) |
| Source campaign ID | `source_campaign_id=[CAMPAIGN_ID]` | `query.source_campaign_id` | Report dimension `source_campaign_id` |
| Source campaign name | `source_campaign_name=[CAMPAIGN_NAME]` | `query.source_campaign_name` | Report dimension `source_campaign_name` |
| Cost macro | `cost=YOUR_PRICE_MACRO`, for example `[CPV_PRICE]` | Raw/query value is unchanged; `cost` column stores normalized click expense | Cost, profit, ROI and expense caps |
| Source IP | `source_ip=[IP]` | `query.source_ip`, unchanged source string | Raw click events only; **not trusted visitor IP** |

`[COUNTRY]` can return a country name; `[COUNTRY_CODE]` in RichPops returns a three-letter code. Neither is automatically converted into a two-letter country code. Set the macro and rule/filter values consistently. RichPops does not advertise every optional macro above: do not enable unsupported placeholders such as `[DEVICE]` without an actual source equivalent.

A campaign's configured traffic source becomes `click_events.source_id` and the Traffic source report dimension. It is distinct from `zone_id`. TraffoFlex generates its own `click_id`; this internal click ID should be passed to the destination and returned in incoming postbacks. The source click ID is available for outgoing postbacks through the existing source-click macro.

### Existing links and destination macros

Old links using `sub1`/`sub2`/`sub3`/`sub4` continue to work. They map to zone/publisher/site/creative respectively, and named parameters take precedence when both are present. Explicit old sub values remain in the original sub columns. New named links do not populate those columns artificially; existing destination `{sub1}`–`{sub4}` macros and rule values fall back to the named values when a sub is absent.

Open and save an existing campaign's URL builder to migrate its saved tracking parameters to the named form. Previously copied URLs remain compatible. Custom optional macro values are preserved.

### Pricing and spend

Choose **Pricing model** in Create/Edit campaign: CPC records incoming `cost`
unchanged; CPM records `cost / 1000` for each tracked click. `cost=2.5` with
CPM adds `0.0025` per click, or `25` for 10,000 clicks. Existing campaigns
without the setting use CPC. Changes apply to subsequent clicks after the
routing cache refresh; historical costs remain unchanged.

Set the source price macro in **Cost macro** in Tracking URL builder, for
example `cost=[CPV_PRICE]`. The separate raw CPV field is retired. Existing
`source_cpv_price` links still preserve that parameter without setting expense;
edit the builder and choose the campaign model before sending it as `cost`.
Missing, unresolved, negative or non-finite prices record zero expense. Legacy
`cpc`, `price`, `bid` and `spend` aliases remain supported, with `cost` taking
precedence; all use the campaign model. Inbound query parameters cannot
override that model. Normalization occurs before rules, destination macros
and click logging, so reports, caps and ROI consume the same click expense.

### Inspect raw values

High-cardinality click IDs and raw-only fields are intentionally excluded from aggregates. They can be inspected in ClickHouse, restricted to the workspace owner:

```sql
SELECT created_at, click_id, source_click_id, utm_content,
       geo_country, geo_region, city, device_type, os, browser, isp,
       JSONExtractString(query, 'zone_id') AS zone_id,
       JSONExtractString(query, 'source_cpv_price') AS source_cpv_price,
       raw_query
FROM traffoflex.click_events
WHERE owner_id = 'YOUR_OWNER_ID' AND click_id = 'YOUR_CLICK_ID'
ORDER BY created_at DESC;
```

Incoming postback logs search internal click/transaction IDs; they are not a raw-click browser. Only values supplied by the source can appear in GEO/device/placement reports; historical missing fields stay Unknown.

## Storage and calculations

The active click chain is `click_events` → `click_stats_1h` → `click_stats_1d`. The day boundary is UTC. Both aggregates retain campaign, stream, destination, source and all 15 additional dimensions. Query `sum(clicks)` and `sum(cost)` because SummingMergeTree combines matching rows asynchronously. Minute aggregation is retired.

Reports use daily aggregates for full UTC days, hourly aggregates for full hours, and raw click events for partial hours. Non-UTC daily charts use raw timestamps when necessary to preserve timezone and DST boundaries. Calendar-date filters mean whole days in the selected timezone; RFC3339 filters retain their exact second boundaries. Destination caps and ROI use complete hourly buckets plus the raw partial boundary hour in their background workers.

Conversions are deduplicated by owner and conversion ID, with compact attributed events providing campaign/stream/destination/source references. GEO/device/placement conversion reports join the compact click lookup using both owner and click ID, not the full raw click history. Unattributed conversions are included in unfiltered totals and Unknown dimension rows; they appear in entity/known-dimension segments after attribution succeeds.

Click broker replays can still add clicks/cost more than once. The existing duplicate audit reports these cases; this release does not change click deduplication or rebuild duplicates silently. Mixed-currency conversion is not implemented. Revenue and cost must use consistent currencies to interpret profit and ROI.

## API

- `GET /api/reports/grouped?group_by=geo_country`
- `GET /api/reports/overview`
- `GET /api/reports/daily`
- Existing `/campaigns`, `/streams`, `/destinations`, `/sources`, `/trafficback`, `/health` endpoints remain.

Common filters: `from`, `to`, `timezone`, `campaign_id`, `stream_id`, `destination_id`, `source_id`, and any of the 15 dimension field names listed above. `empty=geo_country,carrier` selects missing values. IDs allow letters, numbers, `_` and `-`; dimension values allow Unicode up to 256 bytes and reject control characters. SQL identifiers always come from a fixed whitelist.

Grouped-table options: `sort=name|clicks|conversions|revenue|cost|profit|roi|cr|epc|cpc|cpa`, `order=asc|desc`, `limit=1..500` (default 100), `offset>=0`, `min_clicks>=0`, `profit=positive|negative`. The response includes `rows`, overall matching `summary`, `total`, `limit`, and `offset`. Overview and daily charts ignore table-row conditions.

## Migrating an existing ClickHouse volume

This schema change requires stopped writers and a complete volume backup. New materialized views do not retroactively aggregate old events. Applying `006_click_rollups.sql` alone to a populated older volume is insufficient and can leave two active aggregation paths.

Use `scripts/migrate-hour-day-rollups.sh`, adding `--production` for the Replicated/Distributed topology. The script pauses click ingestion, compares raw history with old hourly count/cost, and refuses migration when retained raw history is incomplete. It saves view definitions, renames old physical tables to `*_before_hour_day`, creates the new lookup/hour/day chain, backfills from raw clicks once and verifies raw/hour/day sums. Existing configuration, raw clicks and conversion events are preserved. A successful repeat invocation is a no-op.

Keep traffic writers stopped until the new API/traffic images and report checks are ready. With writers stopped, `--restore --backup-dir DIR` restores saved old tables and views. If raw count/cost changed after the upgrade, it rebuilds the old lookup and aggregate chain from retained raw events and verifies totals before resuming ingestion. Preserve raw events and Kafka/WAL state for this rollback; never restore stale aggregate snapshots alone after accepting new traffic. Keep the complete backup and old tables until verification; remove `*_before_hour_day` tables separately only after successful checks. For a fresh volume, the image initializes the new schema directly.
