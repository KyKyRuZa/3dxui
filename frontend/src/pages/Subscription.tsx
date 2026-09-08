import { useEffect, useRef, useState } from "react";
import { IconAlertTriangle, IconKey, IconCircleCheck } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { Button } from "@components/Button";
import { useAuth } from "@hooks/useAuth";
import { activateSubscription, type Subscription as Sub } from "@api/subscription";
import { getReferral, type ReferralStats } from "@api/referral";
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
  const [referral, setReferral] = useState<ReferralStats | null>(null);
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
    getReferral()
      .then(setReferral)
      .catch(() => {});
  }, []);

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
    <div className={styles.sectionFlush}>
      {error && <div className={styles.error}>{error}</div>}

      {expired ? (
        <div className={`${styles.statusBanner} ${styles.statusBannerExpired}`}>
          <div className={styles.statusHeader}>
            <div className={`${styles.statusIcon} ${styles.statusIconExpired}`}><IconAlertTriangle size={24} /></div>
            <div className={styles.statusTitle}>Подписка истекла</div>
          </div>
          <p className={styles.statusMeta}>
            Вы снова без защиты. Верните доступ одним тапом: купите тариф и получите готовый конфиг за минуту.
            Или пригласите друга и получите <strong>+7 дней бесплатно</strong>.
          </p>
          <div className={styles.actions}>
          <Button onClick={() => navigate("/pricing")}>
            <IconKey size={18} /> Купить ключ VPN
          </Button>
          </div>
        </div>
      ) : sub?.expires_at ? (
        <div className={`${styles.statusBanner} ${styles.statusBannerActive}`}>
          <div className={styles.statusHeader}>
            <div className={`${styles.statusIcon} ${styles.statusIconActive}`}><IconCircleCheck size={24} /></div>
            <div className={styles.statusTitle}>Подписка активна</div>
          </div>
          <p className={styles.statusMeta}>
            До <strong>{formatDate(sub.expires_at)}</strong> (осталось <strong>{daysLeft(sub.expires_at)}</strong>)
          </p>
        </div>
      ) : null}

      {sub && (
        <div className={styles.configCard}>
          <div className={styles.configGrid}>
            <div className={styles.qrSection}>
              {qrUrl ? (
                <>
                  <div className={styles.qrWrap}>
                    <img src={qrUrl} alt="QR код" className={styles.qrImage} />
                  </div>
                  <div className={styles.qrLabel}>QR код (VLESS)</div>
                </>
              ) : (
                <div className={styles.loadingState}>Загрузка QR…</div>
              )}
            </div>

            <div className={styles.linksSection}>
              <div className={styles.linkGroup}>
                <div className={styles.linkLabel}>VLESS ссылка</div>
                <div className={styles.linkBox}>{sub.vless}</div>
              </div>

              <div className={styles.linkGroup}>
                <div className={styles.linkLabel}>Ссылка подписки</div>
                <div className={styles.linkBox}>{sub.subscription_url}</div>
              </div>

              <div className={styles.actions}>
                <Button disabled={!sub} onClick={() => sub && copy(sub.vless, "vless")}>
                  {copied === "vless" ? "Скопировано" : "Скопировать VLESS"}
                </Button>
                <Button
                  variant="secondary"
                  disabled={!sub}
                  onClick={() => sub && copy(sub.subscription_url, "link")}
                >
                  {copied === "link" ? "Скопировано" : "Скопировать ссылку"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {!sub && !error && (
        <div className="card">
          <div className={styles.loadingState}>Загрузка конфигурации…</div>
        </div>
      )}

      <div className={styles.referralCard}>
        <Referral stats={referral} />
      </div>
    </div>
  );
}
