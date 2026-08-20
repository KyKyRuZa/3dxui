import PricingCards from "@components/PricingCards";
import styles from "@styles/global.module.css";

export default function PricingPage() {
  return (
    <div className={styles.root}>
      <div className="container section">
        <h1 className={styles.pageTitle}>Тарифы</h1>
        <p className={styles.pageSubtitle}>
          Выберите подписку, которая подходит вам.
        </p>
      </div>
      <PricingCards />
    </div>
  );
}
