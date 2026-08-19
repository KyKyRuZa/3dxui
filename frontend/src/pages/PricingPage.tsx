import PricingCards from "../components/PricingCards";
import styles from "../styles/global.module.css";

export default function PricingPage() {
  return (
    <div className={styles.root}>
      <div className="container section">
        <h1 style={{ fontSize: 32, fontWeight: 800 }}>Тарифы</h1>
        <p style={{ color: "var(--color-muted)", marginTop: 8 }}>
          Выберите подписку, которая подходит вам.
        </p>
      </div>
      <PricingCards />
    </div>
  );
}
