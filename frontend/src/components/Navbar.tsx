import { Link, useLocation } from "react-router-dom";
import { useEffect, useState } from "react";
import { IconHexagon, IconMenu2, IconMoon, IconSun, IconX } from "@tabler/icons-react";
import { useAuth } from "@hooks/useAuth";
import styles from "@styles/Navbar.module.css";

const links = [
  { id: "features", label: "Возможности" },
  { id: "servers", label: "Сеть серверов" },
  { id: "pricing", label: "Тарифы" },
  { id: "faq", label: "Вопросы" },
];

export default function Navbar() {
  const { user, isAuthenticated, logout } = useAuth();
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const [theme, setTheme] = useState<"light" | "dark">(() =>
    document.documentElement.dataset.theme === "dark" ? "dark" : "light",
  );
  const isAuthPage = location.pathname === "/login" || location.pathname === "/register";
  const sectionTo = (id: string) => (location.pathname === "/" ? `#${id}` : `/#${id}`);

  useEffect(() => setOpen(false), [location.pathname, location.hash]);
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.querySelector('meta[name="theme-color"]')?.setAttribute(
      "content",
      theme === "dark" ? "#0b1520" : "#f4f7fa",
    );
    try { window.localStorage.setItem("walyny4-theme", theme); } catch { /* Storage can be disabled. */ }
  }, [theme]);
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [open]);

  return (
    <header className={styles.nav}>
      <div className={`container ${styles.inner}`}>
        <Link to="/" className={styles.brand} aria-label="Walyny4 — главная">
          <span className={styles.brandIcon} aria-hidden="true"><IconHexagon size={19} stroke={2.2} /></span>
          <span className={styles.brandText}>walyny4<span>.</span></span>
        </Link>

        {!isAuthPage && (
          <nav className={styles.desktopNav} aria-label="Основная навигация">
            {links.map((link) => <Link key={link.id} to={sectionTo(link.id)}>{link.label}</Link>)}
          </nav>
        )}

        <div className={`${styles.actions} ${isAuthPage ? styles.authActions : ""}`}>
          <button
            type="button"
            className={styles.themeToggle}
            onClick={() => setTheme((current) => current === "dark" ? "light" : "dark")}
            aria-label={theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему"}
            aria-pressed={theme === "dark"}
            title={theme === "dark" ? "Светлая тема" : "Тёмная тема"}
          >
            {theme === "dark" ? <IconSun size={17} /> : <IconMoon size={17} />}
          </button>
          {isAuthenticated ? (
            <>
              <Link to="/dashboard" className={styles.accountLink}>
                <span className={styles.avatar}>{user?.username?.charAt(0).toUpperCase() || "U"}</span>
                <span>Личный кабинет</span>
              </Link>
              <button onClick={logout} className={styles.logoutBtn}>Выйти</button>
            </>
          ) : !isAuthPage ? (
            <>
              <Link to="/login" className={styles.loginLink}>Войти</Link>
              <Link to="/register" className={styles.signupLink}>Начать бесплатно</Link>
            </>
          ) : null}
        </div>

        {!isAuthPage && (
          <button type="button" className={styles.burger} onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-controls="navbar-mobile" aria-label={open ? "Закрыть меню" : "Открыть меню"}>
            {open ? <IconX size={22} /> : <IconMenu2 size={22} />}
          </button>
        )}
      </div>

      {!isAuthPage && (
        <div id="navbar-mobile" className={`${styles.mobileOverlay} ${open ? styles.mobileOverlayOpen : ""}`} aria-hidden={!open}>
          <div className={styles.mobilePanel}>
            <div className={styles.mobileLinks}>
              {links.map((link) => <Link key={link.id} to={sectionTo(link.id)}>{link.label}</Link>)}
              <div className={styles.mobileDivider} />
              <button
                type="button"
                className={styles.mobileTheme}
                aria-pressed={theme === "dark"}
                onClick={() => setTheme((current) => current === "dark" ? "light" : "dark")}
              >
                {theme === "dark" ? <IconSun size={17} /> : <IconMoon size={17} />}
                {theme === "dark" ? "Светлая тема" : "Тёмная тема"}
              </button>
              {isAuthenticated ? (
                <>
                  <Link to="/dashboard">Личный кабинет</Link>
                  <button onClick={logout}>Выйти</button>
                </>
              ) : (
                <>
                  <Link to="/login">Войти</Link>
                  <Link to="/register" className={styles.mobileSignup}>Начать бесплатно</Link>
                </>
              )}
            </div>
          </div>
          <button className={styles.overlayClose} aria-label="Закрыть меню" onClick={() => setOpen(false)} />
        </div>
      )}
    </header>
  );
}
