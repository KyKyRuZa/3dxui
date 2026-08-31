import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@hooks/useAuth";
import { getPublicConfig } from "@api/config";
import { telegramWidget, type TelegramWidgetUser } from "@api/auth";
import { setAccessToken } from "@api/axios";
import styles from "@styles/Auth.module.css";

interface AuthResponse {
  access_token: string;
  user: { id: number; username: string; email: string; is_active: boolean; created_at: string };
}

export default function AuthForm() {
  const navigate = useNavigate();
  const { isAuthenticated } = useAuth();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [botUsername, setBotUsername] = useState<string | null>(null);
  const widgetRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isAuthenticated) void navigate("/dashboard");
  }, [isAuthenticated, navigate]);

  useEffect(() => {
    getPublicConfig()
      .then((cfg) => setBotUsername(cfg.bot_username || "AutoColorsBot"))
      .catch(() => setBotUsername("AutoColorsBot"));
  }, []);

  useEffect(() => {
    const login = botUsername || "AutoColorsBot";

    const finish = (data: AuthResponse) => {
      setAccessToken(data.access_token);
      window.location.href = "/dashboard";
    };

    (window as unknown as { onTelegramAuth?: (user: TelegramWidgetUser) => void }).onTelegramAuth =
      async (user: TelegramWidgetUser) => {
        setLoading(true);
        setError("");
        try {
          const data = await telegramWidget(user);
          finish(data);
        } catch {
          setError("Не удалось войти через Telegram. Попробуйте ещё раз.");
        } finally {
          setLoading(false);
        }
      };

    const script = document.createElement("script");
    script.src = "https://telegram.org/js/telegram-widget.js?22";
    script.async = true;
    script.setAttribute("data-telegram-login", login);
    script.setAttribute("data-size", "large");
    script.setAttribute("data-userpic", "false");
    script.setAttribute("data-onauth", "onTelegramAuth(user)");
    script.setAttribute("data-request-access", "write");
    script.onerror = () =>
      setError("Не удалось загрузить виджет Telegram. Проверьте подключение.");
    widgetRef.current?.appendChild(script);

    return () => {
      delete (window as unknown as { onTelegramAuth?: unknown }).onTelegramAuth;
    };
  }, [botUsername]);

  return (
    <div className={styles.wrap}>
      <div className={styles.card}>
        <div className={styles.title}>Вход через Telegram</div>
        <p className={styles.subtitle}>
          Регистрация и вход выполняются только через Telegram. Нажмите кнопку ниже,
          чтобы привязать аккаунт и получить доступ к ключам VPN.
        </p>
        <div ref={widgetRef} className={styles.widget} />
        {loading && <div className={styles.hint}>Авторизация…</div>}
        {error && <div className={styles.error}>{error}</div>}
        <div className={styles.switch}>
          Нет Telegram под руки? Откройте бота{" "}
          <a href={`https://t.me/${botUsername ?? "AutoColorsBot"}`} target="_blank" rel="noreferrer">
            @{botUsername ?? "AutoColorsBot"}
          </a>{" "}
          и запустите Mini App.
        </div>
      </div>
    </div>
  );
}
