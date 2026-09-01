import api from "./axios";

export interface User {
  id: number;
  username: string;
  email: string;
  is_active: boolean;
  created_at: string;
}

export interface AuthResponse {
  access_token: string;
  user: User;
}

export async function refresh(): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/refresh");
  return data;
}

export async function logout(): Promise<void> {
  await api.post("/auth/logout");
}

export async function getProfile(): Promise<User> {
  const { data } = await api.get<User>("/auth/profile");
  return data;
}

export async function telegram(initData: string, consentHash?: string): Promise<AuthResponse> {
	const body: Record<string, string> = { init_data: initData };
	if (consentHash) {
		body.consent_hash = consentHash;
	}
	const { data } = await api.post<AuthResponse>("/auth/telegram", body);
	return data;
}

export interface TelegramWidgetUser {
  id: number;
  first_name: string;
  last_name?: string;
  username?: string;
  photo_url?: string;
  auth_date: number;
  hash: string;
}

export async function telegramWidget(user: TelegramWidgetUser): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/telegram/widget", user);
  return data;
}
