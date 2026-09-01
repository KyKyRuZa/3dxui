import { Link } from "react-router-dom";
import styles from "@styles/Footer.module.css";

export default function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={styles.inner}>
        <span>© {new Date().getFullYear()} TheNoMoreBlocks</span>
        <span>
          Все права защищены •{" "}
          <Link to="/privacy">Политика конфиденциальности</Link>
        </span>
      </div>
    </footer>
  );
}
