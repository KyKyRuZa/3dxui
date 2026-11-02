import {
  IconArrowsExchange,
  IconDevices,
  IconShieldCheck,
  IconWorld,
} from "@tabler/icons-react";
import styles from "@styles/Features.module.css";

const items = [
  {
    icon: IconShieldCheck,
    number: "01",
    title: "Шифрование трафика",
    desc: "Передача данных выполняется по защищённому каналу — в общественных и домашних сетях.",
  },
  {
    icon: IconWorld,
    number: "02",
    title: "Глобальная сеть узлов",
    desc: "Выбирайте подходящую локацию подключения: Германия или США.",
  },
  {
    icon: IconArrowsExchange,
    number: "03",
    title: "Смена IP-адреса",
    desc: "Внешний адрес определяется выбранным сервером, а не вашей локальной сетью.",
  },
  {
    icon: IconDevices,
    number: "04",
    title: "Все устройства",
    desc: "Одна подписка для ноутбука, смартфона и других совместимых устройств.",
  },
];

export default function Features() {
  return (
    <section className={`section ${styles.section}`} id="features">
      <div className="container">
        <div className={styles.heading}>
          <div className={styles.kicker}>ПРИНЦИП РАБОТЫ</div>
          <h2>Технологии, которые работают незаметно</h2>
          <p>Нужные инструменты приватного подключения — без сложной настройки и лишних обещаний.</p>
        </div>
        <div className={styles.grid}>
          {items.map((item) => {
            const Icon = item.icon;
            return (
              <article key={item.number} className={styles.item}>
                <div className={styles.itemTop}>
                  <span className={styles.icon}><Icon size={21} stroke={1.8} /></span>
                  <span className={styles.number}>{item.number}</span>
                </div>
                <h3 className={styles.title}>{item.title}</h3>
                <p className={styles.desc}>{item.desc}</p>
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
