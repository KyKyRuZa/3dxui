import { Link, useLocation } from "react-router-dom";
import { useAuth } from "@hooks/useAuth";
import styles from "@styles/Navbar.module.css";

export default function Navbar() {
  const { user, isAuthenticated, logout } = useAuth();
  const location = useLocation();
  const isAuthPage = location.pathname === "/login" || location.pathname === "/register";

  return (
    <nav className={styles.nav}>
      <div className={styles.inner}>
        <Link to="/" className={styles.brand}>
          <span className={styles.brandIcon}>●</span>
          TheNoMoreBlocks
        </Link>

        <div className={styles.links}>
          <Link to="/pricing">Тарифы</Link>
          <Link to="/dashboard/instructions">Инструкции</Link>
        </div>

        <div className={styles.actions}>
          {isAuthenticated ? (
            <>
              <Link to="/dashboard" className={styles.avatarLink} title="Аккаунт">
                <span className={styles.avatar}>{user?.username?.charAt(0).toUpperCase()}</span>
              </Link>
              <button onClick={logout} className={styles.logoutBtn}>
                Выйти
              </button>
            </>
          ) : !isAuthPage ? (
            <>
              <Link to="/login">Вход</Link>
              <Link to="/register" className="button-primary">
                Регистрация
              </Link>
            </>
          ) : null}
        </div>
      </div>
    </nav>
  );
}
