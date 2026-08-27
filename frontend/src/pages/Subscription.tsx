import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@components/Button";
import { useAuth } from "@hooks/useAuth";
import { activateSubscription, type Subscription as Sub } from "@api/subscription";
import Referral from "@components/Referral";
import styles from "@styles/Subscription.module.css";

function formatDate(ms: number): string {
  return new Date(ms).toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function daysLeft(ms: number): string {
  const days = (ms - Date.now()) / 86400000;
  if (days >= 1) return `~${Math.ceil(days)} дн.`;
  return "менее суток";
}

export default function Subscription() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [sub, setSub] = useState<Sub | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"link" | "vless" | null>(null);
  const [qrUrl, setQrUrl] = useState("");
  const mountedRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    activateSubscription()
      .then(setSub)
      .catch(() => setError("Не удалось активировать подписку"));
  }, [user?.id]);

  useEffect(() => {
    if (!sub?.vless) return;
    let cancelled = false;
    import("qrcode").then((QRCode) => {
      if (!cancelled) {
        QRCode.toDataURL(sub.vless, { width: 220, margin: 2 }).then(setQrUrl);
      }
    });
    return () => {
      cancelled = true;
    };
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

  const expired = !!sub?.expires_at && sub.expires_at <= Date.now();

  return (
    <div className={`section ${styles.sectionFlush}`}>
      {error && <div className={styles.error}>{error}</div>}

      {expired && (
        <div className={styles.expiredBanner}>
          <div className={styles.expiredTitle}>
            🚨 Подписка истекла — вы снова без защиты
          </div>
          <p className={styles.expiredText}>
            Пока без VPN нужные сайты и сервисы для вас закрыты. Верните доступ
            одним тапом: купите тариф и получите готовый конфиг за минуту. Или
            пригласите друга и получите <b>+7 дней бесплатно</b>.
          </p>
          <div className={styles.expiredActions}>
            <Button onClick={() => navigate("/pricing")}>
              🔑 Купить ключ VPN
            </Button>
          </div>
        </div>
      )}

      {sub?.expires_at && !expired && (
        <div className={styles.activeBadge}>
          ✅ Подписка активна до <b>{formatDate(sub.expires_at)}</b> (осталось{" "}
          {daysLeft(sub.expires_at)})
        </div>
      )}

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

      <div className={`card ${styles.cardMt}`}>
        <Referral />
      </div>
    </div>
  );
}
