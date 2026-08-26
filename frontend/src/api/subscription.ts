import api from "./axios";

export interface Subscription {
  subscription_url: string;
  username: string;
  group: string;
  vless: string;
  links: string[];
}

export interface VLESSConfig {
  config_url: string;
  username: string;
}

let cachedActivate: Subscription | null = null;
let cachedGet: Subscription | null = null;

export async function activateSubscription(): Promise<Subscription> {
  if (cachedActivate) return cachedActivate;
  const { data } = await api.post<Subscription>("/subscription/activate");
  cachedActivate = data;
  return data;
}

export async function getSubscription(): Promise<Subscription> {
  if (cachedGet) return cachedGet;
  const { data } = await api.get<Subscription>("/subscription");
  cachedGet = data;
  return data;
}

export async function getVLESSConfig(): Promise<VLESSConfig> {
  const { data } = await api.get<VLESSConfig>("/subscription/config");
  return data;
}

export async function getSingBoxConfig(): Promise<string> {
  const { data } = await api.get<string>("/subscription/config/singbox", {
    responseType: "text",
  });
  return data;
}

// Clear cached subscription data so the next read reflects a fresh state
// (e.g. after a successful purchase that extended/renewed the plan).
export function resetSubscriptionCache() {
  cachedActivate = null;
  cachedGet = null;
}
