import { useEffect, useState, type FormEvent } from "react";
import { Button } from "../components/Button";
import { useAuth } from "../hooks/useAuth";
import { getProfile, updateProfile, changePassword } from "../api/auth";
import styles from "./Settings.module.css";

function errorMessage(err: unknown): string {
  const data = (err as { response?: { data?: { error?: string } } })?.response?.data;
  return data?.error ?? "Произошла ошибка, попробуйте позже";
}

export default function Settings() {
  const { user, setUser } = useAuth();
  const [email, setEmail] = useState("");
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (user) setEmail(user.email);
  }, [user]);

  const saveEmail = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setErr("");
    setMsg("");
    setLoading(true);
    try {
      await updateProfile(email);
      const fresh = await getProfile();
      setUser(fresh);
      setMsg("Профиль сохранён");
    } catch (ex) {
      setErr(errorMessage(ex));
    } finally {
      setLoading(false);
    }
  };

  const savePassword = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setErr("");
    setMsg("");
    setLoading(true);
    try {
      await changePassword(current, next);
      setMsg("Пароль обновлён");
      setCurrent("");
      setNext("");
    } catch (ex) {
      setErr(errorMessage(ex));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.page}>
      {msg && <div className={styles.success}>{msg}</div>}
      {err && <div className={styles.error}>{err}</div>}

      <div className={styles.grid}>
        <div className={styles.card}>
          <h3 className={styles.title}>Профиль</h3>
          <form onSubmit={saveEmail} className={styles.form}>
            <div className={styles.field}>
              <label className={styles.label}>Email</label>
              <input
                className={styles.input}
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className={styles.actions}>
              <Button type="submit" loading={loading}>Сохранить</Button>
            </div>
          </form>
        </div>

        <div className={styles.card}>
          <h3 className={styles.title}>Смена пароля</h3>
          <form onSubmit={savePassword} className={styles.form}>
            <div className={styles.field}>
              <label className={styles.label}>Текущий пароль</label>
              <input
                className={styles.input}
                type="password"
                value={current}
                onChange={(e) => setCurrent(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label}>Новый пароль</label>
              <input
                className={styles.input}
                type="password"
                value={next}
                onChange={(e) => setNext(e.target.value)}
                autoComplete="new-password"
                minLength={8}
                required
              />
            </div>
            <div className={styles.actions}>
              <Button type="submit" variant="secondary" loading={loading}>
                Обновить пароль
              </Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
