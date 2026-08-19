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

export async function login(username: string, password: string): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/login", { username, password });
  return data;
}

export async function register(
  username: string,
  email: string,
  password: string,
): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/register", { username, email, password });
  return data;
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

export async function updateProfile(email: string): Promise<User> {
  const { data } = await api.patch<User>("/auth/profile", { email });
  return data;
}

export async function telegram(initData: string): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/telegram", { init_data: initData });
  return data;
}

export async function changePassword(
  current_password: string,
  new_password: string,
): Promise<void> {
  await api.post("/auth/password", { current_password, new_password });
}
