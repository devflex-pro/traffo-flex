# TraffoFlex Product Overview

TraffoFlex is a self-hosted platform foundation for traffic routing, tracking, postbacks, and analytics. It helps teams that work with paid traffic keep campaigns, destinations, conversions, limits, and reporting in one controlled system.

The product is intended for legitimate affiliate marketing, performance marketing, A/B testing, campaign analytics, and conversion attribution.

## Summary

TraffoFlex helps teams:

- launch campaign links and control where traffic goes;
- split traffic between destinations by rules, weights, and selection strategies;
- disable destinations manually, by health status, by schedule, or by caps;
- avoid sending the same user to the same destination repeatedly when other options exist;
- choose destinations by waterfall, round robin, weighted selection, or best ROI;
- receive postbacks from affiliate networks;
- connect clicks and conversions;
- review revenue, cost, profit, and ROI;
- keep data inside self-hosted infrastructure;
- validate the full pipeline from click to report.

## Target Users

TraffoFlex is useful for:

- media buyers;
- affiliate and performance marketing teams;
- teams that own traffic sources;
- teams that work with several affiliate networks;
- small self-hosted teams that need data control;
- engineering teams building a custom tracker or routing product.

## Core Idea

Performance teams need a clear view of:

- where traffic goes;
- which stream or destination performs;
- which traffic source is profitable;
- where an offer, budget, or conversion limit has been reached;
- where postback or analytics integration breaks;
- when a destination cannot be used because of time windows, caps, or health status.

TraffoFlex connects these pieces:

```text
Campaign -> Stream rules -> Destination policy -> Click event -> Postback -> Conversion -> Report
```

## Campaign And Stream Management

The system manages:

- campaigns;
- streams inside campaigns;
- destinations;
- traffic sources;
- affiliate networks;
- postback templates.

This keeps routing centralized, avoids changing traffic source links for every offer URL change, and makes it easier to test several traffic flows inside one campaign.

## Smart Traffic Routing

`traffic-service` receives an incoming click and chooses a destination from current configuration.

Implemented routing capabilities:

- routing by campaign slug;
- routing by public campaign ID;
- routing by public token;
- stream priority;
- rule matching by source, geo, device, browser, user agent, UTM, sub IDs, and query parameters;
- weighted distribution;
- anti-repeat routing by user key;
- waterfall, round robin, weighted, and best ROI selection;
- trafficback route;
- trafficback loop protection;
- safe macro rendering;
- protection against inbound `url` and `redirect_url` overrides.

This lets teams test multiple landing pages or offers, split traffic by source and user context, gradually shift traffic by weights, route returning users to different destinations, and fall back to trafficback when no destination is available.

## Unique Routing And Best ROI

A stream can enable `unique_policy`. TraffoFlex records which destinations were already used by a specific user key and tries another matching destination first.

Supported user keys:

- `source_click_id`;
- `user_agent`;
- `sub1` through `sub10`;
- UTM fields;
- any raw query parameter through `query.<param>`.

Destination selection strategies:

- `waterfall`: take the first available destination in order.
- `round_robin`: rotate available destinations.
- `weighted`: distribute traffic according to configured weights.
- `best_roi`: rank destinations by ROI calculated from ClickHouse analytics.

If ROI data is still insufficient, the configured fallback strategy is used, usually `round_robin`.

## Destination Availability

TraffoFlex evaluates several factors before sending traffic to a destination:

- manual status;
- health status;
- schedule;
- caps.

If a destination is unavailable, the selector tries another destination. If no destination matches, traffic goes to the configured fallback flow.

## Destination Schedules

Each destination can have a working schedule:

- weekdays;
- start time;
- end time;
- timezone;
- overnight windows.

Example:

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

An overnight window such as `22:00-02:00` on `mon` means Monday 22:00 through Tuesday 02:00.

Schedules help avoid sending traffic outside call-center hours, outside offer availability, or outside a partner's working timezone.

## Destination Caps

Caps are configured as rules with custom hour windows.

Supported metrics:

- `clicks`: maximum number of clicks;
- `cost`: maximum spend;
- `conversions`: maximum number of conversions;
- `revenue`: maximum postback payout or revenue.

Example:

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
      "metric": "cost",
      "window_hours": 24,
      "limit": 500
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

Common windows:

- `1` hour for an hourly cap;
- `24` hours for a daily cap;
- `168` hours for a weekly cap;
- any other positive value for a custom cap.

Caps help teams avoid oversending traffic after an offer limit, partner limit, spend limit, or payout limit has already been reached.

## Destination Healthchecks

TraffoFlex checks destination URLs and stores health state.

Implemented:

- manual trigger from the admin API and UI;
- internal trigger endpoint in `traffic-service`;
- periodic worker;
- current health state;
- health transition history;
- admin UI history view.

Unhealthy destinations are excluded from selection.

## Conversion Tracking And Postbacks

TraffoFlex receives affiliate network postbacks and normalizes them into conversions.

Implemented:

- GET and JSON POST postbacks;
- postback templates;
- field mapping;
- secret/token validation;
- click lookup in ClickHouse;
- conversion enrichment with campaign, stream, destination, and source data;
- dedupe by transaction ID and network scope;
- postback logs;
- conversion events for analytics.

This makes click-to-conversion attribution visible and helps diagnose broken mappings, invalid secrets, duplicate conversions, and partner integration errors.

## Analytics And Reports

Events are written to Redpanda and ClickHouse, then used for reports.

Reports include:

- overview metrics;
- daily chart data;
- campaigns;
- streams;
- destinations;
- traffic sources;
- trafficback;
- health events;
- ingestion errors.

Core metrics:

- clicks;
- conversions;
- revenue;
- cost;
- profit;
- ROI;
- event counts.

Reports make it easier to compare campaigns, streams, and destinations by profitability and decide where to shift or stop traffic.

## Admin UI

The admin frontend includes screens for:

- login;
- dashboard;
- reports;
- campaigns;
- streams;
- destinations;
- traffic sources;
- affiliate networks;
- postback templates;
- postback logs;
- destination health history;
- ingestion errors;
- user approval.

This lets operators handle core workflows without direct MongoDB or ClickHouse access.

## Self-Hosted Control

TraffoFlex runs on infrastructure controlled by the owner. Click, source, conversion, payout, and profit data can stay inside the operator's environment.

Self-hosting also makes it possible to control retention, backups, access, integrations, and product extensions.

## Typical Workflow

1. Admin logs in with email OTP.
2. Admin creates a campaign.
3. Admin adds streams with rules and priority.
4. Admin adds destinations.
5. Admin configures weights or selection strategy.
6. Admin enables unique routing if returning users should be split across destinations.
7. Admin configures destination schedules and caps.
8. Admin configures a traffic source and postback template.
9. Traffic starts through the public campaign URL.
10. Clicks and conversions are collected.
11. Reports show performance, money metrics, and ROI.
12. Weak destinations can be paused or reweighted.
13. Postback logs, health history, and ingestion errors help diagnose issues.

## MVP Status

The current MVP includes the main operating product:

- auth and admin approval;
- campaign and stream management;
- destination management;
- routing rules;
- weighted and strategy-based distribution;
- destination schedules and caps;
- health checks;
- trafficback;
- click logging;
- postback normalization;
- conversion dedupe;
- analytics ingestion;
- reports;
- admin UI.

Production hardening is still required around deployment, secrets, observability, load testing, backups, and security review.

## Responsible Use

TraffoFlex is not intended for deception, phishing, malware delivery, unauthorized cloaking, moderation evasion, spam, or other illegal use cases.

It is intended for legitimate traffic routing, affiliate tracking, campaign analytics, A/B testing, and conversion attribution.
