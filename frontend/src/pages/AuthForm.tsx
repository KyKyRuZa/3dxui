import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "../components/Button";
import { useAuth } from "../hooks/useAuth";
import styles from "./Auth.module.css";

function errorMessage(err: unknown): string {
  const data = (err as { response?: { data?: { error?: string } } })?.response?.data;
  return data?.error ?? "Произошла ошибка, попробуйте позже";
}

export default function AuthForm({ type = "login" }: { type?: "login" | "register" }) {
  const navigate = useNavigate();
  const { login, register } = useAuth();

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const onSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      if (type === "login") {
        await login(username, password);
      } else {
        await register(username, email, password);
      }
      void navigate("/dashboard");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.wrap}>
      <div className={styles.card}>
        <div className={styles.title}>{type === "login" ? "Вход" : "Регистрация"}</div>
        <form className={styles.form} onSubmit={onSubmit}>
          <div className={styles.field}>
            <label className={styles.label}>Имя пользователя</label>
            <input
              className={styles.input}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
              minLength={3}
            />
          </div>
          {type === "register" && (
            <div className={styles.field}>
              <label className={styles.label}>Email</label>
              <input
                className={styles.input}
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
                required
              />
            </div>
          )}
          <div className={styles.field}>
            <label className={styles.label}>Пароль</label>
            <input
              className={styles.input}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={type === "login" ? "current-password" : "new-password"}
              required
              minLength={8}
            />
          </div>
          <Button type="submit" loading={loading} block className={styles.submit}>
            {type === "login" ? "Войти" : "Создать аккаунт"}
          </Button>
          {error && <div className={styles.error}>{error}</div>}
        </form>
        <div className={styles.switch}>
          {type === "login" ? "Нет аккаунта? " : "Уже есть аккаунт? "}
          <Link to={type === "login" ? "/register" : "/login"}>
            {type === "login" ? "Зарегистрироваться" : "Войти"}
          </Link>
        </div>
      </div>
    </div>
  );
}
