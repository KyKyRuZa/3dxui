import axios, { type AxiosError } from "axios";

let accessToken: string | null = null;
let onAuthFail: (() => void) | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export function registerAuthFail(cb: () => void) {
  onAuthFail = cb;
}

export const api = axios.create({
  baseURL: "/api",
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`;
  }
  return config;
});

// Transparently refresh the access token once on 401 and retry the request.
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as (typeof error.config & { _retry?: boolean }) | undefined;
    if (
      error.response?.status === 401 &&
      original &&
      !original._retry &&
      original.url !== "/auth/refresh"
    ) {
      original._retry = true;
      try {
        const { data } = await api.post<{ access_token: string }>("/auth/refresh");
        setAccessToken(data.access_token);
        original.headers.Authorization = `Bearer ${data.access_token}`;
        return api(original);
      } catch {
        setAccessToken(null);
        onAuthFail?.();
        return Promise.reject(error);
      }
    }
    return Promise.reject(error);
  },
);

export default api;
