#!/usr/bin/env bash
set -euo pipefail

MONGO_CONTAINER="${MONGO_CONTAINER:-traffoflex-mongo}"
CLICKHOUSE_CONTAINER="${CLICKHOUSE_CONTAINER:-traffoflex-clickhouse}"
MONGO_DATABASE="${MONGO_DATABASE:-traffoflex}"
CLICKHOUSE_DATABASE="${CLICKHOUSE_DATABASE:-traffoflex}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-traffoflex}"
RESET_CLICKHOUSE_SCHEMA="${RESET_CLICKHOUSE_SCHEMA:-0}"

require_container() {
  local name="$1"
  local running
  running="$(docker inspect -f '{{.State.Running}}' "$name" 2>/dev/null || true)"
  if [[ "$running" != "true" ]]; then
    echo "container $name is not running; start the local stack with: make up" >&2
    exit 1
  fi
}

seed_mongo() {
  docker exec -i "$MONGO_CONTAINER" mongosh --quiet "$MONGO_DATABASE" <<'JS'
const now = new Date();
const ago = (hours) => new Date(now.getTime() - hours * 60 * 60 * 1000);
const teamID = "team_demo";

const campaignIDs = [
  "cmp_demo_finance",
  "cmp_demo_mobile",
  "cmp_demo_ecommerce",
  "cmp_demo_sweepstakes",
];
const streamIDs = [
  "str_demo_finance_us_mobile",
  "str_demo_finance_desktop",
  "str_demo_mobile_android",
  "str_demo_mobile_ios",
  "str_demo_ecommerce_retarget",
  "str_demo_sweepstakes_global",
];
const destinationIDs = [
  "dst_demo_credit_cards",
  "dst_demo_invest_landing",
  "dst_demo_android_offer",
  "dst_demo_ios_offer",
  "dst_demo_shop_sale",
  "dst_demo_coupon_fallback",
  "dst_demo_sweeps_main",
  "dst_demo_sweeps_backup",
];
const sourceIDs = [
  "src_demo_meta",
  "src_demo_google",
  "src_demo_tiktok",
  "src_demo_native",
];
const networkIDs = [
  "net_demo_adcombo",
  "net_demo_clickdealer",
  "net_demo_inhouse",
];
const templateIDs = [
  "pbt_demo_adcombo_main",
  "pbt_demo_clickdealer_main",
  "pbt_demo_inhouse_main",
];
const extraCampaignNumbers = Array.from({ length: 16 }, (_, index) => index + 5);
const extraCampaignIDs = extraCampaignNumbers.map((number) => `cmp_demo_case_${String(number).padStart(2, "0")}`);
const extraStreamIDs = extraCampaignNumbers.map((number) => `str_demo_case_${String(number).padStart(2, "0")}`);
const extraDestinationIDs = extraCampaignNumbers.map((number) => `dst_demo_case_${String(number).padStart(2, "0")}`);
campaignIDs.push(...extraCampaignIDs);
streamIDs.push(...extraStreamIDs);
destinationIDs.push(...extraDestinationIDs);

db.campaigns.deleteMany({ id: /^cmp_demo_/ });
db.streams.deleteMany({ id: /^str_demo_/ });
db.destinations.deleteMany({ id: /^dst_demo_/ });
db.traffic_sources.deleteMany({ id: { $in: sourceIDs } });
db.affiliate_networks.deleteMany({ id: { $in: networkIDs } });
db.postback_templates.deleteMany({ id: { $in: templateIDs } });
db.destination_health.deleteMany({ destination_id: { $in: destinationIDs } });
db.destination_health_history.deleteMany({ demo: true });
db.postback_logs.deleteMany({ demo: true });
db.users.deleteOne({ id: "usr_demo_admin" });

db.users.insertOne({
  _id: "usr_demo_admin",
  id: "usr_demo_admin",
  email: "admin@example.com",
  role: "admin",
  status: "active",
  email_verified: true,
  approved: true,
  otp_hash: "",
  otp_expires_at: new Date(0),
  otp_requested_at: new Date(0),
  created_at: ago(240),
  updated_at: now,
});

db.traffic_sources.insertMany([
  { _id: "src_demo_meta", id: "src_demo_meta", team_id: teamID, name: "Meta Ads", slug: "demo-meta", created_at: ago(220), updated_at: now },
  { _id: "src_demo_google", id: "src_demo_google", team_id: teamID, name: "Google UAC", slug: "demo-google-uac", created_at: ago(219), updated_at: now },
  { _id: "src_demo_tiktok", id: "src_demo_tiktok", team_id: teamID, name: "TikTok Ads", slug: "demo-tiktok", created_at: ago(218), updated_at: now },
  { _id: "src_demo_native", id: "src_demo_native", team_id: teamID, name: "Native DSP", slug: "demo-native-dsp", created_at: ago(217), updated_at: now },
]);

db.affiliate_networks.insertMany([
  { _id: "net_demo_adcombo", id: "net_demo_adcombo", team_id: teamID, name: "AdCombo Demo", slug: "demo-adcombo", created_at: ago(216), updated_at: now },
  { _id: "net_demo_clickdealer", id: "net_demo_clickdealer", team_id: teamID, name: "ClickDealer Demo", slug: "demo-clickdealer", created_at: ago(215), updated_at: now },
  { _id: "net_demo_inhouse", id: "net_demo_inhouse", team_id: teamID, name: "In-house Offers", slug: "demo-inhouse", created_at: ago(214), updated_at: now },
]);

db.campaigns.insertMany([
  {
    _id: "cmp_demo_finance",
    id: "cmp_demo_finance",
    public_id: "go_demo_finance",
    public_token: "rt_demo_finance",
    team_id: teamID,
    name: "Finance US - Q2 Scale",
    slug: "demo-finance-us",
    status: "active",
    entry_url: "https://trk.example.com/c/demo-finance-us",
    custom_domain: "trk.example.com",
    traffic_source_id: "src_demo_meta",
    currency: "USD",
    default_action: "route",
    trafficback_config: {
      enabled: true,
      url: "https://fallback.example.com/finance",
      max_depth: 2,
      fallback_campaign: "",
      fallback_url: "https://fallback.example.com",
    },
    created_at: ago(200),
    updated_at: now,
  },
  {
    _id: "cmp_demo_mobile",
    id: "cmp_demo_mobile",
    public_id: "go_demo_mobile",
    public_token: "rt_demo_mobile",
    team_id: teamID,
    name: "Mobile Apps - Tier 1",
    slug: "demo-mobile-apps",
    status: "active",
    entry_url: "https://trk.example.com/c/demo-mobile-apps",
    custom_domain: "trk.example.com",
    traffic_source_id: "src_demo_google",
    currency: "USD",
    default_action: "route",
    trafficback_config: { enabled: true, url: "https://fallback.example.com/apps", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    created_at: ago(190),
    updated_at: now,
  },
  {
    _id: "cmp_demo_ecommerce",
    id: "cmp_demo_ecommerce",
    public_id: "go_demo_ecommerce",
    public_token: "rt_demo_ecommerce",
    team_id: teamID,
    name: "E-commerce Retargeting",
    slug: "demo-ecommerce-retarget",
    status: "active",
    entry_url: "https://trk.example.com/c/demo-ecommerce-retarget",
    custom_domain: "trk.example.com",
    traffic_source_id: "src_demo_tiktok",
    currency: "USD",
    default_action: "route",
    trafficback_config: { enabled: true, url: "https://fallback.example.com/shop", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    created_at: ago(180),
    updated_at: now,
  },
  {
    _id: "cmp_demo_sweepstakes",
    id: "cmp_demo_sweepstakes",
    public_id: "go_demo_sweeps",
    public_token: "rt_demo_sweeps",
    team_id: teamID,
    name: "Sweepstakes Global",
    slug: "demo-sweepstakes-global",
    status: "paused",
    entry_url: "https://trk.example.com/c/demo-sweepstakes-global",
    custom_domain: "",
    traffic_source_id: "src_demo_native",
    currency: "USD",
    default_action: "trafficback",
    trafficback_config: { enabled: true, url: "https://fallback.example.com/sweeps", max_depth: 1, fallback_campaign: "", fallback_url: "" },
    created_at: ago(170),
    updated_at: now,
  },
]);

const verticals = [
  ["SaaS Trial", "src_demo_google", "route"],
  ["Nutra Leadgen", "src_demo_meta", "route"],
  ["Gaming CPA", "src_demo_tiktok", "route"],
  ["Insurance Quotes", "src_demo_native", "route"],
  ["Crypto Education", "src_demo_meta", "route"],
  ["Travel Deals", "src_demo_google", "route"],
  ["Home Services", "src_demo_native", "route"],
  ["Dating Tier 2", "src_demo_tiktok", "trafficback"],
];
db.campaigns.insertMany(extraCampaignNumbers.map((number, index) => {
  const suffix = String(number).padStart(2, "0");
  const [vertical, sourceID, defaultAction] = verticals[index % verticals.length];
  const status = index % 9 === 0 ? "paused" : "active";
  return {
    _id: `cmp_demo_case_${suffix}`,
    id: `cmp_demo_case_${suffix}`,
    public_id: `go_demo_case_${suffix}`,
    public_token: `rt_demo_case_${suffix}`,
    team_id: teamID,
    name: `${vertical} - ${index % 2 === 0 ? "Scale" : "Test"} ${suffix}`,
    slug: `demo-case-${suffix}`,
    status,
    entry_url: `https://trk.example.com/c/demo-case-${suffix}`,
    custom_domain: "trk.example.com",
    traffic_source_id: sourceID,
    currency: "USD",
    default_action: defaultAction,
    trafficback_config: {
      enabled: true,
      url: `https://fallback.example.com/case-${suffix}`,
      max_depth: 2,
      fallback_campaign: "",
      fallback_url: "",
    },
    created_at: ago(168 - index * 3),
    updated_at: now,
  };
}));

const workdays = ["mon", "tue", "wed", "thu", "fri"];
const allDays = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];
db.destinations.insertMany([
  {
    _id: "dst_demo_credit_cards",
    id: "dst_demo_credit_cards",
    name: "Credit Cards LP",
    type: "landing",
    url: "https://offers.example.com/credit?cid={click_id}&sub1={sub1}&utm={utm_campaign}",
    manual_status: "active",
    health_status: "healthy",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/finance", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: true, timezone: "America/New_York", windows: [{ weekdays: workdays, start_time: "08:00", end_time: "22:00" }] },
    caps: { enabled: true, rules: [{ metric: "clicks", window_hours: 24, limit: 4200 }, { metric: "cost", window_hours: 24, limit: 850 }] },
    created_at: ago(160),
    updated_at: now,
  },
  {
    _id: "dst_demo_invest_landing",
    id: "dst_demo_invest_landing",
    name: "Invest Advisor Funnel",
    type: "offer",
    url: "https://offers.example.com/invest?click={click_id}&source={source_id}",
    manual_status: "active",
    health_status: "degraded",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/finance", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: false, timezone: "", windows: [] },
    caps: { enabled: true, rules: [{ metric: "revenue", window_hours: 168, limit: 18000 }] },
    created_at: ago(159),
    updated_at: now,
  },
  {
    _id: "dst_demo_android_offer",
    id: "dst_demo_android_offer",
    name: "Android VPN Trial",
    type: "offer",
    url: "https://apps.example.com/android-vpn?cid={click_id}",
    manual_status: "active",
    health_status: "healthy",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: false, url: "", max_depth: 0, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: false, timezone: "", windows: [] },
    caps: { enabled: true, rules: [{ metric: "conversions", window_hours: 24, limit: 360 }] },
    created_at: ago(158),
    updated_at: now,
  },
  {
    _id: "dst_demo_ios_offer",
    id: "dst_demo_ios_offer",
    name: "iOS Fitness Trial",
    type: "offer",
    url: "https://apps.example.com/ios-fitness?cid={click_id}",
    manual_status: "active",
    health_status: "healthy",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: false, url: "", max_depth: 0, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: false, timezone: "", windows: [] },
    caps: { enabled: false, rules: [] },
    created_at: ago(157),
    updated_at: now,
  },
  {
    _id: "dst_demo_shop_sale",
    id: "dst_demo_shop_sale",
    name: "Seasonal Sale",
    type: "landing",
    url: "https://shop.example.com/sale?click_id={click_id}",
    manual_status: "active",
    health_status: "healthy",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/shop", max_depth: 1, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: true, timezone: "Europe/Berlin", windows: [{ weekdays: allDays, start_time: "00:00", end_time: "23:59" }] },
    caps: { enabled: true, rules: [{ metric: "cost", window_hours: 24, limit: 1250 }] },
    created_at: ago(156),
    updated_at: now,
  },
  {
    _id: "dst_demo_coupon_fallback",
    id: "dst_demo_coupon_fallback",
    name: "Coupon Fallback",
    type: "fallback",
    url: "https://shop.example.com/coupons?cid={click_id}",
    manual_status: "active",
    health_status: "unknown",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: false, url: "", max_depth: 0, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: false, timezone: "", windows: [] },
    caps: { enabled: false, rules: [] },
    created_at: ago(155),
    updated_at: now,
  },
  {
    _id: "dst_demo_sweeps_main",
    id: "dst_demo_sweeps_main",
    name: "Sweepstakes Main",
    type: "offer",
    url: "https://sweeps.example.com/main?cid={click_id}",
    manual_status: "paused",
    health_status: "healthy",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/sweeps", max_depth: 1, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: false, timezone: "", windows: [] },
    caps: { enabled: false, rules: [] },
    created_at: ago(154),
    updated_at: now,
  },
  {
    _id: "dst_demo_sweeps_backup",
    id: "dst_demo_sweeps_backup",
    name: "Sweepstakes Backup",
    type: "fallback",
    url: "https://sweeps.example.com/backup?cid={click_id}",
    manual_status: "active",
    health_status: "unhealthy",
    redirect: { mode: "http_302" },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/sweeps", max_depth: 1, fallback_campaign: "", fallback_url: "" },
    schedule: { enabled: false, timezone: "", windows: [] },
    caps: { enabled: false, rules: [] },
    created_at: ago(153),
    updated_at: now,
  },
]);

db.destinations.insertMany(extraCampaignNumbers.map((number, index) => {
  const suffix = String(number).padStart(2, "0");
  const isPaused = index % 10 === 0;
  const health = index % 7 === 0 ? "degraded" : index % 11 === 0 ? "unhealthy" : "healthy";
  return {
    _id: `dst_demo_case_${suffix}`,
    id: `dst_demo_case_${suffix}`,
    name: `${verticals[index % verticals.length][0]} Offer ${suffix}`,
    type: index % 3 === 0 ? "landing" : "offer",
    url: `https://offers.example.com/case-${suffix}?cid={click_id}&src={source_id}`,
    manual_status: isPaused ? "paused" : "active",
    health_status: health,
    redirect: { mode: "http_302" },
    trafficback_config: {
      enabled: true,
      url: `https://fallback.example.com/case-${suffix}`,
      max_depth: 2,
      fallback_campaign: "",
      fallback_url: "",
    },
    schedule: index % 4 === 0 ? {
      enabled: true,
      timezone: "UTC",
      windows: [{ weekdays: allDays, start_time: "06:00", end_time: "23:00" }],
    } : { enabled: false, timezone: "", windows: [] },
    caps: {
      enabled: index % 3 === 0,
      rules: index % 3 === 0 ? [{ metric: "cost", window_hours: 24, limit: 500 + index * 75 }] : [],
    },
    created_at: ago(150 - index * 2),
    updated_at: now,
  };
}));

db.streams.insertMany([
  {
    _id: "str_demo_finance_us_mobile",
    id: "str_demo_finance_us_mobile",
    campaign_id: "cmp_demo_finance",
    name: "US Mobile Buyers",
    priority: 10,
    status: "active",
    conditions: [
      { field: "geo_country", operator: "eq", value: "US", values: [] },
      { field: "device_type", operator: "in", values: ["mobile", "tablet"], value: "" },
      { field: "utm_campaign", operator: "contains", value: "scale", values: [] },
    ],
    distribution: {
      mode: "weighted",
      destinations: [{ destination_id: "dst_demo_credit_cards", weight: 70 }, { destination_id: "dst_demo_invest_landing", weight: 30 }],
      unique_policy: { enabled: true, user_key: "source_click_id", history_window_hours: 72, exhausted_mode: "allow_repeat", selection_strategy: "best_roi", roi_window_hours: 168, min_clicks: 30, fallback_strategy: "round_robin" },
    },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/finance", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    created_at: ago(140),
    updated_at: now,
  },
  {
    _id: "str_demo_finance_desktop",
    id: "str_demo_finance_desktop",
    campaign_id: "cmp_demo_finance",
    name: "Desktop Finance",
    priority: 20,
    status: "active",
    conditions: [{ field: "device_type", operator: "eq", value: "desktop", values: [] }],
    distribution: { mode: "waterfall", destinations: [{ destination_id: "dst_demo_invest_landing", weight: 100 }, { destination_id: "dst_demo_credit_cards", weight: 100 }], unique_policy: { enabled: false } },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/finance", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    created_at: ago(139),
    updated_at: now,
  },
  {
    _id: "str_demo_mobile_android",
    id: "str_demo_mobile_android",
    campaign_id: "cmp_demo_mobile",
    name: "Android Users",
    priority: 10,
    status: "active",
    conditions: [{ field: "os", operator: "contains", value: "Android", values: [] }],
    distribution: { mode: "direct", destinations: [{ destination_id: "dst_demo_android_offer", weight: 100 }], unique_policy: { enabled: false } },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/apps", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    created_at: ago(138),
    updated_at: now,
  },
  {
    _id: "str_demo_mobile_ios",
    id: "str_demo_mobile_ios",
    campaign_id: "cmp_demo_mobile",
    name: "iOS Users",
    priority: 20,
    status: "active",
    conditions: [{ field: "os", operator: "contains", value: "iOS", values: [] }],
    distribution: { mode: "direct", destinations: [{ destination_id: "dst_demo_ios_offer", weight: 100 }], unique_policy: { enabled: false } },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/apps", max_depth: 2, fallback_campaign: "", fallback_url: "" },
    created_at: ago(137),
    updated_at: now,
  },
  {
    _id: "str_demo_ecommerce_retarget",
    id: "str_demo_ecommerce_retarget",
    campaign_id: "cmp_demo_ecommerce",
    name: "Warm Retargeting",
    priority: 10,
    status: "active",
    conditions: [{ field: "sub1", operator: "in", values: ["cart", "viewed_product", "checkout"], value: "" }],
    distribution: { mode: "weighted", destinations: [{ destination_id: "dst_demo_shop_sale", weight: 85 }, { destination_id: "dst_demo_coupon_fallback", weight: 15 }], unique_policy: { enabled: true, user_key: "sub2", history_window_hours: 48, exhausted_mode: "allow_repeat", selection_strategy: "round_robin", roi_window_hours: 72, min_clicks: 10, fallback_strategy: "waterfall" } },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/shop", max_depth: 1, fallback_campaign: "", fallback_url: "" },
    created_at: ago(136),
    updated_at: now,
  },
  {
    _id: "str_demo_sweepstakes_global",
    id: "str_demo_sweepstakes_global",
    campaign_id: "cmp_demo_sweepstakes",
    name: "Global Fallback",
    priority: 10,
    status: "paused",
    conditions: [{ field: "geo_country", operator: "not_in", values: ["CN", "IR"], value: "" }],
    distribution: { mode: "waterfall", destinations: [{ destination_id: "dst_demo_sweeps_main", weight: 100 }, { destination_id: "dst_demo_sweeps_backup", weight: 100 }], unique_policy: { enabled: false } },
    trafficback_config: { enabled: true, url: "https://fallback.example.com/sweeps", max_depth: 1, fallback_campaign: "", fallback_url: "" },
    created_at: ago(135),
    updated_at: now,
  },
]);

db.streams.insertMany(extraCampaignNumbers.map((number, index) => {
  const suffix = String(number).padStart(2, "0");
  return {
    _id: `str_demo_case_${suffix}`,
    id: `str_demo_case_${suffix}`,
    campaign_id: `cmp_demo_case_${suffix}`,
    name: `${index % 2 === 0 ? "Primary" : "Qualified"} Traffic ${suffix}`,
    priority: 10,
    status: index % 9 === 0 ? "paused" : "active",
    conditions: [
      { field: "geo_country", operator: index % 3 === 0 ? "in" : "not_in", values: index % 3 === 0 ? ["US", "GB", "CA"] : ["CN", "IR"], value: "" },
      { field: "device_type", operator: "in", values: index % 2 === 0 ? ["mobile", "tablet"] : ["desktop"], value: "" },
    ],
    distribution: {
      mode: index % 3 === 0 ? "waterfall" : "weighted",
      destinations: [{ destination_id: `dst_demo_case_${suffix}`, weight: 100 }],
      unique_policy: index % 4 === 0 ? {
        enabled: true,
        user_key: "source_click_id",
        history_window_hours: 72,
        exhausted_mode: "allow_repeat",
        selection_strategy: "round_robin",
        roi_window_hours: 168,
        min_clicks: 25,
        fallback_strategy: "waterfall",
      } : { enabled: false },
    },
    trafficback_config: {
      enabled: true,
      url: `https://fallback.example.com/case-${suffix}`,
      max_depth: 2,
      fallback_campaign: "",
      fallback_url: "",
    },
    created_at: ago(132 - index * 2),
    updated_at: now,
  };
}));

db.postback_templates.insertMany([
  {
    _id: "pbt_demo_adcombo_main",
    id: "pbt_demo_adcombo_main",
    team_id: teamID,
    network_id: "net_demo_adcombo",
    name: "AdCombo Standard",
    slug: "demo-adcombo-standard",
    secret: "demo-secret-adcombo",
    mapping: { click_id: "cid", transaction_id: "txid", payout: "payout", status: "status", currency: "currency", offer_id: "offer" },
    created_at: ago(120),
    updated_at: now,
  },
  {
    _id: "pbt_demo_clickdealer_main",
    id: "pbt_demo_clickdealer_main",
    team_id: teamID,
    network_id: "net_demo_clickdealer",
    name: "ClickDealer Lead",
    slug: "demo-clickdealer-lead",
    secret: "demo-secret-clickdealer",
    mapping: { click_id: "clickid", transaction_id: "conversion_id", payout: "sum", status: "state", currency: "cur", offer_id: "offer_id" },
    created_at: ago(119),
    updated_at: now,
  },
  {
    _id: "pbt_demo_inhouse_main",
    id: "pbt_demo_inhouse_main",
    team_id: teamID,
    network_id: "net_demo_inhouse",
    name: "In-house Purchase",
    slug: "demo-inhouse-purchase",
    secret: "demo-secret-inhouse",
    mapping: { click_id: "click_id", transaction_id: "order_id", payout: "revenue", status: "status", currency: "currency", offer_id: "sku" },
    created_at: ago(118),
    updated_at: now,
  },
]);

const healthStates = [
  ["dst_demo_credit_cards", "degraded", "healthy", ""],
  ["dst_demo_invest_landing", "healthy", "degraded", "p95 latency above threshold"],
  ["dst_demo_android_offer", "unknown", "healthy", ""],
  ["dst_demo_ios_offer", "unknown", "healthy", ""],
  ["dst_demo_shop_sale", "degraded", "healthy", ""],
  ["dst_demo_coupon_fallback", "unknown", "unknown", ""],
  ["dst_demo_sweeps_main", "healthy", "healthy", ""],
  ["dst_demo_sweeps_backup", "degraded", "unhealthy", "HTTP 503 from upstream"],
];
db.destination_health.insertMany(healthStates.map(([id, previous, current, error], idx) => ({
  _id: id,
  destination_id: id,
  previous,
  current,
  error,
  checked_at: ago(idx + 1),
  updated_at: now,
})));

const healthHistory = [];
for (let i = 0; i < 36; i += 1) {
  const id = destinationIDs[i % destinationIDs.length];
  const states = ["healthy", "degraded", "healthy", "unhealthy", "healthy", "unknown"];
  const previous = states[i % states.length];
  const current = states[(i + 1) % states.length];
  healthHistory.push({
    demo: true,
    destination_id: id,
    previous,
    current,
    error: current === "unhealthy" ? "timeout after 2500ms" : current === "degraded" ? "slow response" : "",
    checked_at: ago(i * 5 + 2),
    updated_at: ago(i * 5 + 1),
  });
}
db.destination_health_history.insertMany(healthHistory);

const statuses = ["accepted", "accepted", "accepted", "duplicate", "rejected"];
const postbackLogs = [];
for (let i = 0; i < 240; i += 1) {
  const network = networkIDs[i % networkIDs.length];
  const status = statuses[i % statuses.length];
  postbackLogs.push({
    demo: true,
    created_at: ago(i * 2 + 1),
    postback_id: `pb_demo_${String(i + 1).padStart(3, "0")}`,
    network_id: network,
    click_id: `clk_demo_${i * 7}`,
    transaction_id: `tx_demo_${String(i + 1).padStart(4, "0")}`,
    status,
    error: status === "rejected" ? "invalid secret" : status === "duplicate" ? "duplicate transaction" : "",
    raw_payload: {
      cid: `clk_demo_${i * 7}`,
      txid: `tx_demo_${String(i + 1).padStart(4, "0")}`,
      payout: String((12 + (i % 9) * 3.5).toFixed(2)),
      status,
    },
  });
}
db.postback_logs.insertMany(postbackLogs);

print("MongoDB demo seed completed");
JS
}

ensure_clickhouse_schema() {
  if [[ "$RESET_CLICKHOUSE_SCHEMA" == "1" ]]; then
    docker exec -i "$CLICKHOUSE_CONTAINER" clickhouse-client \
      --user "$CLICKHOUSE_USER" \
      --password "$CLICKHOUSE_PASSWORD" \
      --query "DROP DATABASE IF EXISTS $CLICKHOUSE_DATABASE SYNC"
  fi

  for file in deploy/clickhouse/init/*.sql; do
    docker exec -i "$CLICKHOUSE_CONTAINER" clickhouse-client \
      --user "$CLICKHOUSE_USER" \
      --password "$CLICKHOUSE_PASSWORD" \
      --multiquery < "$file"
  done
}

seed_clickhouse() {
  docker exec -i "$CLICKHOUSE_CONTAINER" clickhouse-client \
    --user "$CLICKHOUSE_USER" \
    --password "$CLICKHOUSE_PASSWORD" \
    --database "$CLICKHOUSE_DATABASE" \
    --multiquery <<'SQL'
SET mutations_sync = 1;

ALTER TABLE click_events DELETE WHERE startsWith(campaign_id, 'cmp_demo_') OR startsWith(click_id, 'clk_demo_');
ALTER TABLE conversion_events DELETE WHERE startsWith(campaign_id, 'cmp_demo_') OR startsWith(click_id, 'clk_demo_') OR startsWith(conversion_id, 'cnv_demo_');
ALTER TABLE postback_log_events DELETE WHERE startsWith(postback_id, 'pbe_demo_') OR startsWith(click_id, 'clk_demo_');
ALTER TABLE trafficback_events DELETE WHERE startsWith(campaign_id, 'cmp_demo_') OR startsWith(click_id, 'tb_demo_');
ALTER TABLE destination_health_events DELETE WHERE startsWith(destination_id, 'dst_demo_');

INSERT INTO click_events
WITH
  cityHash64(number) % 20 AS campaign_idx,
  if(campaign_idx + 1 < 10, concat('0', toString(campaign_idx + 1)), toString(campaign_idx + 1)) AS suffix
SELECT
  now() - toIntervalMinute((number % 10080) + 15) AS created_at,
  concat('clk_demo_', toString(number)) AS click_id,
  multiIf(campaign_idx = 0, 'cmp_demo_finance', campaign_idx = 1, 'cmp_demo_mobile', campaign_idx = 2, 'cmp_demo_ecommerce', campaign_idx = 3, 'cmp_demo_sweepstakes', concat('cmp_demo_case_', suffix)) AS campaign_id,
  multiIf(campaign_idx = 0, 'str_demo_finance_us_mobile', campaign_idx = 1, 'str_demo_mobile_android', campaign_idx = 2, 'str_demo_ecommerce_retarget', campaign_idx = 3, 'str_demo_sweepstakes_global', concat('str_demo_case_', suffix)) AS stream_id,
  multiIf(campaign_idx = 0, 'dst_demo_credit_cards', campaign_idx = 1, 'dst_demo_android_offer', campaign_idx = 2, 'dst_demo_shop_sale', campaign_idx = 3, 'dst_demo_sweeps_backup', concat('dst_demo_case_', suffix)) AS destination_id,
  multiIf(number % 4 = 0, 'src_demo_meta', number % 4 = 1, 'src_demo_google', number % 4 = 2, 'src_demo_tiktok', 'src_demo_native') AS source_id,
  concat('external_', toString(100000 + number)) AS source_click_id,
  lower(hex(MD5(concat('ip', toString(number % 230))))) AS ip_hash,
  concat('203.0.113.', toString(number % 255)) AS ip_prefix,
  multiIf(number % 3 = 0, 'Mozilla/5.0 iPhone Safari', number % 3 = 1, 'Mozilla/5.0 Android Chrome', 'Mozilla/5.0 Windows Chrome') AS user_agent,
  lower(hex(MD5(concat('ua', toString(number % 40))))) AS ua_hash,
  multiIf(number % 6 = 0, 'US', number % 6 = 1, 'GB', number % 6 = 2, 'DE', number % 6 = 3, 'CA', number % 6 = 4, 'BR', 'AU') AS geo_country,
  multiIf(number % 4 = 0, 'CA', number % 4 = 1, 'NY', number % 4 = 2, 'BE', 'ON') AS geo_region,
  multiIf(number % 5 = 0, 'New York', number % 5 = 1, 'Los Angeles', number % 5 = 2, 'Berlin', number % 5 = 3, 'Toronto', 'Sydney') AS city,
  toString(64500 + (number % 400)) AS asn,
  multiIf(number % 3 = 0, 'Comcast', number % 3 = 1, 'Deutsche Telekom', 'Vodafone') AS isp,
  multiIf(number % 5 < 3, 'mobile', number % 5 = 3, 'desktop', 'tablet') AS device_type,
  multiIf(number % 4 = 0, 'iOS', number % 4 = 1, 'Android', number % 4 = 2, 'Windows', 'macOS') AS os,
  multiIf(number % 3 = 0, 'Safari', number % 3 = 1, 'Chrome', 'Edge') AS browser,
  multiIf(number % 4 = 0, 'https://facebook.com', number % 4 = 1, 'https://google.com', number % 4 = 2, 'https://tiktok.com', 'https://native.example.com') AS referrer,
  multiIf(number % 3 = 0, 'cart', number % 3 = 1, 'viewed_product', 'cold') AS sub1,
  concat('aud_', toString(number % 17)) AS sub2,
  concat('creative_', toString(number % 12)) AS sub3,
  concat('placement_', toString(number % 9)) AS sub4,
  '' AS sub5,
  '' AS sub6,
  '' AS sub7,
  '' AS sub8,
  '' AS sub9,
  '' AS sub10,
  multiIf(number % 4 = 0, 'meta', number % 4 = 1, 'google', number % 4 = 2, 'tiktok', 'native') AS utm_source,
  multiIf(number % 2 = 0, 'cpc', 'paid_social') AS utm_medium,
  multiIf(campaign_idx = 0, 'finance_scale_q2', campaign_idx = 1, 'apps_tier1', campaign_idx = 2, 'shop_retarget', campaign_idx = 3, 'sweeps_global', concat('demo_case_', suffix)) AS utm_campaign,
  concat('ad_', toString(number % 24)) AS utm_content,
  concat('kw_', toString(number % 16)) AS utm_term,
  round(0.07 + (campaign_idx % 7) * 0.012 + (number % 18) * 0.004, 4) AS cost,
  'USD' AS currency,
  if(number % 97 = 0, 1, 0) AS is_bot,
  if(number % 97 = 0, 0.91, 0.04) AS bot_score,
  concat('source=demo&clickid=external_', toString(100000 + number), '&sub1=', sub1) AS raw_query,
  concat('{"source":"demo","clickid":"external_', toString(100000 + number), '"}') AS query
FROM numbers(12000);

INSERT INTO conversion_events
WITH
  cityHash64(number) % 20 AS campaign_idx,
  if(campaign_idx + 1 < 10, concat('0', toString(campaign_idx + 1)), toString(campaign_idx + 1)) AS suffix
SELECT
  now() - toIntervalMinute((number * 17 % 10080) + 10) AS created_at,
  now() - toIntervalMinute((number * 17 % 10080) + 5) AS updated_at,
  concat('cnv_demo_', toString(number)) AS conversion_id,
  concat('clk_demo_', toString(number * 5)) AS click_id,
  concat('tx_demo_', toString(10000 + number)) AS transaction_id,
  multiIf(campaign_idx = 0, 'cmp_demo_finance', campaign_idx = 1, 'cmp_demo_mobile', campaign_idx = 2, 'cmp_demo_ecommerce', campaign_idx = 3, 'cmp_demo_sweepstakes', concat('cmp_demo_case_', suffix)) AS campaign_id,
  multiIf(campaign_idx = 0, 'str_demo_finance_us_mobile', campaign_idx = 1, 'str_demo_mobile_android', campaign_idx = 2, 'str_demo_ecommerce_retarget', campaign_idx = 3, 'str_demo_sweepstakes_global', concat('str_demo_case_', suffix)) AS stream_id,
  multiIf(campaign_idx = 0, 'dst_demo_credit_cards', campaign_idx = 1, 'dst_demo_android_offer', campaign_idx = 2, 'dst_demo_shop_sale', campaign_idx = 3, 'dst_demo_sweeps_backup', concat('dst_demo_case_', suffix)) AS destination_id,
  multiIf(number % 4 = 0, 'src_demo_meta', number % 4 = 1, 'src_demo_google', number % 4 = 2, 'src_demo_tiktok', 'src_demo_native') AS source_id,
  concat('offer_', toString(number % 12)) AS offer_id,
  multiIf(number % 5 = 0, 'sale', 'lead') AS event_type,
  multiIf(number % 17 = 0, 'rejected', number % 13 = 0, 'pending', 'approved') AS status,
  if(status = 'approved', round(multiIf(campaign_idx IN (3, 7, 12, 18), 0.42 + (campaign_idx % 3) * 0.08, campaign_idx % 5 = 0, 2.30, campaign_idx % 5 = 1, 2.55, campaign_idx % 5 = 2, 2.75, campaign_idx % 5 = 3, 3.00, 3.20) + (number % 7) * 0.03, 2), 0) AS payout,
  'USD' AS currency,
  multiIf(number % 3 = 0, 'net_demo_adcombo', number % 3 = 1, 'net_demo_clickdealer', 'net_demo_inhouse') AS network_id,
  concat('{"demo":"true","transaction_id":"tx_demo_', toString(10000 + number), '"}') AS raw_payload
FROM numbers(1800);

INSERT INTO postback_log_events
SELECT
  now() - toIntervalMinute((number * 19 % 10080) + 3) AS created_at,
  concat('pbe_demo_', toString(number)) AS postback_id,
  multiIf(number % 3 = 0, 'net_demo_adcombo', number % 3 = 1, 'net_demo_clickdealer', 'net_demo_inhouse') AS network_id,
  concat('clk_demo_', toString(number * 5)) AS click_id,
  concat('tx_demo_', toString(20000 + number)) AS transaction_id,
  multiIf(number % 17 = 0, 'rejected', number % 13 = 0, 'duplicate', 'accepted') AS status,
  multiIf(status = 'rejected', 'invalid secret', status = 'duplicate', 'duplicate transaction', '') AS error,
  concat('{"demo":"true","status":"', status, '"}') AS raw_payload
FROM numbers(140);

INSERT INTO trafficback_events
SELECT
  now() - toIntervalMinute((number * 37 % 10080) + 7) AS created_at,
  concat('tb_demo_', toString(number)) AS click_id,
  multiIf(number % 3 = 0, 'cmp_demo_finance', number % 3 = 1, 'cmp_demo_ecommerce', 'cmp_demo_sweepstakes') AS campaign_id,
  multiIf(number % 3 = 0, 'str_demo_finance_us_mobile', number % 3 = 1, 'str_demo_ecommerce_retarget', 'str_demo_sweepstakes_global') AS stream_id,
  multiIf(number % 3 = 0, 'dst_demo_invest_landing', number % 3 = 1, 'dst_demo_coupon_fallback', 'dst_demo_sweeps_backup') AS destination_id,
  multiIf(number % 4 = 0, 'destination_unhealthy', number % 4 = 1, 'offer_cap_reached', number % 4 = 2, 'no_matching_stream', 'timeout_or_error') AS reason,
  toUInt8(1 + (number % 2)) AS depth,
  [destination_id] AS visited_destinations
FROM numbers(75);

INSERT INTO destination_health_events
SELECT
  now() - toIntervalMinute((number * 53 % 10080) + 11) AS created_at,
  multiIf(number % 8 = 0, 'dst_demo_credit_cards', number % 8 = 1, 'dst_demo_invest_landing', number % 8 = 2, 'dst_demo_android_offer', number % 8 = 3, 'dst_demo_ios_offer', number % 8 = 4, 'dst_demo_shop_sale', number % 8 = 5, 'dst_demo_coupon_fallback', number % 8 = 6, 'dst_demo_sweeps_main', 'dst_demo_sweeps_backup') AS destination_id,
  multiIf(number % 5 = 0, 'unknown', number % 5 = 1, 'healthy', number % 5 = 2, 'degraded', number % 5 = 3, 'healthy', 'unhealthy') AS previous,
  multiIf(number % 5 = 0, 'healthy', number % 5 = 1, 'degraded', number % 5 = 2, 'healthy', number % 5 = 3, 'unhealthy', 'healthy') AS current,
  multiIf(current = 'unhealthy', 'HTTP 503 from upstream', current = 'degraded', 'slow response', '') AS error
FROM numbers(96);
SQL
}

require_container "$MONGO_CONTAINER"
require_container "$CLICKHOUSE_CONTAINER"
seed_mongo
ensure_clickhouse_schema
seed_clickhouse

echo "Demo data seeded successfully."
