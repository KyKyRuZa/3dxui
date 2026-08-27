import { useEffect, useState } from "react";
import { Button } from "@components/Button";
import { getReferral, type ReferralStats } from "@api/referral";
import styles from "@styles/Referral.module.css";

export default function Referral() {
  const [stats, setStats] = useState<ReferralStats | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    getReferral()
      .then(setStats)
      .catch(() => setError("Не удалось загрузить реферальную программу"));
  }, []);

  const link = stats
    ? `https://t.me/${stats.bot_username}?start=${stats.referral_code}`
    : "";

  const copy = async () => {
    if (!link) return;
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("Не удалось скопировать ссылку");
    }
  };

  if (error) {
    return <div className={styles.error}>{error}</div>;
  }

  return (
    <div className={`card ${styles.wrap}`}>
      <div className={styles.title}>🤝 Реферальная программа</div>
      <p className={styles.lead}>
        Пригласите друзей и получайте <b>+7 дней бесплатно</b> за каждого, кто
        купит платный тариф. Друг по вашей ссылке получает бонус к подписке.
      </p>

      {stats ? (
        <>
          <div className={styles.metrics}>
            <div className={styles.metric}>
              <div className={styles.metricValue}>{stats.invited}</div>
              <div className={styles.metricLabel}>Приглашено</div>
            </div>
            <div className={styles.metric}>
              <div className={styles.metricValue}>{stats.earned_days}</div>
              <div className={styles.metricLabel}>Бонусных дней</div>
            </div>
          </div>

          <div className={styles.linkLabel}>Ваша реферальная ссылка</div>
          <div className={styles.linkBox}>{link}</div>
          <div className={styles.actions}>
            <Button block onClick={copy}>
              {copied ? "Скопировано" : "Скопировать ссылку"}
            </Button>
          </div>
        </>
      ) : (
        <div className={styles.linkBox}>Загрузка…</div>
      )}
    </div>
  );
}
