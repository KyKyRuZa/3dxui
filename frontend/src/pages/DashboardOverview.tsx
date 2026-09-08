import { useAuth } from "@hooks/useAuth";
import { IconBolt } from "@tabler/icons-react";
import styles from "@styles/DashboardOverview.module.css";

export default function DashboardOverview() {
  const { user } = useAuth();

  const metrics = [
    { label: "Пользователь", value: user?.username ?? "—" },
    { label: "Email", value: user?.email ?? "—" },
  ];

  return (
    <div className={styles.root}>
      <div className={styles.kpis}>
        {metrics.map((m, i) => (
          <div 
            key={m.label} 
            className={styles.kpi}
            style={{ animationDelay: `${i * 0.08}s` }}
          >
            <div className={styles.kpiLabel}>{m.label}</div>
            <div className={styles.kpiValue}>{m.value}</div>
          </div>
        ))}
      </div>

      <div className={`section ${styles.sectionFlush}`}>
        <div className="card">
          <div className={styles.quickStart}>
            <div className={styles.quickStartHeader}>
              <div className={styles.quickStartIcon}>
              <IconBolt size={20} stroke={2} />
            </div>
              <div className={styles.cardTitle}>Быстрый старт</div>
            </div>
            <ol className={styles.steps}>
              <li>Установите клиентское приложение на ваше устройство</li>
              <li>Импортируйте ссылку из раздела «Подписка»</li>
              <li>Подключитесь к любому серверу</li>
            </ol>
          </div>
        </div>
      </div>
    </div>
  );
}
