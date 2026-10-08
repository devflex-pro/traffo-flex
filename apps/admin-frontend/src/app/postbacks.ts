import { PostbackTemplate } from "./api";

const macro = (name: string) => `{${name}}`;
const queryValue = (value: string) => encodeURIComponent(value).replace(/%7B/gi, "{").replace(/%7D/gi, "}");

export function incomingPostbackURL(base: string, template: Pick<PostbackTemplate, "network_id" | "secret" | "mapping">): string {
  if (!base || !template.network_id || !template.secret) return "";
  const clickMacro = template.mapping.click_id || "cid";
  const params: Record<string, string> = {
    secret: template.secret,
    click_id: macro(clickMacro),
    transaction_id: macro(template.mapping.transaction_id || clickMacro),
    payout: macro(template.mapping.payout || "sum"),
    currency: template.mapping.currency || "USD",
    status: template.mapping.status ? macro(template.mapping.status) : "approved"
  };
  const query = Object.entries(params).map(([key, value]) => `${key}=${key === "secret" ? encodeURIComponent(value) : queryValue(value)}`).join("&");
  return `${base.replace(/\/$/, "")}/pb/${encodeURIComponent(template.network_id)}?${query}`;
}

export function generatePostbackSecret(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, "0")).join("");
}
