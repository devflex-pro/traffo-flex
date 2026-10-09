export const reportGroups = [
  { key: "campaign", field: "campaign_id", label: "Campaign" },
  { key: "stream", field: "stream_id", label: "Stream" },
  { key: "destination", field: "destination_id", label: "Destination" },
  { key: "source", field: "source_id", label: "Traffic source" },
  { key: "geo_country", field: "geo_country", label: "Country" },
  { key: "geo_region", field: "geo_region", label: "Region" },
  { key: "city", field: "city", label: "City" },
  { key: "device_type", field: "device_type", label: "Device" },
  { key: "os", field: "os", label: "OS" },
  { key: "browser", field: "browser", label: "Browser" },
  { key: "isp", field: "isp", label: "ISP" },
  { key: "zone_id", field: "zone_id", label: "Zone" },
  { key: "publisher_id", field: "publisher_id", label: "Publisher" },
  { key: "site_id", field: "site_id", label: "Site" },
  { key: "creative_id", field: "creative_id", label: "Creative" },
  { key: "carrier", field: "carrier", label: "Carrier" },
  { key: "connection_type", field: "connection_type", label: "Connection type" },
  { key: "source_campaign_id", field: "source_campaign_id", label: "Source campaign ID" },
  { key: "source_campaign_name", field: "source_campaign_name", label: "Source campaign name" }
] as const;

export type ReportGroup = (typeof reportGroups)[number]["key"];
export const defaultReportPath: ReportGroup[] = ["campaign", "geo_country", "zone_id"];
export const reportTimezones = ["UTC", "Europe/Moscow", "Europe/Berlin", "America/New_York", "Asia/Kolkata", "Asia/Tokyo"];

export function dateInTimezone(timezone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date());
  const value = (type: Intl.DateTimeFormatPartTypes) => parts.find(part => part.type === type)?.value;
  return `${value("year")}-${value("month")}-${value("day")}`;
}

export function reportPreset(preset: string, timezone: string) {
  const today = dateInTimezone(timezone);
  const shift = (days: number) => {
    const date = new Date(`${today}T12:00:00Z`);
    date.setUTCDate(date.getUTCDate() + days);
    return date.toISOString().slice(0, 10);
  };
  if (preset === "yesterday") return { from: shift(-1), to: shift(-1) };
  return { from: shift(preset === "today" ? 0 : preset === "30days" ? -29 : -6), to: today };
}
