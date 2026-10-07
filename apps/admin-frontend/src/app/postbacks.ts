import { PostbackTemplate } from "./api";

const macro = (name: string) => `{${name}}`;
const queryValue = (value: string) => encodeURIComponent(value).replace(/%7B/gi, "{").replace(/%7D/gi, "}");

export function incomingPostbackURL(base: string, template: Pick<PostbackTemplate, "network_id" | "secret" | "mapping" | "provider">): string {
  if (!base || !template.network_id || !template.secret) return "";
  const lospollos = template.provider === "lospollos";
  const params: Record<string, string> = {
    secret: template.secret,
    click_id: macro(template.mapping.click_id || "cid"),
    transaction_id: macro(template.mapping.transaction_id || (lospollos ? "cid" : "tx")),
    payout: macro(template.mapping.payout || "sum"),
    currency: "USD",
    status: lospollos ? "approved" : macro(template.mapping.status || "status")
  };
  if (lospollos) for (let i = 1; i <= 4; i++) params[`sub${i}`] = `{s${i}}`;
  const query = Object.entries(params).map(([key, value]) => `${key}=${key === "secret" ? encodeURIComponent(value) : queryValue(value)}`).join("&");
  return `${base.replace(/\/$/, "")}/pb/${encodeURIComponent(template.network_id)}?${query}`;
}

export function lospollosSmartlink(raw: string): string {
  if (!raw.trim()) return "";
  try {
    const url = new URL(raw.trim());
    if (!["http:", "https:"].includes(url.protocol)) return "";
    url.searchParams.set("cid", "{click_id}");
    for (let i = 1; i <= 4; i++) url.searchParams.set(`s${i}`, `{sub${i}}`);
    return url.toString().replace(/%7B([a-z0-9_]+)%7D/gi, "{$1}");
  } catch { return ""; }
}

export function generatePostbackSecret(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, "0")).join("");
}
