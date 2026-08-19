import styles from "./Instructions.module.css";

const items = [
  {
    title: "Android",
    desc: "Скачайте клиентское приложение, добавьте профиль через «Добавить профиль».",
  },
  { title: "iOS", desc: "Отсканируйте QR или импортируйте ссылку в приложение." },
  { title: "Windows", desc: "Добавьте профиль через импорт конфигурации в клиентском ПО." },
  { title: "macOS / Linux", desc: "Импортируйте подписку через клиентское приложение." },
];

export default function Instructions() {
  return (
    <div className="section" style={{ padding: 0 }}>
      <div className={styles.grid}>
        {items.map((it) => (
          <div key={it.title} className={styles.item}>
            <h3>{it.title}</h3>
            <p>{it.desc}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
