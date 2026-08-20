import { Outlet, NavLink, Link, useLocation } from "react-router-dom";
import styles from "@styles/DashboardShell.module.css";

export default function DashboardShell() {
  const location = useLocation();

  const links = [
    { to: "/dashboard", label: "Профиль", end: true },
    { to: "/dashboard/subscription", label: "Подписка" },
    { to: "/dashboard/instructions", label: "Инструкции" },
    { to: "/dashboard/settings", label: "Настройки" },
  ];

  return (
    <div className={styles.shell}>
      <aside className={styles.sidebar}>
        <Link to="/dashboard" className={styles.brand}>
          <span className={styles.brandIcon}>●</span>
          TheNoMoreBlocks
        </Link>
        {links.map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            end={l.end}
            className={({ isActive }) => `${styles.navLink} ${isActive ? styles.active : ""}`}
          >
            {l.label}
          </NavLink>
        ))}
      </aside>

      <div className={styles.main}>
        <header className={styles.header}>
          <div className={styles.headerTitle}>
            {location.pathname === "/dashboard" && "Профиль"}
            {location.pathname === "/dashboard/subscription" && "Подписка"}
            {location.pathname === "/dashboard/instructions" && "Инструкции"}
            {location.pathname === "/dashboard/settings" && "Настройки"}
          </div>
          <div className="badge">
            <span className="dot" /> Активна
          </div>
        </header>
        <main className={styles.content}>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
