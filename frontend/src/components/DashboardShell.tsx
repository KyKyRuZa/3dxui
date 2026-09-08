import { Outlet, NavLink, useLocation } from "react-router-dom";
import {
  IconHome,
  IconHexagon,
  IconCreditCard,
  IconLayoutGrid,
  IconSettings,
} from "@tabler/icons-react";
import styles from "@styles/DashboardShell.module.css";

const navItems = [
  { to: "/dashboard", label: "Профиль", end: true, icon: IconHome },
  { to: "/dashboard/subscription", label: "Подписка", icon: IconHexagon },
  { to: "/dashboard/billing", label: "Платежи", icon: IconCreditCard },
  { to: "/dashboard/instructions", label: "Инструкции", icon: IconLayoutGrid },
  { to: "/dashboard/settings", label: "Настройки", icon: IconSettings },
];

export default function DashboardShell() {
  const location = useLocation();

  return (
    <div className={styles.shell}>
      <aside className={styles.sidebar}>
        <nav aria-label="Dashboard">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) => `${styles.navLink} ${isActive ? styles.active : ""}`}
              >
                <span aria-hidden="true" style={{ display: "inline-flex", alignItems: "center" }}>
                  <Icon size={18} stroke={2} />
                </span>
                {item.label}
              </NavLink>
            );
          })}
        </nav>
      </aside>

      <div className={styles.main}>
        <header className={styles.header}>
          <div className={styles.headerTitle}>
            {location.pathname === "/dashboard" && "Профиль"}
            {location.pathname === "/dashboard/subscription" && "Подписка"}
            {location.pathname === "/dashboard/billing" && "Платежи"}
            {location.pathname === "/dashboard/instructions" && "Инструкции"}
            {location.pathname === "/dashboard/settings" && "Настройки"}
          </div>
          <div className={styles.headerStatus}>
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
