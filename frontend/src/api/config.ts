import api from "./axios";

export interface PublicConfig {
  yookassa_test_mode: boolean;
}

export async function getPublicConfig(): Promise<PublicConfig> {
  const { data } = await api.get<PublicConfig>("/config");
  return data;
}
