import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import * as authApi from "../api/auth";
import { registerAuthFail, setAccessToken } from "../api/axios";
import type { User } from "../api/auth";

interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, email: string, password: string) => Promise<void>;
  telegramLogin: (initData: string) => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function applySession(data: authApi.AuthResponse) {
  setAccessToken(data.access_token);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUserState] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const setUser = useCallback((u: User) => setUserState(u), []);

  const login = useCallback(async (username: string, password: string) => {
    const data = await authApi.login(username, password);
    applySession(data);
    setUserState(data.user);
  }, []);

  const register = useCallback(async (username: string, email: string, password: string) => {
    const data = await authApi.register(username, email, password);
    applySession(data);
    setUserState(data.user);
  }, []);

  const telegramLogin = useCallback(async (initData: string) => {
    const data = await authApi.telegram(initData);
    applySession(data);
    setUserState(data.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } finally {
      setAccessToken(null);
      setUserState(null);
    }
  }, []);

  useEffect(() => {
    registerAuthFail(() => setUserState(null));
    void (async () => {
      try {
        const data = await authApi.refresh();
        applySession(data);
        setUserState(data.user);
      } catch {
        const tgData = typeof window !== "undefined" ? (window as any).Telegram?.WebApp?.initData : null;
        if (tgData) {
          try {
            const data = await authApi.telegram(tgData);
            applySession(data);
            setUserState(data.user);
            return;
          } catch {
            // ignore telegram auth failure and fall through to unauthenticated state
          }
        }
        setUserState(null);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: Boolean(user),
        loading,
        login,
        register,
        telegramLogin,
        logout,
        setUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
