import styles from "@styles/Instructions.module.css";

const items = [
  {
    title: "Android",
    desc: "Скачайте клиентское приложение, добавьте профиль через «Добавить профиль».",
  },
  { title: "iOS", desc: "Отсканируйте QR или импортируйте ссылку в приложение." },
  { title: "Windows", desc: "Добавьте профиль через импорт конфигурации в клиентском ПО." },
  { title: "macOS / Linux", desc: "Импортируйте подписку через клиентское приложение." },
];

const recommended = "Рекомендуемое приложение — Happ: универсальный клиент для всех платформ.";

export default function Instructions() {
  return (
    <div className={`section ${styles.sectionFlush}`}>
      <div className={styles.grid}>
        {items.map((it) => (
          <div key={it.title} className={styles.item}>
            <h3>{it.title}</h3>
            <p>{it.desc}</p>
          </div>
        ))}
      </div>

      <p className={styles.recommended}>{recommended}</p>
    </div>
  );
}
