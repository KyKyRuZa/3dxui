import { Link } from "react-router-dom";
import { Button } from "./Button";
import styles from "./Hero.module.css";

export default function Hero() {
  return (
    <section className={styles.hero}>
      <div className="container">
        <h1 className={styles.title}>
          Защита ваших данных
          <br />в любой сети
        </h1>
        <p className={styles.subtitle}>
          Шифрование трафика, приватность и стабильность. Безопасный доступ к вашим ресурсам из
          любой точки.
        </p>
        <div className={styles.actions}>
          <Link to="/register">
            <Button>Начать использование</Button>
          </Link>
          <Link to="/pricing">
            <Button variant="secondary">Узнать тарифы</Button>
          </Link>
        </div>
      </div>
    </section>
  );
}
