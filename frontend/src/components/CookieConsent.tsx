import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import styles from "@styles/CookieConsent.module.css";

const CONSENT_KEY = "cookie_consent";

export default function CookieConsent() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const consent = localStorage.getItem(CONSENT_KEY);
    if (!consent) {
      setVisible(true);
    }
  }, []);

  const accept = () => {
    localStorage.setItem(CONSENT_KEY, "accepted");
    setVisible(false);
  };

  if (!visible) return null;

  return (
    <div className={styles.banner}>
      <div className={styles.text}>
        Мы используем cookies для поддержания вашей сессии. Подробнее в{" "}
        <Link to="/privacy">Политике конфиденциальности</Link>.
      </div>
      <button className={styles.button} onClick={accept}>
        Принять
      </button>
    </div>
  );
}
