import { Link } from "react-router-dom";
import { IconArrowUpRight, IconBolt, IconShieldCheck, IconUser } from "@tabler/icons-react";
import { useAuth } from "@hooks/useAuth";
import styles from "@styles/DashboardOverview.module.css";

export default function DashboardOverview() {
  const { user } = useAuth();
  return (
    <div className={styles.root}>
      <div className={styles.intro}>
        <div className={styles.kicker}>ОБЗОР АККАУНТА</div>
        <h1>Добро пожаловать{user?.username ? `, ${user.username}` : ""}</h1>
        <p>Управляйте подпиской, настройками подключения и устройствами в одном месте.</p>
      </div>

      <div className={styles.kpis}>
        <div className={styles.kpi}>
          <span className={styles.kpiIcon}><IconUser size={18} /></span>
          <div><div className={styles.kpiLabel}>Пользователь</div><div className={styles.kpiValue}>{user?.username ?? "—"}</div></div>
        </div>
        <div className={styles.kpi}>
          <span className={styles.kpiIcon}><IconShieldCheck size={18} /></span>
          <div><div className={styles.kpiLabel}>Профиль подключения</div><div className={styles.kpiValue}>Управление доступом</div></div>
        </div>
      </div>

      <section className={styles.quickStart}>
        <div className={styles.quickStartIcon}><IconBolt size={21} /></div>
        <div className={styles.quickStartCopy}>
          <h2>Начните с настройки подключения</h2>
          <p>Получите персональную ссылку и QR-код, затем добавьте профиль в совместимое приложение.</p>
          <Link to="/dashboard/subscription">Открыть подписку <IconArrowUpRight size={16} /></Link>
        </div>
      </section>

      <div className={styles.bottomLinks}>
        <Link to="/dashboard/instructions"><span>01</span><strong>Инструкции для устройств</strong><IconArrowUpRight size={16} /></Link>
        <Link to="/dashboard/settings"><span>02</span><strong>Настройки аккаунта</strong><IconArrowUpRight size={16} /></Link>
      </div>
    </div>
  );
}
