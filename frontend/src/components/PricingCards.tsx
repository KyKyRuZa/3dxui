import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@components/Button";
import { createPayment, getPlans, type Plan } from "@api/billing";
import { getPublicConfig } from "@api/config";
import { resetSubscriptionCache } from "@api/subscription";
import styles from "@styles/PricingCards.module.css";

function formatPrice(plan: Plan): string {
  const rub = plan.price_minor / 100;
  const formatted = rub.toLocaleString("ru-RU", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  return `${formatted} ₽`;
}

function planPerks(plan: Plan): string[] {
  const perks = [`${plan.duration_days} дней`];
  if (plan.group_name) perks.push(`Группа: ${plan.group_name}`);
  return perks;
}

export default function PricingCards() {
  const navigate = useNavigate();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [testMode, setTestMode] = useState(false);

  useEffect(() => {
    getPlans()
      .then(setPlans)
      .catch(() => setError("Не удалось загрузить тарифы"))
      .finally(() => setLoading(false));
    getPublicConfig()
      .then((cfg) => setTestMode(cfg.yookassa_test_mode))
      .catch(() => {});
  }, []);

  async function handleBuy(plan: Plan) {
    setBusyId(plan.id);
    setError("");
    setInfo("");
    try {
      const payment = await createPayment(plan.id);
      resetSubscriptionCache();
      window.open(payment.confirmation_url, "_blank", "noopener");
      setInfo(
        "Открыта страница оплаты ЮKassa. После успешной оплаты подписка продлится автоматически — обновите страницу подписки.",
      );
    } catch (e: any) {
      if (e?.response?.status === 401) {
        navigate("/register");
        return;
      }
      setError("Не удалось создать платёж. Попробуйте позже.");
    } finally {
      setBusyId(null);
    }
  }

  if (loading) {
    return (
      <section className="section">
        <div className="container">
          <div className={styles.list}>
            <div className={styles.card}>Загрузка тарифов…</div>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="section">
      <div className="container">
        {error && <div className={styles.error}>{error}</div>}
        {info && <div className={styles.info}>{info}</div>}
        {testMode && (
          <div className={styles.testMode}>
            🧪 Тестовый режим оплаты (ЮKassa): для проверки используйте тестовую
            карту, реальное списание не произойдёт.
          </div>
        )}
        <div className={styles.list}>
          {plans.map((p) => (
            <div key={p.id} className={styles.card}>
              <div className={styles.name}>{p.name}</div>
              <div className={styles.price}>
                {formatPrice(p)}
                <span>/{p.duration_days} дн.</span>
              </div>
              <ul className={styles.perks}>
                {planPerks(p).map((perk) => (
                  <li key={perk}>✓ {perk}</li>
                ))}
              </ul>
              <div className={styles.cta}>
                <Button
                  block
                  loading={busyId === p.id}
                  onClick={() => handleBuy(p)}
                >
                  Купить
                </Button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
