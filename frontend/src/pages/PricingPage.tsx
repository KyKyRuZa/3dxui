import { Link } from "react-router-dom";
import { IconCheck } from "@tabler/icons-react";
import PricingCards from "@components/PricingCards";
import styles from "@styles/global.module.css";
import pageStyles from "@styles/PricingPage.module.css";

export default function PricingPage() {
  return (
    <div className={pageStyles.page}>
      <div className={`container ${pageStyles.header}`}>
        <div className={pageStyles.kicker}>ПОДПИСКА WALYNY4</div>
        <h1 className={styles.pageTitle}>Понятные условия доступа</h1>
        <p className={styles.pageSubtitle}>Выберите срок подписки, который подходит вашему ритму использования.</p>
        <div className={pageStyles.trial}><IconCheck size={17} /> 3 дня пробного доступа для новых аккаунтов <Link to="/register">Создать аккаунт</Link></div>
      </div>
      <PricingCards />
    </div>
  );
}
