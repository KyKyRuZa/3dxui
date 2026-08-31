import { useAuth } from "@hooks/useAuth";
import styles from "@styles/Settings.module.css";

export default function Settings() {
  const { user } = useAuth();
  return (
    <div className={styles.page}>
      <div className={styles.grid}>
        <div className={styles.card}>
          <h3 className={styles.title}>Аккаунт</h3>
          <div className={styles.form}>
            <div className={styles.field}>
              <label className={styles.label}>Пользователь</label>
              <input className={styles.input} value={user?.username ?? ""} readOnly />
            </div>
            <p className={styles.hint}>
              Вход и регистрация выполняются только через Telegram. Пароль не используется.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
