import { useState } from "react";
import { IconHeartHandshake, IconCopy } from "@tabler/icons-react";
import { Button } from "@components/Button";
import type { ReferralStats } from "@api/referral";
import styles from "@styles/Referral.module.css";

type Props = {
  stats: ReferralStats | null;
};

export default function Referral({ stats }: Props) {
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

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
      <div className={styles.title}><IconHeartHandshake size={20} /> Реферальная программа</div>
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
          <div className={styles.linkBoxWrap}>
            <div className={styles.linkBox}>{link}</div>
            <button
              type="button"
              className={styles.copyIconBtn}
              onClick={copy}
              title={copied ? "Скопировано" : "Скопировать"}
            >
              <IconCopy size={16} />
            </button>
          </div>
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
