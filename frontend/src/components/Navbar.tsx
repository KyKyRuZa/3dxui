import { Link, useLocation } from "react-router-dom";
import { useEffect, useState } from "react";
import { useAuth } from "@hooks/useAuth";
import styles from "@styles/Navbar.module.css";

export default function Navbar() {
  const { user, isAuthenticated, logout } = useAuth();
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const isAuthPage = location.pathname === "/login" || location.pathname === "/register";

  useEffect(() => {
    setOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  return (
    <nav className={styles.nav} aria-label="Main">
      <div className={styles.inner}>
        <Link to="/" className={styles.brand} aria-label="Walyny4 VPN home">
          <span className={styles.brandIcon} aria-hidden="true">
            ●
          </span>
          <span className={styles.brandText}>Walyny4 VPN</span>
        </Link>

        <div className={styles.links}>
          <Link to="/pricing">Тарифы</Link>
          <Link to="/dashboard/instructions">Инструкции</Link>
          <Link to="/admin">Admin</Link>
        </div>

        <div className={styles.actions}>
          {isAuthenticated ? (
            <>
              <Link to="/dashboard" className={styles.avatarLink} title="Аккаунт" aria-label="Account">
                <span className={styles.avatar}>{user?.username?.charAt(0).toUpperCase()}</span>
              </Link>
              <button onClick={logout} className={styles.logoutBtn}>
                Выйти
              </button>
            </>
          ) : !isAuthPage ? (
            <>
              <Link to="/login" className={styles.linkButton}>
                Вход
              </Link>
              <Link to="/register" className="button-primary">
                Регистрация
              </Link>
            </>
          ) : null}
        </div>

        <button
          type="button"
          className={`${styles.burger} ${open ? styles.burgerOpen : ""}`}
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-controls="navbar-mobile"
          aria-label={open ? "Закрыть меню" : "Открыть меню"}
        >
          <span className={styles.burgerLine} />
          <span className={styles.burgerLine} />
          <span className={styles.burgerLine} />
        </button>

        <div
          id="navbar-mobile"
          className={`${styles.mobileOverlay} ${open ? styles.mobileOverlayOpen : ""}`}
          aria-hidden={!open}
        >
          <div className={styles.mobilePanel}>
            <div className={styles.mobileHeader}>
              <span className={styles.mobileTitle}>Меню</span>
              <button
                type="button"
                className={styles.mobileClose}
                onClick={() => setOpen(false)}
                aria-label="Закрыть меню"
              >
                ×
              </button>
            </div>

            <div className={styles.mobileLinks}>
              <Link to="/pricing" className={styles.mobileLink}>
                Тарифы
              </Link>
              <Link to="/dashboard/instructions" className={styles.mobileLink}>
                Инструкции
              </Link>
              <Link to="/admin" className={styles.mobileLink}>
                Admin
              </Link>
              {!isAuthenticated && !isAuthPage && (
                <Link to="/login" className={styles.mobileLink}>
                  Вход
                </Link>
              )}
              {!isAuthenticated && !isAuthPage && (
                <Link to="/register" className={styles.mobileLink}>
                  Регистрация
                </Link>
              )}
              {isAuthenticated && (
                <button onClick={logout} className={styles.mobileLinkDanger}>
                  Выйти
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </nav>
  );
}
