import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@components/Button";
import { useAuth } from "@hooks/useAuth";
import { getPublicConfig } from "@api/config";
import styles from "@styles/Auth.module.css";

export default function AuthForm() {
  const navigate = useNavigate();
  const { telegramLogin, isAuthenticated } = useAuth();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [botUsername, setBotUsername] = useState("AutoColorsBot");

  useEffect(() => {
    getPublicConfig()
      .then((c) => c.bot_username && setBotUsername(c.bot_username))
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (isAuthenticated) void navigate("/dashboard");
  }, [isAuthenticated, navigate]);

  const loginViaTelegram = async () => {
    const initData =
      typeof window !== "undefined" ? (window as any).Telegram?.WebApp?.initData : null;
    if (!initData) {
      setError("Откройте приложение через Telegram-бот, чтобы войти или зарегистрироваться.");
      return;
    }
    setLoading(true);
    setError("");
    try {
      await telegramLogin(initData);
      void navigate("/dashboard");
    } catch {
      setError("Не удалось войти через Telegram. Попробуйте ещё раз.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.wrap}>
      <div className={styles.card}>
        <div className={styles.title}>Вход через Telegram</div>
        <p className={styles.subtitle}>
          Регистрация и вход выполняются только через Telegram. Нажмите кнопку ниже,
          чтобы привязать аккаунт и получить доступ к ключам VPN.
        </p>
        <Button type="button" onClick={loginViaTelegram} loading={loading} block className={styles.submit}>
          Войти через Telegram
        </Button>
        {error && <div className={styles.error}>{error}</div>}
        <div className={styles.switch}>
          Нет Telegram под рукой? Откройте бота{" "}
          <a href={`https://t.me/${botUsername}`} target="_blank" rel="noreferrer">
            @{botUsername}
          </a>{" "}
          и запустите Mini App.
        </div>
      </div>
    </div>
  );
}
