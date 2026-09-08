import { Outlet, NavLink, Link, useLocation } from "react-router-dom";
import styles from "@styles/DashboardShell.module.css";

const navItems = [
  { to: "/dashboard", label: "Профиль", end: true, icon: "◉" },
  { to: "/dashboard/subscription", label: "Подписка", icon: "◇" },
  { to: "/dashboard/instructions", label: "Инструкции", icon: "▣" },
  { to: "/dashboard/settings", label: "Настройки", icon: "⚙" },
];

export default function DashboardShell() {
  const location = useLocation();

  return (
    <div className={styles.shell}>
      <aside className={styles.sidebar}>
        <Link to="/dashboard" className={styles.brand}>
          <span className={styles.brandIcon} aria-hidden="true">●</span>
          Walyny4 vpn
        </Link>
        <nav aria-label="Dashboard">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => `${styles.navLink} ${isActive ? styles.active : ""}`}
            >
              <span aria-hidden="true" style={{ fontSize: "var(--text-sm)", opacity: 0.7 }}>{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className={styles.main}>
        <header className={styles.header}>
          <div className={styles.headerTitle}>
            {location.pathname === "/dashboard" && "Профиль"}
            {location.pathname === "/dashboard/subscription" && "Подписка"}
            {location.pathname === "/dashboard/instructions" && "Инструкции"}
            {location.pathname === "/dashboard/settings" && "Настройки"}
          </div>
          <div className={styles.headerStatus}>
            <span className="dot" />
            Активна
          </div>
        </header>
        <main className={styles.content}>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
