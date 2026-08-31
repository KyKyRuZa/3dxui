import api from "./axios";

export interface PublicConfig {
  yookassa_test_mode: boolean;
  bot_username: string;
}

export async function getPublicConfig(): Promise<PublicConfig> {
  const { data } = await api.get<PublicConfig>("/config");
  return data;
}
