import React, { useContext, useEffect, useId, useMemo, useRef, useState } from "react";
import ReactDOM from "react-dom/client";
import {
  QueryClient,
  QueryClientProvider,
  useMutation,
  useQuery,
  useQueryClient
} from "@tanstack/react-query";
import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  useReactTable
} from "@tanstack/react-table";
import { useForm } from "react-hook-form";
import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
  useNavigate
} from "react-router-dom";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { z } from "zod";
import {
  Campaign,
  CampaignRequest,
  Destination,
  DestinationHealthcheckResult,
  DestinationCaps,
  DestinationRequest,
  DestinationSchedule,
  DistributionMode,
  HealthStatus,
  HealthHistoryRow,
  IngestionErrorRow,
  Metrics,
  PostbackLogRow,
  PostbackTemplate,
  ReportRow,
  Status,
  Stream,
  StreamRequest,
  api,
  ApiRequestError,
  AuthUser,
  setActingAsUserID
} from "./api";
import "./styles.css";

const queryClient = new QueryClient();
const RoutingSyncContext = React.createContext<((afterSave?: boolean) => Promise<void>) | null>(null);

function useRoutingSync() {
  const sync = useContext(RoutingSyncContext);
  if (!sync) {
    throw new Error("Routing sync is unavailable");
  }
  return sync;
}
const statusOptions: Status[] = ["active", "paused", "archived"];
const distributionOptions: DistributionMode[] = [
  "direct",
  "weighted",
  "fallback",
  "waterfall",
  "round_robin",
  "best_roi"
];
const conditionOperators = [
  "eq",
  "neq",
  "in",
  "not_in",
  "contains",
  "not_contains",
  "starts_with",
  "ends_with",
  "gt",
  "gte",
  "lt",
  "lte",
  "exists",
  "not_exists",
  "regex"
];
const conditionFields = [
  "source_id",
  "source_click_id",
  "campaign_id",
  "stream_id",
  "destination_id",
  "ip",
  "geo_country",
  "geo_region",
  "city",
  "asn",
  "isp",
  "device_type",
  "os",
  "browser",
  "user_agent",
  "referrer",
  "sub1",
  "sub2",
  "sub3",
  "sub4",
  "sub5",
  "sub6",
  "sub7",
  "sub8",
  "sub9",
  "sub10",
  "utm_source",
  "utm_medium",
  "utm_campaign",
  "utm_content",
  "utm_term",
  "cost",
  "currency",
  "trafficback_depth",
  "query.country",
  "query.device",
  "query.browser",
  "query.zone",
  "query.placement",
  "query.offer",
  "query.campaign"
];
const weekdays = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun"
] as const;

type MetricKey = keyof Metrics;

function formatFixed(value: number) {
  return Number.isFinite(value) ? value.toFixed(2) : "0.00";
}

function formatInteger(value: number) {
  return Number.isFinite(value) ? Math.round(value).toLocaleString("en-US") : "0";
}

function formatMetricValue(
  key: MetricKey,
  value: number
) {
  switch (key) {
    case "clicks":
    case "conversions":
    case "events":
      return formatInteger(value);
    case "roi":
      return `${formatFixed(value)}%`;
    case "revenue":
    case "cost":
    case "profit":
      return `$${formatFixed(value)}`;
    default:
      return formatFixed(value);
  }
}

function humanizeID(value: string) {
  return value
    .replace(/^cmp_demo_/, "")
    .replace(/^str_demo_/, "")
    .replace(/^dst_demo_/, "")
    .replace(/^src_demo_/, "")
    .replace(/_/g, " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function dateInputValue(value: Date) {
  return value.toISOString().slice(
    0,
    10
  );
}

function defaultReportPeriod() {
  const to = new Date();
  const from = new Date(to);
  from.setUTCDate(from.getUTCDate() - 6);
  from.setUTCHours(
    0,
    0,
    0,
    0
  );
  return {
    from: dateInputValue(from),
    to: dateInputValue(to)
  };
}

function reportDateRange(
  from: string,
  to: string
) {
  return {
    from: new Date(`${from}T00:00:00.000Z`).toISOString(),
    to: new Date(`${to}T23:59:59.999Z`).toISOString(),
    timezone: "UTC"
  };
}

function App() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [logoutError, setLogoutError] = useState("");
  const [actingAs, setActingAs] = useState<AuthUser | null>(null);

  useEffect(() => {
    let active = true;
    api.me().then(
      () => { if (active) setAuthenticated(true); },
      () => { if (active) setAuthenticated(false); }
    );
    return () => { active = false; };
  }, []);

  function onLogin() {
    setLogoutError("");
    setAuthenticated(true);
  }

  async function onLogout() {
    try {
      await api.logout();
      setActingAsUserID(null);
      setActingAs(null);
      queryClient.clear();
      setAuthenticated(false);
      setLogoutError("");
    } catch {
      setLogoutError("Logout failed. Please retry.");
    }
  }

  async function onActAs(user: AuthUser | null) {
    await queryClient.cancelQueries();
    setActingAsUserID(user?.id ?? null);
    setActingAs(user);
    queryClient.clear();
  }

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        {logoutError ? <p role="alert" className="bg-red-50 p-2 text-red-700">{logoutError}</p> : null}
        <Routes>
          <Route path="/login" element={<LoginPage onLogin={onLogin} />} />
          <Route
            path="/*"
            element={
              authenticated === null ? (
                <p>Loading session…</p>
              ) : authenticated ? (
                <Layout onLogout={onLogout} actingAs={actingAs} onActAs={onActAs} />
              ) : (
                <Navigate to="/login" replace />
              )
            }
          />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function LoginPage({ onLogin }: { onLogin: () => void }) {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [devOTP, setDevOTP] = useState("");
  const [pendingApproval, setPendingApproval] = useState<AuthUser | null>(null);
  const form = useForm<{ email: string; otp: string }>({
    defaultValues: {
      email: "admin@example.com",
      otp: ""
    }
  });
  const requestOTP = useMutation({
    mutationFn: (values: { email: string }) => api.requestOTP(values.email),
    onSuccess: (data) => {
      setEmail(data.email);
      setDevOTP(data.otp ?? "");
      setPendingApproval(null);
      form.setValue(
        "email",
        data.email
      );
      if (data.otp) {
        form.setValue(
          "otp",
          data.otp
        );
      }
    }
  });
  const verifyOTP = useMutation({
    mutationFn: (values: { email: string; otp: string }) =>
      api.verifyOTP(
        values.email,
        values.otp
      ),
    onSuccess: () => {
      onLogin();
      navigate("/");
    },
    onError: (error) => {
      if (
        error instanceof ApiRequestError &&
        error.response?.status === "pending_approval" &&
        error.response.user
      ) {
        setPendingApproval(error.response.user);
      }
    }
  });
  const error = requestOTP.error ?? verifyOTP.error;

  return (
    <main className="grid min-h-screen place-items-center bg-zinc-100 px-4 text-zinc-950">
      <form
        className="w-full max-w-sm rounded-md border border-zinc-200 bg-white p-6 shadow-sm"
        onSubmit={form.handleSubmit((values) => {
          if (email) {
            verifyOTP.mutate(values);
            return;
          }
          requestOTP.mutate(values);
        })}
      >
        <div className="text-2xl font-semibold">TraffoFlex</div>
        <label className="mt-6 block text-sm font-medium">
          Email
          <input className="input mt-2" {...form.register("email")} />
        </label>
        {email ? (
          <label className="mt-4 block text-sm font-medium">
            OTP
            <input className="input mt-2" {...form.register("otp")} />
          </label>
        ) : null}
        {devOTP ? <p className="mt-3 rounded-md bg-zinc-100 p-3 text-sm text-zinc-700">Local OTP: {devOTP}</p> : null}
        {pendingApproval ? (
          <p className="mt-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
            Email confirmed. Admin activation is required before this account can enter.
          </p>
        ) : null}
        {error ? <p className="mt-3 text-sm text-red-700">{error.message}</p> : null}
        <button className="button-primary mt-6 w-full" type="submit">
          {email ? "Verify OTP" : "Send OTP"}
        </button>
        {email ? (
          <button
            className="button-secondary mt-3 w-full"
            onClick={() => {
              setEmail("");
              setDevOTP("");
              setPendingApproval(null);
              form.setValue(
                "otp",
                ""
              );
            }}
            type="button"
          >
            Change email
          </button>
        ) : null}
      </form>
    </main>
  );
}

function Layout({ onLogout, actingAs, onActAs }: {
  onLogout: () => void;
  actingAs: AuthUser | null;
  onActAs: (user: AuthUser | null) => Promise<void>;
}) {
  const [routingStatus, setRoutingStatus] = useState<"idle" | "pending" | "success" | "error">("idle");
  const [routingMessage, setRoutingMessage] = useState("");
  const [routingPending, setRoutingPending] = useState(0);
  const latestRoutingRequest = useRef(0);
  const me = useQuery({
    queryKey: ["me"],
    queryFn: api.me
  });

  const syncRouting = async (afterSave = false) => {
    const requestID = ++latestRoutingRequest.current;
    setRoutingPending((current) => current + 1);
    setRoutingStatus("pending");
    setRoutingMessage("Refreshing routing snapshot…");
    try {
      await api.reloadTrafficCache();
      if (requestID === latestRoutingRequest.current) {
        setRoutingStatus("success");
        setRoutingMessage("Routing snapshot refreshed.");
      }
    } catch (error) {
      if (requestID === latestRoutingRequest.current) {
        setRoutingStatus("error");
        setRoutingMessage(`${afterSave ? "Change saved in MongoDB, but routing refresh failed." : "Routing refresh failed."} Retry here. ${error instanceof Error ? error.message : ""}`);
      }
    } finally {
      setRoutingPending((current) => current - 1);
    }
  };

  return (
    <div className="min-h-screen bg-zinc-100 text-zinc-950">
      <aside className="fixed left-0 top-0 h-full w-64 border-r border-zinc-200 bg-white p-5">
        <div className="text-xl font-semibold">TraffoFlex</div>
        <div className="mt-2 text-xs text-zinc-500">{me.data?.email}</div>
        {actingAs ? (
          <div className="mt-3 rounded-md border border-amber-300 bg-amber-50 p-2 text-xs text-amber-900">
            Working as {actingAs.email}
            <button className="mt-2 block underline" onClick={() => void onActAs(null)} type="button">
              Return to my data
            </button>
          </div>
        ) : null}
        <nav className="mt-8 flex flex-col gap-1 text-sm">
          <NavLink to="/">Dashboard</NavLink>
          <NavLink to="/campaigns">Campaigns</NavLink>
          <NavLink to="/destinations">Destinations</NavLink>
          <NavLink to="/postbacks">Postbacks</NavLink>
          <NavLink to="/health-history">Health History</NavLink>
          <NavLink to="/reports">Reports</NavLink>
          {me.data?.role === "admin" ? <NavLink to="/ingestion">Ingestion</NavLink> : null}
          {me.data?.role === "admin" ? <NavLink to="/users">Users</NavLink> : null}
        </nav>
        <button className="button-secondary mt-8 w-full" onClick={onLogout} type="button">
          Sign out
        </button>
      </aside>
      <main className="ml-64 p-6">
        <div className="mb-5 flex items-center justify-end gap-3">
          <span aria-live="polite" className={`text-sm ${routingStatus === "error" ? "text-red-700" : "text-zinc-600"}`}>
            {routingMessage}
          </span>
          <button
            className="button-secondary"
            disabled={routingPending > 0}
            onClick={() => void syncRouting()}
            type="button"
          >
            {routingPending > 0 ? "Refreshing…" : "Refresh routing snapshot"}
          </button>
        </div>
        <RoutingSyncContext.Provider value={syncRouting}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/campaigns" element={<CampaignsPage />} />
            <Route path="/destinations" element={<DestinationsPage />} />
            <Route path="/postbacks" element={<PostbacksPage />} />
            <Route path="/health-history" element={<HealthHistoryPage />} />
            <Route path="/reports" element={<ReportsPage />} />
            <Route path="/ingestion" element={<IngestionPage />} />
            <Route path="/users" element={<UsersPage onActAs={onActAs} />} />
          </Routes>
        </RoutingSyncContext.Provider>
      </main>
    </div>
  );
}

function NavLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link className="rounded-md px-3 py-2 text-zinc-700 hover:bg-zinc-100 hover:text-zinc-950" to={to}>
      {children}
    </Link>
  );
}

function Dashboard() {
  const overview = useQuery({
    queryKey: ["overview"],
    queryFn: () => api.overview()
  });
  const campaigns = useQuery({
    queryKey: ["campaigns"],
    queryFn: () => api.campaigns()
  });
  const destinations = useQuery({
    queryKey: ["destinations"],
    queryFn: () => api.destinations()
  });

  return (
    <Page title="Dashboard">
      <MetricGrid metrics={overview.data} />
      <div className="mt-6 grid grid-cols-2 gap-4">
        <Panel title="Campaigns">
          <div className="text-3xl font-semibold">{campaigns.data?.total ?? 0}</div>
        </Panel>
        <Panel title="Destinations">
          <div className="text-3xl font-semibold">{destinations.data?.total ?? 0}</div>
        </Panel>
      </div>
    </Page>
  );
}

function UsersPage({ onActAs }: { onActAs: (user: AuthUser) => Promise<void> }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const users = useQuery({
    queryKey: ["users"],
    queryFn: () => api.users()
  });
  const approve = useMutation({
    mutationFn: api.approveUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] })
  });
  const columns = useMemo<ColumnDef<AuthUser>[]>(
    () => [
      { header: "Email", accessorKey: "email" },
      { header: "Role", accessorKey: "role" },
      { header: "Status", accessorKey: "status" },
      {
        header: "Verified",
        cell: ({ row }) => (row.original.email_verified ? "Yes" : "No")
      },
      {
        header: "Actions",
        cell: ({ row }) =>
          row.original.approved ? (
            <ActionIconButton
              ariaLabel={`Open workspace for ${row.original.email}`}
              icon="workspace"
              label="Open workspace"
              onClick={() => {
                void onActAs(row.original).then(() => navigate("/"));
              }}
            />
          ) : (
            <ActionIconButton
              ariaLabel={`Approve ${row.original.email}`}
              icon="approve"
              label="Approve user"
              onClick={() => approve.mutate(row.original.id)}
            />
          )
      }
    ],
    [approve]
  );

  return (
    <Page title="Users">
      {approve.error ? <p className="mb-4 text-sm text-red-700">{approve.error.message}</p> : null}
      <DataTable columns={columns} data={users.data?.items ?? []} />
    </Page>
  );
}

const rishadsOptionalMacros = [
  { label: "Publisher ID", param: "sub2", macro: "[PUBLISHER_ID]" },
  { label: "Site ID", param: "sub3", macro: "[SITE_ID]" },
  { label: "Creative ID", param: "sub4", macro: "[CREATIVE_ID]" },
  { label: "Device", param: "device_type", macro: "[DEVICE]" },
  { label: "Browser", param: "browser", macro: "[BROWSER]" },
  { label: "OS", param: "os", macro: "[OS]" },
  { label: "ISP", param: "isp", macro: "[ISP]" },
  { label: "Carrier", param: "carrier", macro: "[CARRIER]" },
  { label: "Connection type", param: "source_connection_type", macro: "[CONNECTION_TYPE]" },
  { label: "Source campaign ID", param: "source_campaign_id", macro: "[CAMPAIGN_ID]" },
  { label: "Source campaign name", param: "source_campaign_name", macro: "[CAMPAIGN_NAME]" },
  { label: "CPV price per 1000 impressions", param: "source_cpv_price", macro: "[CPV_PRICE]" },
  { label: "Source IP (raw only)", param: "source_ip", macro: "[IP]" }
] as const;

function localTrackerURL() {
  if (window.location.hostname !== "localhost" && window.location.hostname !== "127.0.0.1") {
    return "";
  }
  return `http://${window.location.hostname}:8080`;
}

function encodeTrackingParameter(value: string) {
  return encodeURIComponent(value.trim())
    .replace(/%7B/gi, "{")
    .replace(/%7D/gi, "}")
    .replace(/%5B/gi, "[")
    .replace(/%5D/gi, "]");
}

function campaignTrackingURL(baseURL: string, slug: string) {
  return `${baseURL.replace(/\/+$/, "")}/c/${encodeURIComponent(slug)}`;
}

function CampaignTrackingCell({ campaign, baseURL }: { campaign: Campaign; baseURL: string }) {
  const [copyStatus, setCopyStatus] = useState("");
  if (!baseURL) {
    return <span className="text-zinc-500">Tracker URL unavailable</span>;
  }
  const url = campaignTrackingURL(baseURL, campaign.slug);
  return (
    <div className="flex min-w-[19rem] items-center gap-2">
      <input
        aria-label={`Tracking URL for ${campaign.name}`}
        className="input min-w-0 flex-1 font-mono text-xs"
        onFocus={(event) => event.currentTarget.select()}
        readOnly
        value={url}
      />
      <ActionIconButton
        ariaLabel={`Copy tracking URL for ${campaign.name}`}
        icon="copy"
        label={copyStatus || "Copy tracking URL"}
        onClick={() => {
          void navigator.clipboard.writeText(url).then(
            () => setCopyStatus("Copied"),
            () => setCopyStatus("Copy failed")
          );
        }}
      />
      <span aria-live="polite" className="sr-only">{copyStatus}</span>
    </div>
  );
}

function CampaignLinkBuilder({ campaign, baseURL }: { campaign: Campaign; baseURL: string }) {
  const [zoneID, setZoneID] = useState("[ZONE_ID]");
  const [country, setCountry] = useState("[COUNTRY]");
  const [clickID, setClickID] = useState("[CLICK_ID]");
  const [optionalParams, setOptionalParams] = useState<string[]>([]);
  const [copyStatus, setCopyStatus] = useState("");
  const baseLink = baseURL ? campaignTrackingURL(baseURL, campaign.slug) : "";
  const params: Array<[string, string]> = [
    ["source_id", zoneID],
    ["sub1", zoneID],
    ["geo_country", country],
    ["clickid", clickID],
    ["utm_content", clickID]
  ];
  for (const macro of rishadsOptionalMacros) {
    if (optionalParams.includes(macro.param)) {
      params.push([macro.param, macro.macro]);
    }
  }
  const query = params
    .filter(([, value]) => value.trim() !== "")
    .map(([key, value]) => `${key}=${encodeTrackingParameter(value)}`)
    .join("&");
  const url = baseLink ? `${baseLink}${query ? `?${query}` : ""}` : "";
  return (
    <div className="space-y-5">
      <p className="text-sm text-zinc-600">
        Copy this URL into your traffic source. The macro placeholders stay readable so the source can substitute their values.
      </p>
      {campaign.status !== "active" ? (
        <p className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
          This campaign is {campaign.status}; its tracking link will start redirecting only after activation and a successful routing refresh.
        </p>
      ) : null}
      <div className="grid gap-3 sm:grid-cols-3">
        <label className="block text-sm font-medium">
          Source / zone ID
          <input className="input mt-1" onChange={(event) => setZoneID(event.target.value)} value={zoneID} />
          <span className="mt-1 block text-xs font-normal text-zinc-500">source_id and sub1</span>
        </label>
        <label className="block text-sm font-medium">
          Country
          <input className="input mt-1" onChange={(event) => setCountry(event.target.value)} value={country} />
          <span className="mt-1 block text-xs font-normal text-zinc-500">geo_country</span>
        </label>
        <label className="block text-sm font-medium">
          Ad network click ID
          <input className="input mt-1" onChange={(event) => setClickID(event.target.value)} value={clickID} />
          <span className="mt-1 block text-xs font-normal text-zinc-500">clickid and utm_content</span>
        </label>
      </div>
      <div className="space-y-1 text-xs text-zinc-600">
        <p>TraffoFlex generates its own click_id for the destination URL; the source [CLICK_ID] is stored as the external click ID.</p>
        <p>[COUNTRY] is a country name. Rules that expect a two-letter code such as US will not match it.</p>
        {campaign.traffic_source_id ? (
          <p>This campaign has a configured traffic source ID, which takes precedence over source_id in click analytics. The zone ID remains available in sub1.</p>
        ) : null}
      </div>
      <details className="rounded-md border border-zinc-200 p-3">
        <summary className="cursor-pointer text-sm font-medium">Optional source macros</summary>
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          {rishadsOptionalMacros.map((macro) => (
            <label className="flex items-center gap-2 text-sm" key={macro.param}>
              <input
                checked={optionalParams.includes(macro.param)}
                onChange={(event) => setOptionalParams((current) =>
                  event.target.checked
                    ? [...current, macro.param]
                    : current.filter((item) => item !== macro.param)
                )}
                type="checkbox"
              />
              {macro.label} <code className="text-xs text-zinc-500">{macro.macro}</code>
            </label>
          ))}
        </div>
        <p className="mt-3 text-xs text-zinc-500">
          CPV price is kept as a raw parameter; it is per 1000 impressions and is not used as click cost. The source IP parameter is also stored as raw data and is not trusted as the visitor IP.
        </p>
      </details>
      <label className="block text-sm font-medium">
        Tracking URL
        <textarea className="input mt-1 min-h-28 font-mono text-xs" onFocus={(event) => event.currentTarget.select()} readOnly value={url} />
      </label>
      <div className="flex items-center gap-3">
        <button
          className="button-primary"
          disabled={!url}
          onClick={() => {
            void navigator.clipboard.writeText(url).then(
              () => setCopyStatus("Copied to clipboard"),
              () => setCopyStatus("Copy failed; select the URL above")
            );
          }}
          type="button"
        >
          Copy tracking URL
        </button>
        <span aria-live="polite" className="text-sm text-zinc-600">{copyStatus}</span>
      </div>
    </div>
  );
}

function CampaignsPage() {
  const queryClient = useQueryClient();
  const syncRouting = useRoutingSync();
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [editingCampaign, setEditingCampaign] = useState<Campaign | null>(null);
  const [linkCampaign, setLinkCampaign] = useState<Campaign | null>(null);
  const [campaignFormOpen, setCampaignFormOpen] = useState(false);
  const clientConfig = useQuery({
    queryKey: ["client-config"],
    queryFn: api.clientConfig
  });
  const trackerBaseURL = clientConfig.data?.tracker_base_url || localTrackerURL();
  const campaigns = useQuery({
    queryKey: ["campaigns"],
    queryFn: () => api.campaigns()
  });
  const selectedCampaign = campaigns.data?.items.find((campaign) => campaign.id === selectedID) ?? null;
  const columns = useMemo<ColumnDef<Campaign>[]>(
    () => [
      { header: "Name", accessorKey: "name" },
      { header: "Slug", accessorKey: "slug" },
      {
        header: "Tracking URL",
        cell: ({ row }) => (
          <CampaignTrackingCell campaign={row.original} baseURL={trackerBaseURL} />
        )
      },
      { header: "Currency", accessorKey: "currency" },
      { header: "Status", accessorKey: "status" },
      {
        header: "Actions",
        cell: ({ row }) => (
          <div className="flex gap-2">
            <ActionIconButton
              ariaLabel={`Build tracking URL for ${row.original.name}`}
              icon="link"
              label="Build tracking URL"
              onClick={() => setLinkCampaign(row.original)}
            />
            <ActionIconButton
              ariaLabel={`View streams for ${row.original.name}`}
              icon="streams"
              label="View streams"
              onClick={() => setSelectedID(row.original.id)}
            />
            <ActionIconButton
              ariaLabel={`Edit campaign ${row.original.name}`}
              icon="edit"
              label="Edit campaign"
              onClick={() => {
                setEditingCampaign(row.original);
                setCampaignFormOpen(true);
              }}
            />
          </div>
        )
      }
    ],
    [trackerBaseURL]
  );

  const createCampaign = useMutation({
    mutationFn: api.createCampaign,
    onSuccess: () => {
      setCampaignFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ["campaigns"] });
      void syncRouting(true);
    }
  });
  const updateCampaign = useMutation({
    mutationFn: (data: CampaignRequest) =>
      api.updateCampaign(
        editingCampaign?.id ?? "",
        data
      ),
    onSuccess: () => {
      setEditingCampaign(null);
      setCampaignFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ["campaigns"] });
      void syncRouting(true);
    }
  });
  const deleteCampaign = useMutation({
    mutationFn: api.deleteCampaign,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["campaigns"] });
      void syncRouting(true);
    }
  });

  return (
    <Page title="Campaigns">
      {clientConfig.isError ? (
        <p className="mb-4 text-sm text-red-700">
          {trackerBaseURL ? "Could not load the configured tracker address; showing the local tracker. " : "Tracker URL is unavailable: "}
          {clientConfig.error.message}
        </p>
      ) : null}
      <Panel title="Campaign list">
        <div className="mb-4 flex justify-end">
          <button
            className="button-primary"
            onClick={() => {
              setEditingCampaign(null);
              setCampaignFormOpen(true);
            }}
            type="button"
          >
            Create campaign
          </button>
        </div>
        <DataTable columns={columns} data={campaigns.data?.items ?? []} />
      </Panel>
      <Modal
        onClose={() => {
          setCampaignFormOpen(false);
          setEditingCampaign(null);
        }}
        open={campaignFormOpen}
        title={editingCampaign ? "Edit campaign" : "Create campaign"}
      >
        <CampaignForm
          initial={editingCampaign}
          key={editingCampaign?.id ?? "new-campaign"}
          onSubmit={(data) => {
            if (editingCampaign) {
              updateCampaign.mutate(data);
              return;
            }
            createCampaign.mutate(data);
          }}
          submitLabel={editingCampaign ? "Save" : "Create"}
        />
      </Modal>
      <Modal
        onClose={() => setLinkCampaign(null)}
        open={linkCampaign !== null}
        title={linkCampaign ? `Tracking URL: ${linkCampaign.name}` : "Tracking URL"}
      >
        {linkCampaign ? (
          trackerBaseURL ? (
            <CampaignLinkBuilder
              baseURL={trackerBaseURL}
              campaign={linkCampaign}
              key={linkCampaign.id}
            />
          ) : (
            <div className="space-y-3 text-sm">
              <p>Tracker address is unavailable. Reload its configuration to build the tracking URL.</p>
              <button className="button-primary" onClick={() => void clientConfig.refetch()} type="button">
                Reload tracker address
              </button>
            </div>
          )
        ) : null}
      </Modal>
      <Panel title="Streams" className="mt-4">
        {selectedCampaign ? (
          <StreamsManager campaign={selectedCampaign} />
        ) : (
          <p className="text-sm text-zinc-500">Select a campaign to manage streams.</p>
        )}
      </Panel>
      <DangerList
        items={campaigns.data?.items ?? []}
        label={(campaign) => campaign.name}
        onDelete={(campaign) => deleteCampaign.mutate(campaign.id)}
      />
    </Page>
  );
}

function CampaignForm({
  initial,
  onSubmit,
  submitLabel
}: {
  initial?: Campaign | null;
  onSubmit: (data: CampaignRequest) => void;
  submitLabel: string;
}) {
  const [formError, setFormError] = useState("");
  const form = useForm<CampaignRequest>({
    defaultValues: {
      name: initial?.name ?? "",
      slug: initial?.slug ?? "",
      status: initial?.status ?? "paused",
      traffic_source_id: initial?.traffic_source_id ?? "",
      currency: initial?.currency ?? "USD",
      default_action: initial?.default_action ?? "",
      trafficback_config: {
        enabled: initial?.trafficback_config?.enabled ?? false,
        url: initial?.trafficback_config?.url ?? "",
        max_depth: initial?.trafficback_config?.max_depth || 3
      }
    }
  });
  const trafficbackEnabled = form.watch("trafficback_config.enabled");
  return (
    <form
      className="space-y-3"
      onSubmit={form.handleSubmit((values) => {
        const payload = values.trafficback_config.enabled ? values : {
          ...values,
          trafficback_config: { enabled: false, url: "", max_depth: 0 }
        };
        const parsed = campaignSchema.safeParse(payload);
        if (!parsed.success) {
          setFormError(parsed.error.issues[0]?.message ?? "Invalid campaign");
          return;
        }
        setFormError("");
        onSubmit(payload);
      })}
    >
      {formError ? <p className="text-sm text-red-700">{formError}</p> : null}
      <TextField label="Name" register={form.register("name")} />
      <TextField label="Slug" register={form.register("slug")} />
      <TextField label="Currency" register={form.register("currency")} />
      <SelectField label="Status" options={statusOptions} register={form.register("status")} />
      <label className="flex items-center gap-2 text-sm font-medium">
        <input type="checkbox" {...form.register("trafficback_config.enabled")} />
        Send traffic to trafficback when no destination is available
      </label>
      {trafficbackEnabled ? (
        <div className="space-y-3">
          <TextField label="Trafficback URL" register={form.register("trafficback_config.url")} />
          <TextField
            label="Maximum trafficback depth"
            register={form.register("trafficback_config.max_depth", { valueAsNumber: true })}
            type="number"
          />
          <p className="text-xs text-zinc-500">
            Add trafficback_depth={"{trafficback_depth}"} to the URL if it can return to this campaign.
          </p>
        </div>
      ) : null}
      <button className="button-primary" type="submit">
        {submitLabel}
      </button>
    </form>
  );
}

function StreamsManager({ campaign }: { campaign: Campaign }) {
  const queryClient = useQueryClient();
  const syncRouting = useRoutingSync();
  const [editingStream, setEditingStream] = useState<Stream | null>(null);
  const [streamFormOpen, setStreamFormOpen] = useState(false);
  const streams = useQuery({
    queryKey: [
      "streams",
      campaign.id
    ],
    queryFn: () => api.streams(campaign.id)
  });
  const destinations = useQuery({
    queryKey: ["destinations"],
    queryFn: () => api.destinations()
  });
  const createStream = useMutation({
    mutationFn: (data: StreamRequest) =>
      api.createStream(
        campaign.id,
        data
      ),
    onSuccess: () => {
      setStreamFormOpen(false);
      queryClient.invalidateQueries({
        queryKey: [
          "streams",
          campaign.id
        ]
      });
      void syncRouting(true);
    }
  });
  const updateStream = useMutation({
    mutationFn: (data: StreamRequest) =>
      api.updateStream(
        editingStream?.id ?? "",
        data
      ),
    onSuccess: () => {
      setEditingStream(null);
      setStreamFormOpen(false);
      queryClient.invalidateQueries({
        queryKey: [
          "streams",
          campaign.id
        ]
      });
      void syncRouting(true);
    }
  });
  const deleteStream = useMutation({
    mutationFn: api.deleteStream,
    onSuccess: () => {
      setEditingStream(null);
      setStreamFormOpen(false);
      queryClient.invalidateQueries({
        queryKey: [
          "streams",
          campaign.id
        ]
      });
      void syncRouting(true);
    }
  });
  const columns = useMemo<ColumnDef<Stream>[]>(
    () => [
      { header: "Name", accessorKey: "name" },
      { header: "Priority", accessorKey: "priority" },
      { header: "Status", accessorKey: "status" },
      {
        header: "Rules",
        cell: ({ row }) => row.original.conditions.length
      },
      {
        header: "Distribution",
        cell: ({ row }) => row.original.distribution.mode
      },
      {
        header: "Actions",
        cell: ({ row }) => (
          <div className="flex gap-2">
            <ActionIconButton
              ariaLabel={`Edit stream ${row.original.name}`}
              icon="edit"
              label="Edit stream"
              onClick={() => {
                setEditingStream(row.original);
                setStreamFormOpen(true);
              }}
            />
            <ActionIconButton
              ariaLabel={`Delete stream ${row.original.name}`}
              danger
              icon="delete"
              label="Delete stream"
              onClick={() => deleteStream.mutate(row.original.id)}
            />
          </div>
        )
      }
    ],
    [deleteStream]
  );

  return (
    <>
      <div className="mb-4 flex justify-end">
        <button
          className="button-primary"
          onClick={() => {
            setEditingStream(null);
            setStreamFormOpen(true);
          }}
          type="button"
        >
          Add stream
        </button>
      </div>
      <DataTable columns={columns} data={streams.data?.items ?? []} />
      <Modal
        onClose={() => {
          setStreamFormOpen(false);
          setEditingStream(null);
        }}
        open={streamFormOpen}
        title={editingStream ? "Edit stream" : "Create stream"}
      >
        <StreamForm
          destinations={destinations.data?.items ?? []}
          initial={editingStream}
          key={editingStream?.id ?? "new-stream"}
          onCancel={() => {
            setStreamFormOpen(false);
            setEditingStream(null);
          }}
          onSubmit={(data) => {
            if (editingStream) {
              updateStream.mutate(data);
              return;
            }
            createStream.mutate(data);
          }}
          submitLabel={editingStream ? "Save stream" : "Add stream"}
        />
      </Modal>
    </>
  );
}

function StreamForm({
  destinations,
  initial,
  onCancel,
  onSubmit,
  submitLabel
}: {
  destinations: Destination[];
  initial?: Stream | null;
  onCancel: () => void;
  onSubmit: (data: StreamRequest) => void;
  submitLabel: string;
}) {
  const [formError, setFormError] = useState("");
  const [conditions, setConditions] = useState(
    initial?.conditions ?? []
  );
  const [targets, setTargets] = useState(
    initial?.distribution.destinations ?? defaultTargets(destinations)
  );
  const [uniqueEnabled, setUniqueEnabled] = useState(
    initial?.distribution.unique_policy?.enabled ?? false
  );
  const form = useForm<StreamRequest>({
    defaultValues: {
      name: initial?.name ?? "",
      priority: initial?.priority ?? 10,
      status: initial?.status ?? "active",
      conditions: initial?.conditions ?? [],
      distribution: {
        mode: initial?.distribution.mode ?? "weighted",
        destinations: initial?.distribution.destinations ?? defaultTargets(destinations),
        unique_policy: {
          enabled: initial?.distribution.unique_policy?.enabled ?? false,
          user_key: initial?.distribution.unique_policy?.user_key ?? "source_click_id",
          history_window_hours: initial?.distribution.unique_policy?.history_window_hours ?? 168,
          exhausted_mode: initial?.distribution.unique_policy?.exhausted_mode ?? "allow_repeat",
          selection_strategy: initial?.distribution.unique_policy?.selection_strategy ?? "best_roi",
          roi_window_hours: initial?.distribution.unique_policy?.roi_window_hours ?? 24,
          min_clicks: initial?.distribution.unique_policy?.min_clicks ?? 100,
          fallback_strategy: initial?.distribution.unique_policy?.fallback_strategy ?? "round_robin"
        }
      }
    }
  });

  useEffect(
    () => {
      if (!initial && targets.length === 0 && destinations.length > 0) {
        setTargets(defaultTargets(destinations));
      }
    },
    [
      destinations,
      initial,
      targets.length
    ]
  );

  return (
    <form
      className="space-y-4"
      onSubmit={form.handleSubmit((values) => {
        setFormError("");
        const payload: StreamRequest = {
          ...values,
          conditions: normalizedConditions(conditions),
          distribution: {
            ...values.distribution,
            destinations: normalizedTargets(targets),
            unique_policy: normalizedUniquePolicy(
              values.distribution.unique_policy,
              uniqueEnabled
            )
          }
        };
        try {
          streamSchema.parse(payload);
        } catch (err) {
          setFormError(err instanceof Error ? err.message : "Invalid stream");
          return;
        }
        onSubmit(payload);
      })}
    >
      {formError ? <p className="text-sm text-red-700">{formError}</p> : null}
      <div className="grid grid-cols-2 gap-3">
        <TextField label="Name" register={form.register("name")} />
        <TextField
          label="Priority"
          register={form.register(
            "priority",
            { valueAsNumber: true }
          )}
          type="number"
        />
        <SelectField label="Status" options={statusOptions} register={form.register("status")} />
        <SelectField label="Distribution" options={distributionOptions} register={form.register("distribution.mode")} />
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Rules</h3>
          <button
            className="button-secondary"
            onClick={() => setConditions([
              ...conditions,
              {
                field: "sub1",
                operator: "eq",
                value: ""
              }
            ])}
            type="button"
          >
            Add rule
          </button>
        </div>
        <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2 text-xs font-semibold uppercase text-zinc-500">
          <span>Field</span>
          <span>Operator</span>
          <span>Value</span>
          <span>Action</span>
        </div>
        {conditions.map((condition, index) => (
          <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2" key={index}>
            <select
              className="input"
              onChange={(event) => updateCondition(
                conditions,
                setConditions,
                index,
                { field: event.target.value }
              )}
              value={condition.field}
            >
              {conditionFields.map((field) => (
                <option key={field} value={field}>
                  {field}
                </option>
              ))}
            </select>
            <select
              className="input"
              onChange={(event) => updateCondition(
                conditions,
                setConditions,
                index,
                { operator: event.target.value }
              )}
              value={condition.operator}
            >
              {conditionOperators.map((operator) => (
                <option key={operator} value={operator}>
                  {operator}
                </option>
              ))}
            </select>
            <input
              className="input"
              disabled={condition.operator === "exists" || condition.operator === "not_exists"}
              onChange={(event) => updateCondition(
                conditions,
                setConditions,
                index,
                { value: event.target.value }
              )}
              placeholder={condition.operator === "in" || condition.operator === "not_in" ? "a,b,c" : "value"}
              value={condition.values?.join(",") ?? condition.value ?? ""}
            />
            <ActionIconButton
              ariaLabel={`Remove rule ${index + 1}`}
              danger
              icon="remove"
              label="Remove rule"
              onClick={() => setConditions(conditions.filter((_, itemIndex) => itemIndex !== index))}
            />
          </div>
        ))}
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Destinations</h3>
          <button
            className="button-secondary"
            onClick={() => setTargets([
              ...targets,
              {
                destination_id: destinations[0]?.id ?? "",
                weight: 100
              }
            ])}
            type="button"
          >
            Add destination
          </button>
        </div>
        {targets.map((target, index) => (
          <div className="grid grid-cols-[1fr_96px_auto] gap-2" key={index}>
            <select
              className="input"
              onChange={(event) => updateTarget(
                targets,
                setTargets,
                index,
                { destination_id: event.target.value }
              )}
              value={target.destination_id}
            >
              <option value="">Select destination</option>
              {destinations.map((destination) => (
                <option key={destination.id} value={destination.id}>
                  {destination.name}
                </option>
              ))}
            </select>
            <input
              className="input"
              min={1}
              onChange={(event) => updateTarget(
                targets,
                setTargets,
                index,
                { weight: Number(event.target.value) }
              )}
              type="number"
              value={target.weight}
            />
            <ActionIconButton
              ariaLabel={`Remove destination ${index + 1} from stream`}
              danger
              icon="remove"
              label="Remove destination"
              onClick={() => setTargets(targets.filter((_, itemIndex) => itemIndex !== index))}
            />
          </div>
        ))}
      </div>

      <label className="flex items-center gap-2 text-sm font-medium">
        <input
          checked={uniqueEnabled}
          onChange={(event) => setUniqueEnabled(event.target.checked)}
          type="checkbox"
        />
        Unique destination routing
      </label>
      {uniqueEnabled ? (
        <div className="grid grid-cols-2 gap-3">
          <TextField label="User key" register={form.register("distribution.unique_policy.user_key")} />
          <TextField
            label="History window hours"
            register={form.register(
              "distribution.unique_policy.history_window_hours",
              { valueAsNumber: true }
            )}
            type="number"
          />
          <SelectField
            label="Exhausted mode"
            options={[
              "allow_repeat",
              "no_destination"
            ]}
            register={form.register("distribution.unique_policy.exhausted_mode")}
          />
          <SelectField
            label="Selection strategy"
            options={[
              "waterfall",
              "round_robin",
              "best_roi"
            ]}
            register={form.register("distribution.unique_policy.selection_strategy")}
          />
          <TextField
            label="ROI window hours"
            register={form.register(
              "distribution.unique_policy.roi_window_hours",
              { valueAsNumber: true }
            )}
            type="number"
          />
          <TextField
            label="Min clicks"
            register={form.register(
              "distribution.unique_policy.min_clicks",
              { valueAsNumber: true }
            )}
            type="number"
          />
          <SelectField
            label="Fallback strategy"
            options={[
              "waterfall",
              "round_robin",
              "weighted"
            ]}
            register={form.register("distribution.unique_policy.fallback_strategy")}
          />
        </div>
      ) : null}

      <div className="flex gap-2">
        <button className="button-primary" type="submit">
          {submitLabel}
        </button>
        {initial ? (
          <button className="button-secondary" onClick={onCancel} type="button">
            Cancel
          </button>
        ) : null}
      </div>
    </form>
  );
}

const destinationHealthColors: Record<HealthStatus, string> = {
  healthy: "bg-emerald-500",
  degraded: "bg-amber-500",
  unhealthy: "bg-red-500",
  unknown: "bg-zinc-400"
};

function DestinationHealthIndicator({ status }: { status: HealthStatus }) {
  return (
    <span className="inline-flex items-center gap-2">
      <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${destinationHealthColors[status]}`} aria-hidden="true" />
      <span className="capitalize">{status}</span>
    </span>
  );
}

function DestinationsPage() {
  const queryClient = useQueryClient();
  const syncRouting = useRoutingSync();
  const [editingDestination, setEditingDestination] = useState<Destination | null>(null);
  const [destinationFormOpen, setDestinationFormOpen] = useState(false);
  const [healthcheckResult, setHealthcheckResult] = useState<DestinationHealthcheckResult | null>(null);
  const destinations = useQuery({
    queryKey: ["destinations"],
    queryFn: () => api.destinations()
  });
  const createDestination = useMutation({
    mutationFn: api.createDestination,
    onSuccess: () => {
      setDestinationFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ["destinations"] });
      void syncRouting(true);
    }
  });
  const updateDestination = useMutation({
    mutationFn: (data: DestinationRequest) =>
      api.updateDestination(
        editingDestination?.id ?? "",
        data
      ),
    onSuccess: () => {
      setEditingDestination(null);
      setDestinationFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ["destinations"] });
      void syncRouting(true);
    }
  });
  const deleteDestination = useMutation({
    mutationFn: api.deleteDestination,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["destinations"] });
      void syncRouting(true);
    }
  });
  const triggerHealthcheck = useMutation({
    mutationFn: api.triggerDestinationHealthcheck,
    onMutate: () => setHealthcheckResult(null),
    onSuccess: (result) => {
      setHealthcheckResult(result);
      void queryClient.invalidateQueries({ queryKey: ["destinations"] });
    }
  });
  const columns = useMemo<ColumnDef<Destination>[]>(
    () => [
      { header: "Name", accessorKey: "name" },
      { header: "URL", accessorKey: "url" },
      { header: "Manual", accessorKey: "manual_status" },
      {
        header: "Health",
        cell: ({ row }) => <DestinationHealthIndicator status={row.original.health_status} />
      },
      {
        header: "Actions",
        cell: ({ row }) => (
          <div className="flex gap-2">
            <ActionIconButton
              ariaLabel={`Check health of ${row.original.name}`}
              disabled={triggerHealthcheck.isPending}
              icon="check"
              label="Check health"
              onClick={() => triggerHealthcheck.mutate(row.original.id)}
            />
            <ActionIconButton
              ariaLabel={`Edit destination ${row.original.name}`}
              icon="edit"
              label="Edit destination"
              onClick={() => {
                setEditingDestination(row.original);
                setDestinationFormOpen(true);
              }}
            />
            <ActionIconButton
              ariaLabel={`Delete destination ${row.original.name}`}
              danger
              icon="delete"
              label="Delete destination"
              onClick={() => deleteDestination.mutate(row.original.id)}
            />
          </div>
        )
      }
    ],
    [
      deleteDestination,
      triggerHealthcheck
    ]
  );

  return (
    <Page title="Destinations">
      {healthcheckResult ? (
        <p className="mb-4 text-sm" aria-live="polite">
          Probe {healthcheckResult.destination_id}: {healthcheckResult.status}
          {healthcheckResult.probe_status_code ? ` (HTTP ${healthcheckResult.probe_status_code})` : ""}
          {healthcheckResult.error ? ` — ${healthcheckResult.error}` : ""}.
          Failures: {healthcheckResult.consecutive_failures}; successes: {healthcheckResult.consecutive_successes}.
        </p>
      ) : null}
      {triggerHealthcheck.isError ? (
        <p className="mb-4 text-sm text-red-700" aria-live="polite">Probe failed: {triggerHealthcheck.error.message}</p>
      ) : null}
      <Panel title="Destination list">
        <div className="mb-4 flex justify-end">
          <button
            className="button-primary"
            onClick={() => {
              setEditingDestination(null);
              setDestinationFormOpen(true);
            }}
            type="button"
          >
            Create destination
          </button>
        </div>
        <DataTable columns={columns} data={destinations.data?.items ?? []} />
      </Panel>
      <Modal
        onClose={() => {
          setDestinationFormOpen(false);
          setEditingDestination(null);
        }}
        open={destinationFormOpen}
        title={editingDestination ? "Edit destination" : "Create destination"}
      >
        <DestinationForm
          initial={editingDestination}
          key={editingDestination?.id ?? "new-destination"}
          onSubmit={(data) => {
            if (editingDestination) {
              updateDestination.mutate(data);
              return;
            }
            createDestination.mutate(data);
          }}
          submitLabel={editingDestination ? "Save" : "Create"}
        />
      </Modal>
    </Page>
  );
}

function DestinationForm({
  initial,
  onSubmit,
  submitLabel
}: {
  initial?: Destination | null;
  onSubmit: (data: DestinationRequest) => void;
  submitLabel: string;
}) {
  const [formError, setFormError] = useState("");
  const [schedule, setSchedule] = useState<DestinationSchedule>(
    initial?.schedule ?? { enabled: false }
  );
  const [caps, setCaps] = useState<DestinationCaps>(
    initial?.caps ?? { enabled: false }
  );
  const form = useForm<DestinationRequest>({
    defaultValues: {
      name: initial?.name ?? "",
      type: initial?.type ?? "url",
      url: initial?.url ?? "",
      healthcheck_url: initial?.healthcheck_url ?? "",
      manual_status: initial?.manual_status ?? "active",
      health_status: initial?.health_status ?? "unknown",
      redirect: { mode: initial?.redirect.mode ?? "http_302" },
      schedule: initial?.schedule ?? { enabled: false },
      caps: initial?.caps ?? { enabled: false }
    }
  });
  useEffect(
    () => {
      form.reset({
        name: initial?.name ?? "",
        type: initial?.type ?? "url",
        url: initial?.url ?? "",
        healthcheck_url: initial?.healthcheck_url ?? "",
        manual_status: initial?.manual_status ?? "active",
        health_status: initial?.health_status ?? "unknown",
        redirect: { mode: initial?.redirect.mode ?? "http_302" },
        schedule: initial?.schedule ?? { enabled: false },
        caps: initial?.caps ?? { enabled: false }
      });
      setSchedule(initial?.schedule ?? { enabled: false });
      setCaps(initial?.caps ?? { enabled: false });
      setFormError("");
    },
    [
      form,
      initial
    ]
  );
  return (
    <form
      className="space-y-3"
      onSubmit={form.handleSubmit((values) => {
        setFormError("");
        const payload = {
          ...values,
          schedule: normalizeSchedule(schedule),
          caps: normalizeCaps(caps)
        };
        try {
          destinationSchema.parse(payload);
        } catch (err) {
          setFormError(err instanceof Error ? err.message : "Invalid destination");
          return;
        }
        onSubmit(payload);
        form.reset();
      })}
    >
      {formError ? <p className="text-sm text-red-700">{formError}</p> : null}
      <TextField label="Name" register={form.register("name")} />
      <TextField label="URL" register={form.register("url")} />
      <TextField label="Healthcheck URL (optional, required for macro URLs or GET checks)" register={form.register("healthcheck_url")} />
      <SelectField label="Manual status" options={statusOptions} register={form.register("manual_status")} />
      <SelectField label="Redirect" options={["http_302", "meta_refresh", "javascript", "interstitial"]} register={form.register("redirect.mode")} />
      <DestinationScheduleEditor schedule={schedule} setSchedule={setSchedule} />
      <DestinationCapsEditor caps={caps} setCaps={setCaps} />
      <button className="button-primary" type="submit">
        {submitLabel}
      </button>
    </form>
  );
}

function DestinationScheduleEditor({
  schedule,
  setSchedule
}: {
  schedule: DestinationSchedule;
  setSchedule: (schedule: DestinationSchedule) => void;
}) {
  const windows = schedule.windows ?? [];
  return (
    <div className="space-y-2 rounded-md border border-zinc-200 p-3">
      <div className="flex items-center justify-between">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            checked={schedule.enabled}
            onChange={(event) => setSchedule({
              ...schedule,
              enabled: event.target.checked
            })}
            type="checkbox"
          />
          Schedule
        </label>
        <button
          className="button-secondary"
          onClick={() => setSchedule({
            ...schedule,
            enabled: true,
            timezone: schedule.timezone || "Europe/Moscow",
            windows: [
              ...windows,
              {
                weekdays: [
                  "mon",
                  "tue",
                  "wed",
                  "thu",
                  "fri"
                ],
                start_time: "09:00",
                end_time: "18:00"
              }
            ]
          })}
          type="button"
        >
          Add window
        </button>
      </div>
      <FilterInput
        label="Timezone"
        onChange={(value) => setSchedule({
          ...schedule,
          timezone: value
        })}
        value={schedule.timezone ?? ""}
      />
      {windows.map((window, index) => (
        <div className="space-y-2 rounded-md bg-zinc-50 p-2" key={index}>
          <div className="grid grid-cols-2 gap-2">
            <FilterInput
              label="Start"
              onChange={(value) => updateScheduleWindow(
                schedule,
                setSchedule,
                index,
                { start_time: value }
              )}
              type="time"
              value={window.start_time}
            />
            <FilterInput
              label="End"
              onChange={(value) => updateScheduleWindow(
                schedule,
                setSchedule,
                index,
                { end_time: value }
              )}
              type="time"
              value={window.end_time}
            />
          </div>
          <div className="grid grid-cols-7 gap-1">
            {weekdays.map((weekday) => (
              <label className="flex items-center gap-1 text-xs" key={weekday}>
                <input
                  checked={window.weekdays.includes(weekday)}
                  onChange={(event) => {
                    const nextWeekdays = event.target.checked
                      ? [
                        ...window.weekdays,
                        weekday
                      ]
                      : window.weekdays.filter((item) => item !== weekday);
                    updateScheduleWindow(
                      schedule,
                      setSchedule,
                      index,
                      { weekdays: nextWeekdays }
                    );
                  }}
                  type="checkbox"
                />
                {weekday}
              </label>
            ))}
          </div>
          <ActionIconButton
            ariaLabel={`Remove schedule window ${index + 1}`}
            danger
            icon="remove"
            label="Remove window"
            onClick={() => setSchedule({
              ...schedule,
              windows: windows.filter((_, itemIndex) => itemIndex !== index)
            })}
          />
        </div>
      ))}
    </div>
  );
}

function DestinationCapsEditor({
  caps,
  setCaps
}: {
  caps: DestinationCaps;
  setCaps: (caps: DestinationCaps) => void;
}) {
  const rules = caps.rules ?? [];
  return (
    <div className="space-y-2 rounded-md border border-zinc-200 p-3">
      <div className="flex items-center justify-between">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            checked={caps.enabled}
            onChange={(event) => setCaps({
              ...caps,
              enabled: event.target.checked
            })}
            type="checkbox"
          />
          Caps
        </label>
        <button
          className="button-secondary"
          onClick={() => setCaps({
            ...caps,
            enabled: true,
            rules: [
              ...rules,
              {
                metric: "clicks",
                window_hours: 24,
                limit: 1000
              }
            ]
          })}
          type="button"
        >
          Add cap
        </button>
      </div>
      {rules.map((rule, index) => (
        <div className="grid grid-cols-[1fr_96px_96px_auto] gap-2" key={index}>
          <select
            className="input"
            onChange={(event) => updateCapRule(
              caps,
              setCaps,
              index,
              { metric: event.target.value as "clicks" | "cost" | "conversions" | "revenue" }
            )}
            value={rule.metric}
          >
            {[
              "clicks",
              "cost",
              "conversions",
              "revenue"
            ].map((metric) => (
              <option key={metric} value={metric}>
                {metric}
              </option>
            ))}
          </select>
          <input
            className="input"
            min={1}
            onChange={(event) => updateCapRule(
              caps,
              setCaps,
              index,
              { window_hours: Number(event.target.value) }
            )}
            type="number"
            value={rule.window_hours}
          />
          <input
            className="input"
            min={1}
            onChange={(event) => updateCapRule(
              caps,
              setCaps,
              index,
              { limit: Number(event.target.value) }
            )}
            type="number"
            value={rule.limit}
          />
          <ActionIconButton
            ariaLabel={`Remove cap rule ${index + 1}`}
            danger
            icon="remove"
            label="Remove cap rule"
            onClick={() => setCaps({
              ...caps,
              rules: rules.filter((_, itemIndex) => itemIndex !== index)
            })}
          />
        </div>
      ))}
    </div>
  );
}

function PostbacksPage() {
  const queryClient = useQueryClient();
  const [editingTemplate, setEditingTemplate] = useState<PostbackTemplate | null>(null);
  const [templateFormOpen, setTemplateFormOpen] = useState(false);
  const [logFilters, setLogFilters] = useState({
    network_id: "",
    click_id: "",
    transaction_id: "",
    status: "",
    limit: 100
  });
  const templates = useQuery({
    queryKey: ["postback-templates"],
    queryFn: () => api.postbackTemplates()
  });
  const logs = useQuery({
    queryKey: [
      "postback-logs",
      logFilters
    ],
    queryFn: () => api.postbackLogs(logFilters)
  });
  const createTemplate = useMutation({
    mutationFn: api.createPostbackTemplate,
    onSuccess: () => {
      setTemplateFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ["postback-templates"] });
    }
  });
  const updateTemplate = useMutation({
    mutationFn: (data: {
      network_id: string;
      name: string;
      slug: string;
      secret: string;
    }) =>
      api.updatePostbackTemplate(
        editingTemplate?.id ?? "",
        {
          ...data,
          mapping: editingTemplate?.mapping ?? defaultPostbackMapping
        }
      ),
    onSuccess: () => {
      setEditingTemplate(null);
      setTemplateFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ["postback-templates"] });
    }
  });
  const deleteTemplate = useMutation({
    mutationFn: api.deletePostbackTemplate,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["postback-templates"] })
  });
  const columns = useMemo<ColumnDef<PostbackTemplate>[]>(
    () => [
      { header: "Name", accessorKey: "name" },
      { header: "Network", accessorKey: "network_id" },
      { header: "Slug", accessorKey: "slug" },
      {
        header: "Actions",
        cell: ({ row }) => (
          <div className="flex gap-2">
            <ActionIconButton
              ariaLabel={`Edit postback template ${row.original.name}`}
              icon="edit"
              label="Edit template"
              onClick={() => {
                setEditingTemplate(row.original);
                setTemplateFormOpen(true);
              }}
            />
            <ActionIconButton
              ariaLabel={`Delete postback template ${row.original.name}`}
              danger
              icon="delete"
              label="Delete template"
              onClick={() => deleteTemplate.mutate(row.original.id)}
            />
          </div>
        )
      }
    ],
    [deleteTemplate]
  );
  const logColumns = useMemo<ColumnDef<PostbackLogRow>[]>(
    () => [
      {
        header: "Created",
        cell: ({ row }) => new Date(row.original.created_at).toLocaleString()
      },
      { header: "Network", accessorKey: "network_id" },
      { header: "Click", accessorKey: "click_id" },
      { header: "Transaction", accessorKey: "transaction_id" },
      { header: "Status", accessorKey: "status" },
      {
        header: "Error",
        cell: ({ row }) => (
          <span className="break-words text-red-700">{row.original.error ?? ""}</span>
        )
      }
    ],
    []
  );
  const form = useForm<{ network_id: string; name: string; slug: string; secret: string }>({
    defaultValues: {
      network_id: editingTemplate?.network_id ?? "net_demo",
      name: editingTemplate?.name ?? "",
      slug: editingTemplate?.slug ?? "",
      secret: editingTemplate?.secret ?? ""
    }
  });
  useEffect(
    () => {
      form.reset({
        network_id: editingTemplate?.network_id ?? "net_demo",
        name: editingTemplate?.name ?? "",
        slug: editingTemplate?.slug ?? "",
        secret: editingTemplate?.secret ?? ""
      });
    },
    [
      editingTemplate,
      form
    ]
  );

  return (
    <Page title="Postbacks">
      <Panel title="Templates">
        <div className="mb-4 flex justify-end">
          <button
            className="button-primary"
            onClick={() => {
              setEditingTemplate(null);
              setTemplateFormOpen(true);
            }}
            type="button"
          >
            Create template
          </button>
        </div>
        <DataTable columns={columns} data={templates.data?.items ?? []} />
      </Panel>
      <Modal
        onClose={() => {
          setTemplateFormOpen(false);
          setEditingTemplate(null);
        }}
        open={templateFormOpen}
        title={editingTemplate ? "Edit template" : "Create template"}
      >
        <form
          key={editingTemplate?.id ?? "new-template"}
          className="space-y-3"
          onSubmit={form.handleSubmit((values) => {
            if (editingTemplate) {
              updateTemplate.mutate(values);
              return;
            }
            createTemplate.mutate({
              ...values,
              mapping: defaultPostbackMapping
            });
            form.reset();
          })}
        >
          <TextField label="Network ID" register={form.register("network_id")} />
          <TextField label="Name" register={form.register("name")} />
          <TextField label="Slug" register={form.register("slug")} />
          <TextField label="Secret" register={form.register("secret")} />
          <button className="button-primary" type="submit">
            {editingTemplate ? "Save" : "Create"}
          </button>
        </form>
      </Modal>
      <Panel title="Recent postback logs" className="mt-4">
        <div className="mb-4 grid grid-cols-5 gap-3">
          <FilterInput
            label="Network"
            onChange={(value) => setLogFilters((current) => ({ ...current, network_id: value }))}
            value={logFilters.network_id}
          />
          <FilterInput
            label="Click"
            onChange={(value) => setLogFilters((current) => ({ ...current, click_id: value }))}
            value={logFilters.click_id}
          />
          <FilterInput
            label="Transaction"
            onChange={(value) => setLogFilters((current) => ({ ...current, transaction_id: value }))}
            value={logFilters.transaction_id}
          />
          <FilterInput
            label="Status"
            onChange={(value) => setLogFilters((current) => ({ ...current, status: value }))}
            value={logFilters.status}
          />
          <FilterInput
            label="Limit"
            onChange={(value) =>
              setLogFilters((current) => ({
                ...current,
                limit: Number(value) || 100
              }))
            }
            type="number"
            value={String(logFilters.limit)}
          />
        </div>
        <DataTable columns={logColumns} data={logs.data?.items ?? []} />
      </Panel>
    </Page>
  );
}

function HealthHistoryPage() {
  const [filters, setFilters] = useState({
    destination_id: "",
    current: "",
    limit: 100
  });
  const history = useQuery({
    queryKey: [
      "health-history",
      filters
    ],
    queryFn: () => api.healthHistory(filters)
  });
  const columns = useMemo<ColumnDef<HealthHistoryRow>[]>(
    () => [
      {
        header: "Checked",
        cell: ({ row }) => new Date(row.original.checked_at).toLocaleString()
      },
      { header: "Destination", accessorKey: "destination_id" },
      { header: "Previous", accessorKey: "previous" },
      { header: "Current", accessorKey: "current" },
      {
        header: "Error",
        cell: ({ row }) => (
          <span className="break-words text-red-700">{row.original.error ?? ""}</span>
        )
      }
    ],
    []
  );

  return (
    <Page title="Health History">
      <Panel title="Destination health transitions">
        <div className="mb-4 grid grid-cols-3 gap-3">
          <FilterInput
            label="Destination"
            onChange={(value) => setFilters((current) => ({ ...current, destination_id: value }))}
            value={filters.destination_id}
          />
          <FilterInput
            label="Current"
            onChange={(value) => setFilters((current) => ({ ...current, current: value }))}
            value={filters.current}
          />
          <FilterInput
            label="Limit"
            onChange={(value) =>
              setFilters((current) => ({
                ...current,
                limit: Number(value) || 100
              }))
            }
            type="number"
            value={String(filters.limit)}
          />
        </div>
        <DataTable columns={columns} data={history.data?.items ?? []} />
      </Panel>
    </Page>
  );
}

const defaultPostbackMapping = {
  click_id: "cid",
  transaction_id: "tx",
  payout: "sum",
  status: "status"
};

function ReportsPage() {
  const defaultPeriod = useMemo(
    defaultReportPeriod,
    []
  );
  const [group, setGroup] = useState("campaigns");
  const [chartMode, setChartMode] = useState<"volume" | "money" | "efficiency">("money");
  const [period, setPeriod] = useState(defaultPeriod);
  const reportFilters = useMemo(
    () => reportDateRange(
      period.from,
      period.to
    ),
    [
      period
    ]
  );
  const campaigns = useQuery({
    queryKey: ["report-campaign-lookup"],
    queryFn: () => api.campaigns({ limit: 500 })
  });
  const streams = useQuery({
    queryKey: [
      "report-stream-lookup",
      campaigns.data?.items.map((campaign) => campaign.id).join(",") ?? ""
    ],
    enabled: Boolean(campaigns.data),
    queryFn: async () => {
      const results = await Promise.all(
        (campaigns.data?.items ?? []).map((campaign) =>
          api.streams(
            campaign.id,
            { limit: 500 }
          )
        )
      );
      return results.flatMap((result) => result.items);
    }
  });
  const destinations = useQuery({
    queryKey: ["report-destination-lookup"],
    queryFn: () => api.destinations({ limit: 500 })
  });
  const sources = useQuery({
    queryKey: ["report-source-lookup"],
    queryFn: () => api.trafficSources({ limit: 500 })
  });
  const report = useQuery({
    queryKey: [
      "report",
      group,
      reportFilters
    ],
    queryFn: () => api.report(
      group,
      reportFilters
    )
  });
  const dailyReport = useQuery({
    queryKey: [
      "report-daily",
      reportFilters
    ],
    queryFn: () => api.dailyReport(reportFilters)
  });
  const nameLookup = useMemo(
    () => {
      const lookup: Record<string, string> = {};
      for (const campaign of campaigns.data?.items ?? []) {
        lookup[campaign.id] = campaign.name;
      }
      for (const stream of streams.data ?? []) {
        lookup[stream.id] = stream.name;
      }
      for (const destination of destinations.data?.items ?? []) {
        lookup[destination.id] = destination.name;
      }
      for (const source of sources.data?.items ?? []) {
        lookup[source.id] = source.name;
      }
      return lookup;
    },
    [
      campaigns.data,
      destinations.data,
      sources.data,
      streams.data
    ]
  );
  const dailyRows = dailyReport.data?.rows ?? [];
  const rows = useMemo(
    () =>
      (report.data?.rows ?? []).map((row) => ({
        ...row,
        name: nameLookup[row.id] ?? humanizeID(row.name || row.id)
      })),
    [
      nameLookup,
      report.data
    ]
  );
  const columns = useMemo<ColumnDef<ReportRow>[]>(
    () => [
      {
        header: "Name",
        cell: ({ row }) => (
          <div>
            <div className="font-medium text-zinc-950">{row.original.name || row.original.id}</div>
            <div className="mt-1 text-xs text-zinc-500">{row.original.id}</div>
          </div>
        )
      },
      {
        header: "Clicks",
        cell: ({ row }) => formatMetricValue(
          "clicks",
          row.original.metrics.clicks
        )
      },
      {
        header: "Conversions",
        cell: ({ row }) => formatMetricValue(
          "conversions",
          row.original.metrics.conversions
        )
      },
      {
        header: "Revenue",
        cell: ({ row }) => formatMetricValue(
          "revenue",
          row.original.metrics.revenue
        )
      },
      {
        header: "Cost",
        cell: ({ row }) => formatMetricValue(
          "cost",
          row.original.metrics.cost
        )
      },
      {
        header: "Profit",
        cell: ({ row }) => (
          <span className={row.original.metrics.profit >= 0 ? "text-emerald-700" : "text-red-700"}>
            {formatMetricValue(
              "profit",
              row.original.metrics.profit
            )}
          </span>
        )
      },
      {
        header: "ROI",
        cell: ({ row }) => (
          <span className={row.original.metrics.roi >= 0 ? "text-emerald-700" : "text-red-700"}>
            {formatMetricValue(
              "roi",
              row.original.metrics.roi
            )}
          </span>
        )
      },
      {
        header: "Events",
        cell: ({ row }) => formatMetricValue(
          "events",
          row.original.metrics.events ?? 0
        )
      }
    ],
    []
  );
  const chartBars = chartMode === "volume" ? (
    <>
      <Bar dataKey="metrics.clicks" fill="#2563eb" name="Clicks" />
      <Bar dataKey="metrics.conversions" fill="#16a34a" name="Conversions" />
      <Bar dataKey="metrics.events" fill="#9333ea" name="Events" />
    </>
  ) : chartMode === "money" ? (
    <>
      <Bar dataKey="metrics.revenue" fill="#0f766e" name="Revenue" />
      <Bar dataKey="metrics.cost" fill="#f97316" name="Cost" />
      <Bar dataKey="metrics.profit" fill="#16a34a" name="Profit" />
    </>
  ) : (
    <>
      <Bar dataKey="metrics.roi" fill="#7c3aed" name="ROI" />
    </>
  );

  return (
    <Page title="Reports">
      <div className="mb-4 flex gap-2">
        {["campaigns", "streams", "destinations", "sources", "trafficback", "health"].map((item) => (
          <button
            className={item === group ? "button-primary" : "button-secondary"}
            key={item}
            onClick={() => setGroup(item)}
            type="button"
          >
            {item}
          </button>
        ))}
      </div>
      <Panel title="Period" className="mb-6">
        <div className="grid grid-cols-[repeat(2,minmax(180px,240px))_auto] items-end gap-3">
          <label className="text-sm font-medium">
            From
            <input
              className="input mt-2"
              onChange={(event) =>
                setPeriod((current) => ({
                  ...current,
                  from: event.target.value
                }))
              }
              type="date"
              value={period.from}
            />
          </label>
          <label className="text-sm font-medium">
            To
            <input
              className="input mt-2"
              onChange={(event) =>
                setPeriod((current) => ({
                  ...current,
                  to: event.target.value
                }))
              }
              type="date"
              value={period.to}
            />
          </label>
          <button
            className="button-secondary"
            onClick={() => setPeriod(defaultReportPeriod())}
            type="button"
          >
            Last 7 days
          </button>
        </div>
      </Panel>
      <MetricGrid metrics={report.data?.summary} />
      <Panel title="Daily chart" className="mt-6">
        <div className="mb-4 flex gap-2">
          {(["money", "volume", "efficiency"] as const).map((item) => (
            <button
              className={item === chartMode ? "button-primary" : "button-secondary"}
              key={item}
              onClick={() => setChartMode(item)}
              type="button"
            >
              {item}
            </button>
          ))}
        </div>
        <div className="h-72">
          <ResponsiveContainer height="100%" width="100%">
            <BarChart data={dailyRows}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="name" />
              <YAxis />
              <Tooltip
                formatter={(value, name) => {
                  const metricName = String(name).replace(
                    "metrics.",
                    ""
                  ).toLowerCase() as MetricKey;
                  return [
                    formatMetricValue(
                      metricName,
                      Number(value)
                    ),
                    metricName
                  ];
                }}
              />
              {chartBars}
            </BarChart>
          </ResponsiveContainer>
        </div>
      </Panel>
      <Panel title="Table" className="mt-6">
        <DataTable columns={columns} data={rows} />
      </Panel>
    </Page>
  );
}

function IngestionPage() {
  const errors = useQuery({
    queryKey: ["ingestion-errors"],
    queryFn: api.ingestionErrors
  });
  const columns = useMemo<ColumnDef<IngestionErrorRow>[]>(
    () => [
      {
        header: "Observed",
        cell: ({ row }) => new Date(row.original.observed_at).toLocaleString()
      },
      { header: "Topic", accessorKey: "topic" },
      {
        header: "Error",
        cell: ({ row }) => (
          <span className="break-words text-red-700">{row.original.error}</span>
        )
      },
      {
        header: "Raw message",
        cell: ({ row }) => (
          <code className="block max-w-xl break-words text-xs text-zinc-600">
            {row.original.raw_message}
          </code>
        )
      }
    ],
    []
  );

  return (
    <Page title="Ingestion">
      <Panel title="Kafka ingestion errors">
        <DataTable columns={columns} data={errors.data?.rows ?? []} />
      </Panel>
    </Page>
  );
}

function MetricGrid({ metrics }: { metrics?: Metrics }) {
  const items: Array<[string, MetricKey, number]> = [
    [
      "Clicks",
      "clicks",
      metrics?.clicks ?? 0
    ],
    [
      "Conversions",
      "conversions",
      metrics?.conversions ?? 0
    ],
    [
      "Revenue",
      "revenue",
      metrics?.revenue ?? 0
    ],
    [
      "Cost",
      "cost",
      metrics?.cost ?? 0
    ],
    [
      "Profit",
      "profit",
      metrics?.profit ?? 0
    ],
    [
      "ROI",
      "roi",
      metrics?.roi ?? 0
    ]
  ];

  return (
    <div className="grid grid-cols-6 gap-3">
      {items.map(([label, key, value]) => (
        <div className="rounded-md border border-zinc-200 bg-white p-4" key={label}>
          <div className="text-xs uppercase text-zinc-500">{label}</div>
          <div className="mt-2 text-2xl font-semibold">
            {formatMetricValue(
              key,
              value
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

type ActionIcon = "workspace" | "approve" | "streams" | "edit" | "check" | "delete" | "remove" | "close" | "copy" | "link";

function ActionIconGraphic({ icon }: { icon: ActionIcon }) {
  const shapes: Record<ActionIcon, React.ReactNode> = {
    workspace: <><path d="M14 3h7v18h-7" /><path d="m10 8 4 4-4 4M14 12H3" /></>,
    approve: <path d="m4 12 5 5L20 6" />,
    streams: <><rect x="4" y="4" width="16" height="4" rx="1" /><rect x="4" y="10" width="16" height="4" rx="1" /><rect x="4" y="16" width="16" height="4" rx="1" /></>,
    edit: <><path d="m4 20 4.5-1 11-11a2.1 2.1 0 0 0-3-3l-11 11L4 20Z" /><path d="m14.5 7.5 3 3" /></>,
    check: <><path d="M3 12h4l3-7 4 14 3-7h4" /></>,
    delete: <><path d="M4 7h16M10 3h4M6 7l1 14h10l1-14M10 11v6M14 11v6" /></>,
    remove: <><circle cx="12" cy="12" r="9" /><path d="M8 12h8" /></>,
    close: <path d="M5 5l14 14M19 5 5 19" />,
    copy: <><rect x="8" y="8" width="12" height="12" rx="2" /><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" /></>,
    link: <><path d="M10 13a5 5 0 0 0 7.1 0l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1" /><path d="M14 11a5 5 0 0 0-7.1 0l-2 2a5 5 0 0 0 7.1 7.1l1.1-1.1" /></>
  };
  return (
    <svg aria-hidden="true" className="h-4 w-4" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" viewBox="0 0 24 24">
      {shapes[icon]}
    </svg>
  );
}

function ActionIconButton({
  icon,
  label,
  ariaLabel = label,
  onClick,
  danger = false,
  disabled = false,
  tooltipBelow = false
}: {
  icon: ActionIcon;
  label: string;
  ariaLabel?: string;
  onClick: () => void;
  danger?: boolean;
  disabled?: boolean;
  tooltipBelow?: boolean;
}) {
  const tooltipId = useId();
  return (
    <span className="group relative inline-flex">
      <button
        aria-describedby={tooltipId}
        aria-label={ariaLabel}
        className={`inline-flex h-9 w-9 items-center justify-center rounded-md border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 ${
          danger
            ? "border-red-200 text-red-700 hover:bg-red-50 focus-visible:ring-red-600"
            : "border-zinc-200 text-zinc-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 focus-visible:ring-blue-600"
        }`}
        disabled={disabled}
        onClick={onClick}
        type="button"
      >
        <ActionIconGraphic icon={icon} />
      </button>
      <span
        className={`pointer-events-none invisible absolute left-1/2 z-30 -translate-x-1/2 whitespace-nowrap rounded bg-zinc-900 px-2 py-1 text-xs font-medium text-white opacity-0 shadow-lg transition-opacity group-hover:visible group-hover:opacity-100 group-focus-within:visible group-focus-within:opacity-100 ${
          tooltipBelow ? "top-full mt-2" : "bottom-full mb-2"
        }`}
        id={tooltipId}
        role="tooltip"
      >
        {label}
      </span>
    </span>
  );
}

function DataTable<T>({ columns, data }: { columns: ColumnDef<T>[]; data: T[] }) {
  const table = useReactTable({
    columns,
    data,
    getCoreRowModel: getCoreRowModel()
  });
  return (
    <table className="w-full border-collapse text-left text-sm">
      <thead>
        {table.getHeaderGroups().map((headerGroup) => (
          <tr className="border-b border-zinc-200" key={headerGroup.id}>
            {headerGroup.headers.map((header) => (
              <th className="px-3 py-2 font-medium text-zinc-600" key={header.id}>
                {flexRender(
                  header.column.columnDef.header,
                  header.getContext()
                )}
              </th>
            ))}
          </tr>
        ))}
      </thead>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr className="border-b border-zinc-100 last:border-b-0" key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <td className="px-3 py-3 align-top" key={cell.id}>
                {flexRender(
                  cell.column.columnDef.cell,
                  cell.getContext()
                )}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function DangerList<T>({
  items,
  label,
  onDelete
}: {
  items: T[];
  label: (item: T) => string;
  onDelete: (item: T) => void;
}) {
  if (items.length === 0) {
    return null;
  }
  return (
    <Panel title="Danger zone" className="mt-4">
      <div className="flex flex-wrap gap-2">
        {items.map((item) => (
          <button className="button-secondary" key={label(item)} onClick={() => onDelete(item)} type="button">
            Delete {label(item)}
          </button>
        ))}
      </div>
    </Panel>
  );
}

function Page({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h1 className="text-2xl font-semibold">{title}</h1>
      <div className="mt-5">{children}</div>
    </section>
  );
}

function Panel({
  title,
  children,
  className = ""
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={`rounded-md border border-zinc-200 bg-white p-4 ${className}`}>
      <h2 className="mb-4 text-sm font-semibold uppercase text-zinc-500">{title}</h2>
      {children}
    </section>
  );
}

function Modal({
  children,
  onClose,
  open,
  title
}: {
  children: React.ReactNode;
  onClose: () => void;
  open: boolean;
  title: string;
}) {
  const [mounted, setMounted] = useState(open);
  const [visible, setVisible] = useState(false);
  useEffect(
    () => {
      if (open) {
        setMounted(true);
        const timer = window.setTimeout(
          () => setVisible(true),
          24
        );
        return () => window.clearTimeout(timer);
      }
      setVisible(false);
      const timer = window.setTimeout(
        () => setMounted(false),
        220
      );
      return () => window.clearTimeout(timer);
    },
    [open]
  );
  if (!mounted) {
    return null;
  }
  return (
    <div
      className={`fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-zinc-950/40 px-4 py-8 transition-opacity duration-300 ease-out ${
        visible ? "opacity-100" : "opacity-0"
      }`}
    >
      <div
        className={`w-full max-w-4xl rounded-md bg-white shadow-xl transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] will-change-transform ${
          visible ? "translate-y-0 scale-100 opacity-100" : "translate-y-6 scale-[0.96] opacity-0"
        }`}
      >
        <div className="flex items-center justify-between border-b border-zinc-200 px-5 py-4">
          <h2 className="text-base font-semibold text-zinc-950">{title}</h2>
          <ActionIconButton
            icon="close"
            label="Close"
            onClick={onClose}
            tooltipBelow
          />
        </div>
        <div className="max-h-[calc(100vh-10rem)] overflow-y-auto px-5 py-4">
          {children}
        </div>
      </div>
    </div>
  );
}

function TextField({
  label,
  register,
  type = "text"
}: {
  label: string;
  register: ReturnType<typeof useForm>["register"] extends (name: infer Name) => infer Registration
    ? Registration
    : never;
  type?: string;
}) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <input className="input mt-1" type={type} {...register} />
    </label>
  );
}

function SelectField({
  label,
  options,
  register
}: {
  label: string;
  options: string[];
  register: ReturnType<typeof useForm>["register"] extends (name: infer Name) => infer Registration
    ? Registration
    : never;
}) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <select className="input mt-1" {...register}>
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

function FilterInput({
  label,
  onChange,
  type = "text",
  value
}: {
  label: string;
  onChange: (value: string) => void;
  type?: string;
  value: string;
}) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <input
        className="input mt-1"
        onChange={(event) => onChange(event.target.value)}
        type={type}
        value={value}
      />
    </label>
  );
}

function defaultTargets(destinations: Destination[]) {
  if (destinations.length === 0) {
    return [];
  }
  return [
    {
      destination_id: destinations[0].id,
      weight: 100
    }
  ];
}

function normalizedTargets(targets: StreamRequest["distribution"]["destinations"]) {
  return targets
    .filter((target) => target.destination_id.trim() !== "")
    .map((target) => ({
      destination_id: target.destination_id,
      weight: Number(target.weight)
    }));
}

function normalizedConditions(conditions: StreamRequest["conditions"]) {
  return conditions
    .filter((condition) => condition.field.trim() !== "")
    .map((condition) => {
      if (condition.operator === "exists" || condition.operator === "not_exists") {
        return {
          field: condition.field,
          operator: condition.operator
        };
      }
      if (condition.operator === "in" || condition.operator === "not_in") {
        return {
          field: condition.field,
          operator: condition.operator,
          values: splitCSV(condition.values?.join(",") ?? condition.value ?? "")
        };
      }
      return {
        field: condition.field,
        operator: condition.operator,
        value: condition.value ?? condition.values?.join(",") ?? ""
      };
    });
}

function normalizedUniquePolicy(
  policy: StreamRequest["distribution"]["unique_policy"],
  enabled: boolean
) {
  if (!enabled) {
    return { enabled: false };
  }
  return {
    enabled: true,
    user_key: policy?.user_key ?? "source_click_id",
    history_window_hours: Number(policy?.history_window_hours ?? 168),
    exhausted_mode: policy?.exhausted_mode ?? "allow_repeat",
    selection_strategy: policy?.selection_strategy ?? "best_roi",
    roi_window_hours: Number(policy?.roi_window_hours ?? 24),
    min_clicks: Number(policy?.min_clicks ?? 100),
    fallback_strategy: policy?.fallback_strategy ?? "round_robin"
  };
}

function splitCSV(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function updateCondition(
  conditions: StreamRequest["conditions"],
  setConditions: (conditions: StreamRequest["conditions"]) => void,
  index: number,
  patch: Partial<StreamRequest["conditions"][number]>
) {
  setConditions(conditions.map((condition, itemIndex) => {
    if (itemIndex !== index) {
      return condition;
    }
    const next = {
      ...condition,
      ...patch
    };
    if (patch.value !== undefined) {
      if (next.operator === "in" || next.operator === "not_in") {
        next.values = splitCSV(patch.value);
        next.value = undefined;
      } else {
        next.value = patch.value;
        next.values = undefined;
      }
    }
    if (patch.operator !== undefined) {
      if (patch.operator === "exists" || patch.operator === "not_exists") {
        next.value = undefined;
        next.values = undefined;
      } else if (patch.operator === "in" || patch.operator === "not_in") {
        next.values = splitCSV(next.value ?? next.values?.join(",") ?? "");
        next.value = undefined;
      } else {
        next.value = next.value ?? next.values?.join(",") ?? "";
        next.values = undefined;
      }
    }
    return next;
  }));
}

function updateTarget(
  targets: StreamRequest["distribution"]["destinations"],
  setTargets: (targets: StreamRequest["distribution"]["destinations"]) => void,
  index: number,
  patch: Partial<StreamRequest["distribution"]["destinations"][number]>
) {
  setTargets(targets.map((target, itemIndex) => {
    if (itemIndex !== index) {
      return target;
    }
    return {
      ...target,
      ...patch
    };
  }));
}

function normalizeSchedule(schedule: DestinationSchedule): DestinationSchedule {
  if (!schedule.enabled) {
    return { enabled: false };
  }
  return {
    enabled: true,
    timezone: schedule.timezone || "Europe/Moscow",
    windows: (schedule.windows ?? []).filter((window) => window.weekdays.length > 0)
  };
}

function updateScheduleWindow(
  schedule: DestinationSchedule,
  setSchedule: (schedule: DestinationSchedule) => void,
  index: number,
  patch: Partial<NonNullable<DestinationSchedule["windows"]>[number]>
) {
  const windows = schedule.windows ?? [];
  setSchedule({
    ...schedule,
    windows: windows.map((window, itemIndex) => {
      if (itemIndex !== index) {
        return window;
      }
      return {
        ...window,
        ...patch
      };
    })
  });
}

function normalizeCaps(caps: DestinationCaps): DestinationCaps {
  if (!caps.enabled) {
    return { enabled: false };
  }
  return {
    enabled: true,
    rules: (caps.rules ?? []).map((rule) => ({
      metric: rule.metric,
      window_hours: Number(rule.window_hours),
      limit: Number(rule.limit)
    }))
  };
}

function updateCapRule(
  caps: DestinationCaps,
  setCaps: (caps: DestinationCaps) => void,
  index: number,
  patch: Partial<NonNullable<DestinationCaps["rules"]>[number]>
) {
  const rules = caps.rules ?? [];
  setCaps({
    ...caps,
    rules: rules.map((rule, itemIndex) => {
      if (itemIndex !== index) {
        return rule;
      }
      return {
        ...rule,
        ...patch
      };
    })
  });
}

const campaignSchema = z.object({
  name: z.string().min(1),
  slug: z.string().min(3),
  status: z.enum([
    "active",
    "paused",
    "archived"
  ]),
  currency: z.string().regex(/^[A-Z]{3}$/),
  trafficback_config: z.object({
    enabled: z.boolean(),
    url: z.string(),
    max_depth: z.number()
  }).refine(
    (config) => !config.enabled || /^https?:\/\/[^\s/?#@]+(?:[/?#]|$)/i.test(config.url.trim()),
    "Trafficback URL must be an absolute HTTP or HTTPS URL"
  ).refine(
    (config) => !config.enabled || (Number.isInteger(config.max_depth) && config.max_depth >= 1 && config.max_depth <= 10),
    "Maximum trafficback depth must be between 1 and 10"
  )
}).refine(
  (campaign) => campaign.status !== "active" || campaign.trafficback_config.enabled,
  "Active campaign requires a trafficback URL"
);

const streamSchema = z.object({
  name: z.string().min(1),
  priority: z.number().min(0),
  status: z.enum([
    "active",
    "paused",
    "archived"
  ]),
  conditions: z.array(z.object({
    field: z.string().min(1),
    operator: z.enum([
      "eq",
      "neq",
      "in",
      "not_in",
      "contains",
      "not_contains",
      "starts_with",
      "ends_with",
      "gt",
      "gte",
      "lt",
      "lte",
      "exists",
      "not_exists",
      "regex"
    ]),
    value: z.string().optional(),
    values: z.array(z.string()).optional()
  })),
  distribution: z.object({
    mode: z.enum([
      "direct",
      "weighted",
      "fallback",
      "waterfall",
      "round_robin",
      "best_roi"
    ]),
    destinations: z.array(z.object({
      destination_id: z.string().min(1),
      weight: z.number().positive()
    })).min(1),
    unique_policy: z.object({
      enabled: z.boolean(),
      user_key: z.string().optional(),
      history_window_hours: z.number().positive().optional(),
      exhausted_mode: z.enum([
        "allow_repeat",
        "no_destination"
      ]).optional(),
      selection_strategy: z.enum([
        "waterfall",
        "round_robin",
        "best_roi"
      ]).optional(),
      roi_window_hours: z.number().positive().optional(),
      min_clicks: z.number().min(0).optional(),
      fallback_strategy: z.enum([
        "waterfall",
        "round_robin",
        "weighted"
      ]).optional()
    }).optional()
  })
});

const destinationSchema = z.object({
  name: z.string().min(1),
  type: z.string().min(1),
  url: z.string().url(),
  healthcheck_url: z.string().optional().refine(
    (value) => !value || (/^https?:\/\//i.test(value) && !/[{}]/.test(value)),
    "Healthcheck URL must use HTTP(S) and contain no macros"
  ),
  manual_status: z.enum([
    "active",
    "paused",
    "archived"
  ]),
  health_status: z.enum([
    "healthy",
    "degraded",
    "unhealthy",
    "unknown"
  ]),
  redirect: z.object({
    mode: z.enum([
      "http_302",
      "meta_refresh",
      "javascript",
      "interstitial"
    ])
  }),
  schedule: z.object({
    enabled: z.boolean(),
    timezone: z.string().optional(),
    windows: z.array(z.object({
      weekdays: z.array(z.enum([
        "mon",
        "tue",
        "wed",
        "thu",
        "fri",
        "sat",
        "sun"
      ])),
      start_time: z.string(),
      end_time: z.string()
    })).optional()
  }),
  caps: z.object({
    enabled: z.boolean(),
    rules: z.array(z.object({
      metric: z.enum([
        "clicks",
        "cost",
        "conversions",
        "revenue"
      ]),
      window_hours: z.number().positive(),
      limit: z.number().positive()
    })).optional()
  })
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
