import { useEffect, useState } from "react";
import { IconReceipt, IconCurrencyRuble } from "@tabler/icons-react";
import { getBillingHistory, type PaymentHistoryItem } from "@api/billing";
import styles from "@styles/BillingHistory.module.css";

const STATUS_LABEL: Record<string, string> = {
  pending: "Ожидает оплаты",
  succeeded: "Оплачен",
  canceled: "Отменён",
  failed: "Ошибка",
};

const STATUS_CLASS: Record<string, string> = {
  pending: styles.statusPending,
  succeeded: styles.statusSucceeded,
  canceled: styles.statusCanceled,
  failed: styles.statusFailed,
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatAmount(minor: number, currency: string): string {
  const major = (minor / 100).toFixed(2);
  const sym = currency === "RUB" ? "₽" : currency;
  return `${major} ${sym}`;
}

export default function BillingHistory() {
  const [items, setItems] = useState<PaymentHistoryItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    getBillingHistory()
      .then(setItems)
      .catch(() => setError("Не удалось загрузить историю платежей"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className={styles.sectionFlush}><div className={styles.loadingState}>Загрузка…</div></div>;
  }

  if (error) {
    return <div className={styles.sectionFlush}><div className={styles.error}>{error}</div></div>;
  }

  return (
    <div className={styles.sectionFlush}>
      <div className={styles.header}>
        <div className={styles.headerIcon}><IconReceipt size={22} /></div>
        <h2 className={styles.headerTitle}>История платежей</h2>
      </div>

      {items.length === 0 ? (
        <div className="card">
          <div className={styles.emptyState}>Платежей пока нет. После покупки тарифа они появятся здесь.</div>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Дата</th>
                <th>Тариф</th>
                <th>Сумма</th>
                <th>Статус</th>
              </tr>
            </thead>
            <tbody>
              {items.map((p) => (
                <tr key={p.id}>
                  <td className={styles.cellDate}>{formatDate(p.created_at)}</td>
                  <td>{p.plan_name}</td>
                  <td>
                    <span className={styles.cellAmount}>
                      <IconCurrencyRuble size={14} stroke={2.5} />
                      {formatAmount(p.amount_minor, p.currency)}
                    </span>
                  </td>
                  <td>
                    <span className={`${styles.badge} ${STATUS_CLASS[p.status] || styles.statusUnknown}`}>
                      {STATUS_LABEL[p.status] || p.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
