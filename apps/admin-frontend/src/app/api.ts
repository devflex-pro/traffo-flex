const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8070";
let actingAsUserID = "";

export function setActingAsUserID(id: string | null) {
  actingAsUserID = id ?? "";
}

export type Status = "active" | "paused" | "archived";
export type HealthStatus = "healthy" | "degraded" | "unhealthy" | "unknown";
export type RedirectMode = "http_302" | "meta_refresh" | "javascript" | "interstitial";
export type DistributionMode = "direct" | "weighted" | "fallback" | "waterfall" | "round_robin" | "best_roi";

export type Metrics = {
  clicks: number;
  conversions: number;
  revenue: number;
  cost: number;
  profit: number;
  roi: number;
  events?: number;
};

export type ListResponse<T> = {
  items: T[];
  limit: number;
  offset: number;
  total: number;
};

export type ListFilters = {
  limit?: number;
  offset?: number;
};

export type ReportRow = {
  id: string;
  name: string;
  metrics: Metrics;
};

export type GroupedReport = {
  group_by: string;
  summary: Metrics;
  rows: ReportRow[];
  filters: Record<string, string>;
};

export type ReportFilters = {
  from?: string;
  to?: string;
  timezone?: string;
  campaign_id?: string;
  stream_id?: string;
  destination_id?: string;
  source_id?: string;
};

export type IngestionErrorRow = {
  observed_at: string;
  topic: string;
  error: string;
  raw_message: string;
};

export type IngestionErrorsReport = {
  rows: IngestionErrorRow[];
  filters: Record<string, string>;
};

export type Campaign = {
  id: string;
  public_id?: string;
  public_token?: string;
  name: string;
  slug: string;
  status: Status;
  traffic_source_id?: string;
  currency?: string;
  default_action?: string;
  trafficback_config: TrafficbackConfig;
  tracking_params?: TrackingParam[] | null;
};

export type TrackingParam = {
  key: string;
  value: string;
};

export type CampaignStructure = {
  campaign_id: string;
  stream_count: number;
  destination_count: number;
};

export type ClientConfig = {
  tracker_base_url: string;
};

export type TrafficbackConfig = {
  enabled: boolean;
  url: string;
  max_depth: number;
};

export type CampaignRequest = {
  name: string;
  slug: string;
  status: Status;
  traffic_source_id?: string;
  currency?: string;
  default_action?: string;
  trafficback_config: TrafficbackConfig;
};

export type Destination = {
  id: string;
  name: string;
  type: string;
  url: string;
  healthcheck_url?: string;
  manual_status: Status;
  health_status: HealthStatus;
  redirect: { mode: RedirectMode };
  schedule: DestinationSchedule;
  caps: DestinationCaps;
};

export type DestinationRequest = {
  name: string;
  type: string;
  url: string;
  healthcheck_url?: string;
  manual_status: Status;
  health_status: HealthStatus;
  redirect: { mode: RedirectMode };
  schedule: DestinationSchedule;
  caps: DestinationCaps;
};

export type DestinationHealthcheckResult = {
  destination_id: string;
  status: HealthStatus;
  error?: string;
  probe_status_code?: number;
  consecutive_failures: number;
  consecutive_successes: number;
};

export type DestinationSchedule = {
  enabled: boolean;
  timezone?: string;
  windows?: Array<{
    weekdays: Array<"mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun">;
    start_time: string;
    end_time: string;
  }>;
};

export type DestinationCaps = {
  enabled: boolean;
  rules?: Array<{
    metric: "clicks" | "cost" | "conversions" | "revenue";
    window_hours: number;
    limit: number;
  }>;
};

export type WeightedTarget = {
  destination_id: string;
  weight: number;
};

export type UniqueDestinationPolicy = {
  enabled: boolean;
  user_key?: string;
  history_window_hours?: number;
  exhausted_mode?: "allow_repeat" | "no_destination";
  selection_strategy?: DistributionMode;
  roi_window_hours?: number;
  min_clicks?: number;
  fallback_strategy?: Exclude<DistributionMode, "best_roi">;
};

export type Stream = {
  id: string;
  campaign_id: string;
  name: string;
  priority: number;
  status: Status;
  conditions: Array<{ field: string; operator: string; value?: string; values?: string[] }>;
  distribution: {
    mode: DistributionMode;
    destinations: WeightedTarget[];
    unique_policy: UniqueDestinationPolicy;
  };
};

export type StreamRequest = {
  name: string;
  priority: number;
  status: Status;
  conditions: Array<{ field: string; operator: string; value?: string; values?: string[] }>;
  distribution: {
    mode: DistributionMode;
    destinations: WeightedTarget[];
    unique_policy?: UniqueDestinationPolicy;
  };
};

export type TrafficSource = {
  id: string;
  name: string;
  slug: string;
};

export type PostbackTemplate = {
  id: string;
  network_id: string;
  name: string;
  slug: string;
  secret?: string;
  mapping: Record<string, string>;
};

export type PostbackLogRow = {
  created_at: string;
  postback_id: string;
  network_id: string;
  click_id?: string;
  transaction_id?: string;
  status: string;
  error?: string;
  raw_payload: Record<string, string>;
};

export type PostbackLogsReport = {
  items: PostbackLogRow[];
  filters: Record<string, string>;
  limit: number;
  offset: number;
  total: number;
};

export type HealthHistoryRow = {
  destination_id: string;
  previous: HealthStatus;
  current: HealthStatus;
  error?: string;
  checked_at: string;
  updated_at: string;
};

export type HealthHistoryReport = {
  items: HealthHistoryRow[];
  filters: Record<string, string>;
  limit: number;
  offset: number;
  total: number;
};

export type AuthUser = {
  id: string;
  email: string;
  role: "admin" | "user";
  status: "active" | "pending_approval";
  email_verified: boolean;
  approved: boolean;
};

export type OTPChallenge = {
  email: string;
  otp?: string;
};

export type AuthSession = {
  token?: string;
  user: AuthUser;
};

export type PostbackLogFilters = {
  network_id?: string;
  click_id?: string;
  transaction_id?: string;
  status?: string;
  limit?: number;
  offset?: number;
};

export type HealthHistoryFilters = {
  destination_id?: string;
  current?: string;
  limit?: number;
  offset?: number;
};

export type PostbackTemplateRequest = {
  network_id: string;
  name: string;
  slug: string;
  secret?: string;
  mapping: Record<string, string>;
};

type ApiError = {
  error?: string;
  status?: string;
  user?: AuthUser;
};

export class ApiRequestError extends Error {
  statusCode: number;
  response?: ApiError;

  constructor(
    message: string,
    statusCode: number,
    response?: ApiError
  ) {
    super(message);
    this.statusCode = statusCode;
    this.response = response;
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const res = await fetch(
    `${API_BASE_URL}${path}`,
    {
      ...options,
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(options.method && options.method !== "GET" ? { "X-TraffoFlex-CSRF": "1" } : {}),
        ...(actingAsUserID ? { "X-TraffoFlex-Act-As": actingAsUserID } : {}),
        ...(options.headers ?? {})
      }
    }
  );
  if (!res.ok) {
    let message = `Request failed with status ${res.status}`;
    let body: ApiError | undefined;
    try {
      body = (await res.json()) as ApiError;
      if (body.error) {
        message = body.error;
      }
    } catch (err) {
      if (err instanceof Error) {
        message = `${message}: ${err.message}`;
      }
    }
    throw new ApiRequestError(
      message,
      res.status,
      body
    );
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

function jsonBody(data: unknown): RequestInit {
  return {
    body: JSON.stringify(data)
  };
}

function queryString(filters: Record<string, string | number | undefined>): string {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== "") {
      params.set(
        key,
        String(value)
      );
    }
  });
  const query = params.toString();
  return query ? `?${query}` : "";
}

export const api = {
  clientConfig: () => request<ClientConfig>("/api/client-config"),
  requestOTP: (email: string) =>
    request<OTPChallenge>(
      "/api/auth/request-otp",
      {
        method: "POST",
        ...jsonBody({ email })
      }
    ),
  verifyOTP: (
    email: string,
    otp: string
  ) =>
    request<AuthSession>(
      "/api/auth/verify-otp",
      {
        method: "POST",
        ...jsonBody({
          email,
          otp
        })
      }
    ),
  logout: () => request<void>("/api/auth/logout", { method: "POST" }),
  me: () => request<AuthUser>("/api/me"),
  users: (filters: ListFilters = {}) =>
    request<ListResponse<AuthUser>>(`/api/users${queryString(filters)}`),
  approveUser: (id: string) =>
    request<AuthUser>(
      `/api/users/${id}/approve`,
      {
        method: "POST"
      }
    ),
  overview: (filters: ReportFilters = {}) =>
    request<Metrics>(`/api/reports/overview${queryString(filters)}`),
  dailyReport: (filters: ReportFilters = {}) =>
    request<GroupedReport>(`/api/reports/daily${queryString(filters)}`),
  report: (
    group: string,
    filters: ReportFilters = {}
  ) => request<GroupedReport>(`/api/reports/${group}${queryString(filters)}`),
  ingestionErrors: () => request<IngestionErrorsReport>("/api/reports/ingestion-errors"),
  campaigns: (filters: ListFilters = {}) =>
    request<ListResponse<Campaign>>(`/api/campaigns${queryString(filters)}`),
  campaignStructure: () => request<{ items: CampaignStructure[] }>("/api/campaigns/structure"),
  campaign: (id: string) => request<Campaign>(`/api/campaigns/${id}`),
  createCampaign: (data: CampaignRequest) =>
    request<Campaign>(
      "/api/campaigns",
      {
        method: "POST",
        ...jsonBody(data)
      }
    ),
  updateCampaign: (
    id: string,
    data: CampaignRequest
  ) =>
    request<Campaign>(
      `/api/campaigns/${id}`,
      {
        method: "PUT",
        ...jsonBody(data)
      }
    ),
  updateTrackingParams: (
    id: string,
    trackingParams: TrackingParam[]
  ) => request<Campaign>(
    `/api/campaigns/${id}/tracking-params`,
    {
      method: "PUT",
      ...jsonBody({ tracking_params: trackingParams })
    }
  ),
  deleteCampaign: (id: string) =>
    request<void>(
      `/api/campaigns/${id}`,
      {
        method: "DELETE"
      }
    ),
  streams: (
    campaignID: string,
    filters: ListFilters = {}
  ) =>
    request<ListResponse<Stream>>(`/api/campaigns/${campaignID}/streams${queryString(filters)}`),
  createStream: (
    campaignID: string,
    data: StreamRequest
  ) =>
    request<Stream>(
      `/api/campaigns/${campaignID}/streams`,
      {
        method: "POST",
        ...jsonBody(data)
      }
    ),
  updateStream: (
    id: string,
    data: StreamRequest
  ) =>
    request<Stream>(
      `/api/streams/${id}`,
      {
        method: "PUT",
        ...jsonBody(data)
      }
    ),
  deleteStream: (id: string) =>
    request<void>(
      `/api/streams/${id}`,
      {
        method: "DELETE"
      }
    ),
  destinations: (filters: ListFilters = {}) =>
    request<ListResponse<Destination>>(`/api/destinations${queryString(filters)}`),
  createDestination: (data: DestinationRequest) =>
    request<Destination>(
      "/api/destinations",
      {
        method: "POST",
        ...jsonBody(data)
      }
    ),
  updateDestination: (
    id: string,
    data: DestinationRequest
  ) =>
    request<Destination>(
      `/api/destinations/${id}`,
      {
        method: "PUT",
        ...jsonBody(data)
      }
    ),
  deleteDestination: (id: string) =>
    request<void>(
      `/api/destinations/${id}`,
      {
        method: "DELETE"
      }
    ),
  reloadTrafficCache: () =>
    request<{ status: string }>(
      "/api/internal/traffic/cache/reload",
      { method: "POST" }
    ),
  triggerDestinationHealthcheck: (id: string) =>
    request<DestinationHealthcheckResult>(
      `/api/destinations/${id}/healthcheck`,
      {
        method: "POST"
      }
    ),
  trafficSources: (filters: ListFilters = {}) =>
    request<ListResponse<TrafficSource>>(`/api/traffic-sources${queryString(filters)}`),
  postbackLogs: (filters: PostbackLogFilters = {}) =>
    request<PostbackLogsReport>(`/api/postback-logs${queryString(filters)}`),
  healthHistory: (filters: HealthHistoryFilters = {}) =>
    request<HealthHistoryReport>(`/api/destinations/health-history${queryString(filters)}`),
  postbackTemplates: (filters: ListFilters = {}) =>
    request<ListResponse<PostbackTemplate>>(`/api/postback-templates${queryString(filters)}`),
  createPostbackTemplate: (data: PostbackTemplateRequest) =>
    request<PostbackTemplate>(
      "/api/postback-templates",
      {
        method: "POST",
        ...jsonBody(data)
      }
    ),
  updatePostbackTemplate: (
    id: string,
    data: PostbackTemplateRequest
  ) =>
    request<PostbackTemplate>(
      `/api/postback-templates/${id}`,
      {
        method: "PUT",
        ...jsonBody(data)
      }
    ),
  deletePostbackTemplate: (id: string) =>
    request<void>(
      `/api/postback-templates/${id}`,
      {
        method: "DELETE"
      }
    )
};
