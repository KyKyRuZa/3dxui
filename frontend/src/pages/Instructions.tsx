import styles from "@styles/Instructions.module.css";

const items = [
  {
    icon: "🤖",
    title: "Android",
    desc: "Скачайте клиентское приложение, добавьте профиль через «Добавить профиль».",
  },
  { icon: "🍎", title: "iOS", desc: "Отсканируйте QR или импортируйте ссылку в приложение." },
  { icon: "🪟", title: "Windows", desc: "Добавьте профиль через импорт конфигурации в клиентском ПО." },
  { icon: "🐧", title: "macOS / Linux", desc: "Импортируйте подписку через клиентское приложение." },
];

const recommended = "Рекомендуемое приложение — Happ: универсальный клиент для всех платформ.";

export default function Instructions() {
  return (
    <div className={`section ${styles.sectionFlush}`}>
      <div className={styles.grid}>
        {items.map((it) => (
          <div key={it.title} className={styles.item}>
            <div className={styles.itemHeader}>
              <div className={styles.itemIcon}>{it.icon}</div>
              <div className={styles.itemTitle}>{it.title}</div>
            </div>
            <p className={styles.itemDesc}>{it.desc}</p>
          </div>
        ))}
      </div>

      <div className={styles.recommended}>
        <span className={styles.recommendedIcon}>⭐</span>
        {recommended}
      </div>
    </div>
  );
}
