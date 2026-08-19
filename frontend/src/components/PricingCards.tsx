import { Link } from "react-router-dom";
import { Button } from "./Button";
import styles from "./PricingCards.module.css";

const plans = [
  {
    name: "1 месяц",
    price: "149 ₽",
    period: "/мес",
    perks: ["1 устройство", "Все серверы", "Поддержка"],
  },
  {
    name: "3 месяца",
    price: "349 ₽",
    period: "/3 мес",
    perks: ["3 устройства", "Все серверы", "Приоритетная поддержка"],
    highlighted: true,
  },
  {
    name: "6 месяцев",
    price: "599 ₽",
    period: "/6 мес",
    perks: ["6 устройств", "Все серверы", "Приоритетная поддержка"],
  },
  {
    name: "12 месяцев",
    price: "999 ₽",
    period: "/год",
    perks: ["10 устройств", "Все серверы", "VIP-поддержка"],
  },
];

export default function PricingCards() {
  return (
    <section className="section">
      <div className="container">
        <div className={styles.list}>
          {plans.map((p) => (
            <div
              key={p.name}
              className={styles.card}
              style={p.highlighted ? { borderColor: "var(--color-primary)" } : undefined}
            >
              <div className={styles.name}>{p.name}</div>
              <div className={styles.price}>
                {p.price}
                <span>{p.period}</span>
              </div>
              <ul className={styles.perks}>
                {p.perks.map((perk) => (
                  <li key={perk}>✓ {perk}</li>
                ))}
              </ul>
              <div className={styles.cta}>
                <Link to="/register">
                  <Button variant={p.highlighted ? "primary" : "secondary"} block>
                    Выбрать
                  </Button>
                </Link>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
