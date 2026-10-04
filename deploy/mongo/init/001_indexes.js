db = db.getSiblingDB("traffoflex");

db.campaigns.createIndex(
  { slug: 1 },
  { unique: true, name: "campaigns_slug_unique" }
);
db.campaigns.createIndex(
  { public_id: 1 },
  { sparse: true, unique: true, name: "campaigns_public_id_unique" }
);
db.campaigns.createIndex(
  { public_token: 1 },
  { sparse: true, unique: true, name: "campaigns_public_token_unique" }
);
db.campaigns.createIndex(
  { status: 1, updated_at: -1 },
  { name: "campaigns_status_updated_at" }
);
db.campaigns.createIndex(
  { owner_id: 1, created_at: 1 },
  { name: "campaigns_owner_created_at" }
);

db.streams.createIndex(
  { campaign_id: 1, priority: 1 },
  { name: "streams_campaign_priority" }
);
db.streams.createIndex(
  { owner_id: 1, campaign_id: 1, created_at: 1 },
  { name: "streams_owner_campaign_created_at" }
);
db.streams.createIndex(
  { status: 1 },
  { name: "streams_status" }
);

db.destinations.createIndex(
  { manual_status: 1, health_status: 1 },
  { name: "destinations_status_health" }
);
db.destinations.createIndex(
  { owner_id: 1, created_at: 1 },
  { name: "destinations_owner_created_at" }
);

db.traffic_sources.createIndex(
  { owner_id: 1, slug: 1 },
  { unique: true, name: "traffic_sources_owner_slug_unique" }
);

db.affiliate_networks.createIndex(
  { owner_id: 1, slug: 1 },
  { unique: true, name: "affiliate_networks_owner_slug_unique" }
);

db.postback_templates.createIndex(
  { owner_id: 1, network_id: 1, slug: 1 },
  { unique: true, name: "postback_templates_owner_network_slug_unique" }
);
db.postback_templates.createIndex(
  { owner_id: 1, created_at: 1 },
  { name: "postback_templates_owner_created_at" }
);

db.conversions.createIndex(
  { owner_id: 1, network_id: 1, transaction_id: 1 },
  { unique: true, name: "conversions_network_transaction_unique" }
);
db.conversions.createIndex(
  { click_id: 1 },
  { name: "conversions_click_id" }
);
db.conversions.createIndex(
  { delivery_status: 1, created_at: 1 },
  { name: "conversions_pending_delivery" }
);
db.conversions.createIndex(
  { attribution_status: 1, attribution_next_attempt_at: 1 },
  { name: "conversions_pending_attribution" }
);

db.postback_logs.createIndex(
  { created_at: -1 },
  { name: "postback_logs_created_at" }
);
db.postback_logs.createIndex(
  { owner_id: 1, created_at: -1 },
  { name: "postback_logs_owner_created_at" }
);
db.postback_logs.createIndex(
  { network_id: 1, created_at: -1 },
  { name: "postback_logs_network_created_at" }
);
db.postback_logs.createIndex(
  { click_id: 1, created_at: -1 },
  { name: "postback_logs_click_created_at" }
);
db.postback_logs.createIndex(
  { network_id: 1, transaction_id: 1 },
  { name: "postback_logs_network_transaction" }
);
db.postback_logs.createIndex(
  { delivery_status: 1, created_at: 1 },
  { name: "postback_logs_pending_delivery" }
);

db.destination_health.createIndex(
  { destination_id: 1 },
  { unique: true, name: "destination_health_destination_unique" }
);
db.destination_health.createIndex(
  { checked_at: -1 },
  { name: "destination_health_checked_at" }
);

db.destination_health_history.createIndex(
  { destination_id: 1, checked_at: -1 },
  { name: "destination_health_history_destination_checked_at" }
);
db.destination_health_history.createIndex(
  { checked_at: -1 },
  { name: "destination_health_history_checked_at" }
);
db.destination_health_history.createIndex(
  { owner_id: 1, checked_at: -1 },
  { name: "destination_health_history_owner_checked_at" }
);

db.users.createIndex(
  { email: 1 },
  { unique: true, name: "users_email_unique" }
);
db.users.createIndex(
  { status: 1, created_at: -1 },
  { name: "users_status_created_at" }
);
