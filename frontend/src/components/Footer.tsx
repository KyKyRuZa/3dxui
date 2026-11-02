import { Link } from "react-router-dom";
import { IconHexagon } from "@tabler/icons-react";
import styles from "@styles/Footer.module.css";

export default function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={`container ${styles.inner}`}>
        <Link to="/" className={styles.brand}>
          <span className={styles.brandIcon}><IconHexagon size={17} /></span>
          <span>walyny4<span>.</span></span>
        </Link>
        <span className={styles.copyright}>© {new Date().getFullYear()} Walyny4. Все права защищены.</span>
        <Link to="/privacy" className={styles.privacy}>Политика конфиденциальности</Link>
      </div>
    </footer>
  );
}
