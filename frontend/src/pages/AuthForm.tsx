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

  const startLink = async () => {
    if (!consent) {
      setError("Для продолжения необходимо согласие на обработку персональных данных.");
      return;
    }
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
        <div className={styles.title}>Вход через Telegram</div>
        <p className={styles.subtitle}>
          Регистрация и вход выполняются только через Telegram. Нажмите кнопку ниже —
          откроется бот, который вернёт вас на сайт с активной сессией.
        </p>

        {!loginUrl && (
          <>
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
            <Button
              variant="primary"
              block
              loading={status === "linking"}
              onClick={startLink}
            >
              Войти через Telegram
            </Button>
          </>
        )}

        {loginUrl && (
          <a
            className={`${buttonStyles.button} ${buttonStyles.primary} ${buttonStyles.block}`}
            href={loginUrl}
            target="_blank"
            rel="noreferrer"
          >
            Открыть Telegram-бота
          </a>
        )}

        {status === "waiting" && (
          <div className={styles.hint}>
            Ожидание подтверждения в Telegram… Не закрывайте эту страницу.
          </div>
        )}
        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.switch}>
          Нет Telegram под руки? Откройте бота{" "}
          <a href={`https://t.me/${botUsername}`} target="_blank" rel="noreferrer">
            @{botUsername}
          </a>{" "}
          и запустите Mini App.
        </div>
      </div>
    </div>
  );
}
