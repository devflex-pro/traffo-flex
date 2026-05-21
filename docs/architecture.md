# Architecture

TraffoFlex MVP uses four apps:

```text
admin-frontend → api-service → MongoDB / ClickHouse
traffic users  → traffic-service → ClickHouse
affiliate nets → postback-service → MongoDB / ClickHouse
```

Service boundaries are strict:
- api-service is for admin/API.
- traffic-service is for high-load traffic redirects.
- postback-service is for postback/conversion flows.
