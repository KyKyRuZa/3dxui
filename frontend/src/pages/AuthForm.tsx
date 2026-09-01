import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@components/Button";
import { useAuth } from "@hooks/useAuth";
import { getPublicConfig } from "@api/config";
import { setAccessToken } from "@api/axios";
import buttonStyles from "@styles/Button.module.css";
import styles from "@styles/Auth.module.css";

interface LinkResponse {
	token: string;
	login_url: string;
	expires_in: number;
}

export default function AuthForm() {
	const navigate = useNavigate();
	const { isAuthenticated } = useAuth();
	const [error, setError] = useState("");
	const [status, setStatus] = useState<"idle" | "waiting" | "linking">("idle");
	const [loginUrl, setLoginUrl] = useState<string | null>(null);
	const [botUsername, setBotUsername] = useState("AutoColorsBot");
	const [consent, setConsent] = useState(false);
	const tokenRef = useRef<string | null>(null);
	const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

	const [mode, setMode] = useState<"login" | "register">("login");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [loading, setLoading] = useState(false);

	// Telegram code login state
	const [showCodeInput, setShowCodeInput] = useState(false);
	const [tgCode, setTgCode] = useState("");
	const [codeLoading, setCodeLoading] = useState(false);

	useEffect(() => {
		if (isAuthenticated) navigate("/dashboard");
	}, [isAuthenticated, navigate]);

	useEffect(() => {
		getPublicConfig()
			.then((cfg) => setBotUsername(cfg.bot_username || "AutoColorsBot"))
			.catch(() => setBotUsername("AutoColorsBot"));
	}, []);

	useEffect(() => {
		return () => {
			if (pollRef.current) clearInterval(pollRef.current);
		};
	}, []);

	const stopPoll = () => {
		if (pollRef.current) {
			clearInterval(pollRef.current);
			pollRef.current = null;
		}
	};

	const handlePasswordAuth = async (isRegister: boolean) => {
		if (!username || !password) {
			setError("Введите имя пользователя и пароль.");
			return;
		}
		if (isRegister && password.length < 6) {
			setError("Пароль должен быть не менее 6 символов.");
			return;
		}
		if (isRegister && username.length < 3) {
			setError("Имя пользователя должно быть не менее 3 символов.");
			return;
		}
		if (isRegister && !consent) {
			setError("Для регистрации необходимо согласие на обработку персональных данных.");
			return;
		}
		setError("");
		setLoading(true);
		try {
			const endpoint = isRegister ? "/api/auth/register" : "/api/auth/login";
			const res = await fetch(endpoint, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ username, password }),
			});
			const data = await res.json();
			if (!res.ok) {
				setError(data.error || "Ошибка авторизации.");
				return;
			}
			setAccessToken(data.access_token);
			window.location.href = "/dashboard";
		} catch {
			setError("Ошибка сети. Попробуйте позже.");
		} finally {
			setLoading(false);
		}
	};

	const handleCodeLogin = async () => {
		if (!tgCode || tgCode.length < 6) {
			setError("Введите код из Telegram (8 цифр).");
			return;
		}
		setError("");
		setCodeLoading(true);
		try {
			const res = await fetch("/api/auth/verify-login-code", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ code: tgCode }),
			});
			const data = await res.json();
			if (!res.ok) {
				setError(data.error || "Неверный или просроченный код.");
				return;
			}
			setAccessToken(data.access_token);
			window.location.href = "/dashboard";
		} catch {
			setError("Ошибка сети. Попробуйте позже.");
		} finally {
			setCodeLoading(false);
		}
	};

	const startLink = async () => {
		setError("");
		setStatus("linking");
		try {
			const res = await fetch("/api/auth/telegram/link", { method: "POST" });
			if (!res.ok) throw new Error("link failed");
			const data: LinkResponse = await res.json();
			tokenRef.current = data.token;
			setLoginUrl(data.login_url);
			setStatus("waiting");
			pollRef.current = setInterval(checkToken, 2500);
			setTimeout(() => {
				if (tokenRef.current === data.token) {
					stopPoll();
					setError("Время ссылки истекло. Попробуйте ещё раз.");
					setStatus("idle");
					tokenRef.current = null;
				}
			}, (data.expires_in || 300) * 1000);
		} catch {
			setError("Не удалось создать ссылку для входа. Попробуйте ещё раз.");
			setStatus("idle");
		}
	};

	const checkToken = async () => {
		const token = tokenRef.current;
		if (!token) return;
		try {
			const res = await fetch(`/api/auth/telegram/link/${token}`);
			if (res.status === 404) {
				stopPoll();
				setError("Ссылка недействительна или истекла.");
				setStatus("idle");
				tokenRef.current = null;
				return;
			}
			const data = await res.json();
			if (data.access_token) {
				stopPoll();
				setAccessToken(data.access_token);
				tokenRef.current = null;
				window.location.href = "/dashboard";
			}
		} catch {
			// transient network error, keep polling
		}
	};

	return (
		<div className={styles.wrap}>
			<div className={styles.card}>
				<div className={styles.title}>
					{mode === "login" ? "Вход в аккаунт" : "Регистрация"}
				</div>

				<div className={styles.form}>
					<div className={styles.field}>
						<label className={styles.label}>Имя пользователя</label>
						<input
							className={styles.input}
							type="text"
							value={username}
							onChange={(e) => setUsername(e.target.value)}
							placeholder="username"
							autoComplete={mode === "register" ? "username" : "username"}
						/>
					</div>
					<div className={styles.field}>
						<label className={styles.label}>Пароль</label>
						<input
							className={styles.input}
							type="password"
							value={password}
							onChange={(e) => setPassword(e.target.value)}
							placeholder="••••••"
							autoComplete={mode === "register" ? "new-password" : "current-password"}
						/>
					</div>

					{mode === "register" && (
						<label className={styles.consent}>
							<input
								type="checkbox"
								checked={consent}
								onChange={(e) => setConsent(e.target.checked)}
							/>
							<span>
								Я согласен с{" "}
								<a href="/privacy" target="_blank" rel="noreferrer">
									Политикой конфиденциальности
								</a>{" "}
								и даю согласие на обработку персональных данных
							</span>
						</label>
					)}

					{mode === "login" ? (
						<Button
							variant="primary"
							block
							loading={loading}
							onClick={() => handlePasswordAuth(false)}
						>
							Войти
						</Button>
					) : (
						<Button
							variant="primary"
							block
							loading={loading}
							onClick={() => handlePasswordAuth(true)}
						>
							Зарегистрироваться
						</Button>
					)}
				</div>

				<div className={styles.divider}>
					<span>или</span>
				</div>

				{!showCodeInput ? (
					<>
						{!loginUrl ? (
							<button
								className={`${buttonStyles.button} ${styles.tgButton}`}
								onClick={startLink}
								disabled={status === "linking"}
							>
								{status === "linking" ? "Загрузка..." : "Войти через Telegram"}
							</button>
						) : (
							<a
								className={`${buttonStyles.button} ${styles.tgButton}`}
								href={loginUrl}
								target="_blank"
								rel="noreferrer"
							>
								Открыть Telegram-бота
							</a>
						)}

						<div className={styles.codeToggle}>
							<button
								className={styles.linkBtn}
								onClick={() => { setShowCodeInput(true); setError(""); }}
							>
								У меня есть код из Telegram
							</button>
						</div>
					</>
				) : (
					<div className={styles.codeLogin}>
						<div className={styles.field}>
							<label className={styles.label}>Код из Telegram</label>
							<input
								className={styles.input}
								type="text"
								value={tgCode}
								onChange={(e) => setTgCode(e.target.value.replace(/\D/g, "").slice(0, 8))}
								placeholder="48271593"
								maxLength={8}
								inputMode="numeric"
							/>
						</div>
						<Button
							variant="primary"
							block
							loading={codeLoading}
							onClick={handleCodeLogin}
						>
							Войти по коду
						</Button>
						<div className={styles.codeToggle}>
							<button
								className={styles.linkBtn}
								onClick={() => { setShowCodeInput(false); setTgCode(""); setError(""); }}
							>
								Назад
							</button>
						</div>
					</div>
				)}

				{status === "waiting" && (
					<div className={styles.hint}>
						Ожидание подтверждения в Telegram… Не закрывайте эту страницу.
					</div>
				)}
				{error && <div className={styles.error}>{error}</div>}

				<div className={styles.switch}>
					{mode === "login" ? (
						<>
							Нет аккаунта?{" "}
							<button
								className={styles.linkBtn}
								onClick={() => { setMode("register"); setError(""); }}
							>
								Зарегистрироваться
							</button>
						</>
					) : (
						<>
							Уже есть аккаунт?{" "}
							<button
								className={styles.linkBtn}
								onClick={() => { setMode("login"); setError(""); }}
							>
								Войти
							</button>
						</>
					)}
				</div>

				{!isAuthenticated && (
					<div className={styles.hint}>
						Нет Telegram под руки? Откройте бота{" "}
						<a href={`https://t.me/${botUsername}`} target="_blank" rel="noreferrer">
							@{botUsername}
						</a>{" "}
						и запустите Mini App.
					</div>
				)}
			</div>
		</div>
	);
}
