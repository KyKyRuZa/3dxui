import { useAuth } from "@hooks/useAuth";
import styles from "@styles/DashboardOverview.module.css";

export default function DashboardOverview() {
  const { user } = useAuth();

  const metrics = [
    { label: "Статус", value: user?.is_active ? "Активна" : "Неактивна" },
    { label: "Пользователь", value: user?.username ?? "—" },
    { label: "Email", value: user?.email ?? "—" },
  ];

  return (
    <div>
      <div className={styles.kpis}>
        {metrics.map((m) => (
          <div key={m.label} className={styles.kpi}>
            <div className={styles.kpiLabel}>{m.label}</div>
            <div className={styles.kpiValue}>{m.value}</div>
          </div>
        ))}
      </div>

      <div className={`section ${styles.sectionFlush}`}>
        <div className="card">
          <div className={styles.cardTitle}>Быстрый старт</div>
          <ol className={styles.steps}>
            <li>Установите клиентское приложение на ваше устройство</li>
            <li>Импортируйте ссылку из раздела «Подписка»</li>
            <li>Подключитесь к любому серверу</li>
          </ol>
        </div>
      </div>
    </div>
  );
}
