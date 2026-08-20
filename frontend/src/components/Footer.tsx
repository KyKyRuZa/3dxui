import styles from "@styles/Footer.module.css";

export default function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={styles.inner}>
        <span>© {new Date().getFullYear()} TheNoMoreBlocks</span>
        <span>Все права защищены • Политика конфиденциальности • Пользовательское соглашение</span>
      </div>
    </footer>
  );
}
