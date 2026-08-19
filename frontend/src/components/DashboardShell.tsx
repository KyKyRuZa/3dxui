import { Outlet, NavLink, Link, useLocation } from "react-router-dom";
import styles from "./DashboardShell.module.css";

export default function DashboardShell() {
  const location = useLocation();

  const links = [
    { to: "/dashboard", label: "Обзор", end: true },
    { to: "/dashboard/subscription", label: "Подписка" },
    { to: "/dashboard/instructions", label: "Инструкции" },
    { to: "/dashboard/settings", label: "Настройки" },
  ];

  return (
    <div style={{ display: "flex", minHeight: "100vh" }}>
      <aside className={styles.sidebar}>
        <Link to="/dashboard" className={styles.brand}>
          <span className={styles.brandIcon}>●</span>
          Nexus
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

      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <header className={styles.header}>
          <div style={{ fontWeight: 600 }}>
            {location.pathname === "/dashboard" && "Обзор"}
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
