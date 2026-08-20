import { useEffect, useState } from "react";
import QRCode from "qrcode";
import { Button } from "../components/Button";
import { useAuth } from "../hooks/useAuth";
import { activateSubscription, type Subscription as Sub } from "../api/subscription";
import styles from "./Subscription.module.css";

export default function Subscription() {
  const { user } = useAuth();
  const [sub, setSub] = useState<Sub | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"link" | "vless" | null>(null);
  const [qrUrl, setQrUrl] = useState("");

  useEffect(() => {
    activateSubscription()
      .then(setSub)
      .catch(() => setError("Не удалось активировать подписку"));
  }, [user?.id]);

  useEffect(() => {
    if (sub?.vless) {
      QRCode.toDataURL(sub.vless, { width: 220, margin: 2 }).then(setQrUrl);
    }
  }, [sub?.vless]);

  const copy = async (text: string, which: "link" | "vless") => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(which);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      setError("Не удалось скопировать в буфер обмена");
    }
  };

  return (
    <div className={`section ${styles.sectionFlush}`}>
      {error && <div className={styles.error}>{error}</div>}

      <div className="card">
        <div className={styles.cardTitle}>QR код (VLESS)</div>
        {qrUrl ? (
          <div className={styles.qrWrap}>
            <img src={qrUrl} alt="QR код" className={styles.qrImage} />
          </div>
        ) : (
          <div className={styles.keyBox}>Загрузка…</div>
        )}
        <div className={styles.actions}>
          <Button disabled={!sub} onClick={() => sub && copy(sub.vless, "vless")}>
            {copied === "vless" ? "Скопировано" : "Скопировать VLESS ссылку"}
          </Button>
        </div>
      </div>

      <div className={`card ${styles.cardMt}`}>
        <div className={styles.cardTitle}>VLESS ссылка</div>
        {sub ? (
          <div className={styles.keyBox}>{sub.vless}</div>
        ) : (
          <div className={styles.keyBox}>Загрузка…</div>
        )}
        <div className={styles.actions}>
          <Button disabled={!sub} onClick={() => sub && copy(sub.vless, "vless")}>
            {copied === "vless" ? "Скопировано" : "Скопировать VLESS ссылку"}
          </Button>
        </div>
      </div>

      <div className={`card ${styles.cardMt}`}>
        <div className={styles.cardTitle}>Ссылка подписки</div>
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
    </div>
  );
}
