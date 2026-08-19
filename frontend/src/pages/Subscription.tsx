import { useEffect, useState } from "react";
import { Button } from "../components/Button";
import { useAuth } from "../hooks/useAuth";
import {
  getSubscription,
  getVLESSConfig,
  getSingBoxConfig,
  type Subscription as Sub,
  type VLESSConfig,
} from "../api/subscription";
import styles from "./Subscription.module.css";

export default function Subscription() {
  const { user } = useAuth();
  const [sub, setSub] = useState<Sub | null>(null);
  const [vless, setVless] = useState<VLESSConfig | null>(null);
  const [singbox, setSingbox] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"key" | "link" | "vless" | "singbox" | null>(null);

  useEffect(() => {
    getSubscription()
      .then(setSub)
      .catch(() => setError("Не удалось загрузить подписку"));
    getVLESSConfig()
      .then(setVless)
      .catch(() => {});
    getSingBoxConfig()
      .then(setSingbox)
      .catch(() => {});
  }, [user?.id]);

  const copy = async (text: string, which: "key" | "link" | "vless" | "singbox") => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(which);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      setError("Не удалось скопировать в буфер обмена");
    }
  };

  const downloadSingBox = () => {
    if (!singbox) return;
    const blob = new Blob([singbox], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "singbox-config.json";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div className="section" style={{ padding: 0 }}>
      {error && <div className={styles.error}>{error}</div>}

      <div className="card">
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Подписка (3x-ui)</div>
        {sub ? (
          <div className={styles.keyBox}>{sub.subscription_url}</div>
        ) : (
          <div className={styles.keyBox}>Загрузка…</div>
        )}
        <div className={styles.actions}>
          <Button disabled={!sub} onClick={() => sub && copy(sub.subscription_url, "link")}>
            {copied === "link" ? "Скопировано" : "Скопировать ссылку"}
          </Button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>VLESS конфиг (Hiddify)</div>
        {vless ? (
          <div className={styles.keyBox}>{vless.config_url}</div>
        ) : (
          <div className={styles.keyBox}>Загрузка…</div>
        )}
        <div className={styles.actions}>
          <Button disabled={!vless} onClick={() => vless && copy(vless.config_url, "vless")}>
            {copied === "vless" ? "Скопировано" : "Скопировать VLESS ссылку"}
          </Button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Sing-box JSON</div>
        {singbox ? (
          <div className={styles.keyBox}>Готово к скачиванию</div>
        ) : (
          <div className={styles.keyBox}>Загрузка…</div>
        )}
        <div className={styles.actions}>
          <Button disabled={!singbox} onClick={downloadSingBox}>
            Скачать singbox-config.json
          </Button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Имя пользователя</div>
        <div className={styles.keyBox}>{sub?.username ?? vless?.username ?? "—"}</div>
        <div className={styles.actions}>
          <Button
            variant="secondary"
            disabled={!sub && !vless}
            onClick={() => copy(sub?.username || vless?.username || "", "key")}
          >
            {copied === "key" ? "Скопировано" : "Скопировать"}
          </Button>
        </div>
      </div>
    </div>
  );
}
