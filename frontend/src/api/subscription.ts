import api from "./axios";

export interface Subscription {
  subscription_url: string;
  username: string;
}

export interface VLESSConfig {
  config_url: string;
  username: string;
}

export async function getSubscription(): Promise<Subscription> {
  const { data } = await api.get<Subscription>("/subscription");
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
