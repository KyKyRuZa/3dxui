import styles from "./Features.module.css";

const items = [
  {
    title: "Надёжное шифрование",
    desc: "Передача данных по защищённому каналу с маскировкой под обычный HTTPS.",
  },
  {
    title: "Минимальная задержка",
    desc: "Оптимизированные серверы в Европе и Азии для быстрой работы.",
  },
  { title: "Конфиденциальность", desc: "Мы не храним логи и не передаём данные третьим лицам." },
  { title: "Техническая поддержка 24/7", desc: "Поможем с настройкой на любом устройстве." },
];

export default function Features() {
  return (
    <section className="section">
      <div className="container">
        <div className={styles.grid}>
          {items.map((it) => (
            <div key={it.title} className={styles.item}>
              <div className={styles.icon}>◆</div>
              <div className={styles.title}>{it.title}</div>
              <div className={styles.desc}>{it.desc}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
