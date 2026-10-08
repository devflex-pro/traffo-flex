# Postbacks

TraffoFlex receives conversion callbacks from affiliate networks and can send
approved conversions to traffic sources. The two directions have separate tabs
in the admin **Postbacks** page. The frontend uses `api-service` for configuration;
public callbacks are handled by `postback-service`.

## Incoming setup

1. Create an **Incoming** integration. Select your affiliate network or create
   it in the same form, give the integration a name and keep its generated secret.
2. Enter the network's macro names without braces: for example, `cid` for
   `{cid}` and `sum` for `{sum}`. Set the payout currency. Transaction ID and
   Status macros are optional: a blank transaction macro uses the click macro,
   and a blank status macro sends `approved`.
3. Save the integration and copy its generated Postback URL into the affiliate
   network's postback settings. The URL uses the network's ID, not the template
   slug. For click macro `cid`, payout macro `sum`, USD and no transaction/status
   macro, its shape is:

   ```text
   https://cb.tf.devflex.pro/pb/NETWORK_ID?secret=YOUR_SECRET&click_id={cid}&transaction_id={cid}&payout={sum}&currency=USD&status=approved
   ```

The form is shared by all affiliate networks. Offer and smartlink URLs are
configured as campaign destinations. Add `TRAFFIC_CLICK_PARAMETER={click_id}`
there, using the parameter name required by that network, so its callbacks can
return the TraffoFlex click ID. Pass the traffic source's own token to the
campaign tracking URL as `source_click_id=SOURCE_CLICK_MACRO`.

Using the click ID as the transaction ID counts **one conversion per click**.
Repeated callbacks with the same owner/network/transaction ID return the
previously accepted conversion without changing payout or status.
Use a distinct transaction-ID macro for networks that report multiple orders
or events per click. Status updates for an already accepted transaction are not
implemented.

For example, LosPollos documents `{cid}` and `{sum}` in its
[FAQ](https://www.lospollos.com/es/faq/). Enter `cid` and `sum` in the generic
form, leave Transaction ID and Status blank, and paste the saved URL into
**Create Postback → Global → Postback URL**. Configure `cid={click_id}` on its
smartlink separately, in the campaign destination. Existing callback URLs
created with the previous preset keep working; no stored data is migrated.

## Incoming requests

`GET /pb/{network_id}` accepts `click_id` (aliases `cid`, `subid`, `sub_id`),
`transaction_id` (aliases `tx`, `tid`, `order_id`), `payout` (aliases `sum`,
`amount`, `revenue`), `currency`, `status` and `event_type`. `secret`, `token` or
`key` must match a nonempty secret on an incoming template. The matched template
selects the owner's workspace. The template mapping in the form defines the
affiliate network macros used to build a URL with these canonical parameters;
it does not rename the receiver's accepted parameter names.

`POST /api/postbacks` accepts form data or a JSON object with string values and
looks up incoming templates with `network_id=api`.

Accepted conversions are durably saved before the HTTP response. Analytics
delivery and click attribution are retried in the background. A late click may
therefore delay campaign revenue without rejecting the conversion. Incoming
logs show accepted, duplicate and rejected requests; errors with no authenticated
owner may not appear in a workspace's log.

The incoming log is a table with a single exact-ID search (postback, click or
transaction ID), optional UTC date boundaries and a sortable time column.
Filtering, sorting and pagination run on the API over all workspace records.
`GET /api/postback-logs` accepts `id`, RFC3339 `from`/`to`, `order=asc|desc`,
`limit` and `offset`; the previous individual filters remain supported.

## Outgoing delivery

Create an **Outgoing** integration with a public HTTP(S) URL and optionally
select a traffic source and/or campaign. An empty scope is Global **within the
current workspace**; when both fields are selected, both must match. For example:

```text
https://source.example/postback?clickid={source_click_id}&revenue={payout}&event_id={conversion_id}
```

Macros are case insensitive. Supported values include `source_click_id`,
`click_id`/`cid`, `conversion_id`, `transaction_id`, `payout`/`sum`/`revenue`,
`currency`, `status`, `event_type`, `network_id`, `sub1`…`sub10` and `s1`…`s4`.
Values are query encoded. Payout is in the conversion's original currency;
TraffoFlex does not perform currency conversion.

Only new conversions with status `approved`, `sale` or `confirmed` are sent.
Click metadata is read from ClickHouse in the background, scoped to the same
owner. If `{source_click_id}` is required but absent, the job fails visibly.
Requests never run on the click or incoming-postback request path.

Jobs are saved in MongoDB `outbound_postbacks`, uniquely keyed by conversion and
template, with journaled writes and expiring leases. Failed requests retry with
backoff, up to five attempts. Delivery uses HTTP GET, a five-second timeout,
no redirects, and a maximum four-KiB response read. Only 2xx responses count as
delivered. Private/reserved IP addresses and DNS resolutions are blocked.
Disabled or deleted templates cancel queued jobs. The admin delivery log shows
the latest 100 jobs, their status, attempt count, HTTP status and error.

Delivery is **at least once**: a receiver may accept a request before the sender
can save its checkpoint. Receivers should deduplicate by `conversion_id` or the
stable `Idempotency-Key` header. Local queue deduplication does not guarantee
exactly-once delivery across an uncertain HTTP acknowledgement.

## Upgrade and tests

These changes add fields and a collection; no existing MongoDB or ClickHouse
data is rewritten. Templates without `direction` keep incoming behavior.
`POSTBACK_PUBLIC_URL` supplies the public callback origin to the frontend; the
production Compose file derives it from `POSTBACK_DOMAIN`. New queue indexes are
created idempotently by postback-service at startup. Historical conversions
without `outbound_status` are not replayed, and templates created after a
conversion do not receive it. No new dependencies are required.

Run `make test`, `go vet ./...` in each Go module and the frontend build. The
persistent recovery test uses an isolated MongoDB only:

```sh
cd apps/postback-service
OUTBOUND_TEST_MONGO_URI=mongodb://127.0.0.1:27019 go test ./internal/outbound -v
```

Log filtering, chronological sorting, pagination and owner isolation also have
an isolated MongoDB test:

```sh
cd apps/api-service
POSTBACK_LOG_TEST_MONGO_URI=mongodb://127.0.0.1:27019 go test ./internal/postbacklogs -v
```
