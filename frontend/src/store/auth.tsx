import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import * as authApi from "@api/auth";
import { registerAuthFail, setAccessToken } from "@api/axios";
import type { User } from "@api/auth";

interface AuthContextValue {
	user: User | null;
	isAuthenticated: boolean;
	loading: boolean;
	telegramLogin: (initData: string, consentHash?: string) => Promise<void>;
	logout: () => Promise<void>;
	setUser: (user: User) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function applySession(data: authApi.AuthResponse) {
	setAccessToken(data.access_token)
}

// Generates a simple hash of the consent text for the given timestamp.
// This proves the user consented to a specific version of the policy.
function generateConsentHash(timestamp: number): string {
	const text = `privacy_policy_v1_${timestamp}`;
	let hash = 0;
	for (let i = 0; i < text.length; i++) {
		const char = text.charCodeAt(i);
		hash = ((hash << 5) - hash) + char;
		hash = hash & hash;
	}
	return Math.abs(hash).toString(16);
}

export function AuthProvider({ children }: { children: ReactNode }) {
	const [user, setUserState] = useState<User | null>(null);
	const [loading, setLoading] = useState(true);

	const setUser = useCallback((u: User) => setUserState(u), []);

	const telegramLogin = useCallback(async (initData: string, consentHash?: string) => {
		const data = await authApi.telegram(initData, consentHash);
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
