import { useEffect, useState } from "react";
import { IconArrowUpRight, IconCheck, IconCreditCard } from "@tabler/icons-react";
import { useNavigate } from "react-router-dom";
import { Button } from "@components/Button";
import { createPayment, getPlans, type Plan } from "@api/billing";
import { resetSubscriptionCache } from "@api/subscription";
import styles from "@styles/PricingCards.module.css";

function formatPriceValue(plan: Plan): string {
  return new Intl.NumberFormat("en-US", {
    maximumFractionDigits: 2,
  }).format(plan.price_minor / 100);
}

export default function PricingCards() {
  const navigate = useNavigate();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [paymentUrl, setPaymentUrl] = useState("");

  useEffect(() => {
    let cancelled = false;
    getPlans()
      .then((data) => { if (!cancelled) setPlans(data); })
      .catch(() => { if (!cancelled) setError("Не удалось загрузить тарифы. Попробуйте обновить страницу."); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  async function handleBuy(plan: Plan) {
    setBusyId(plan.id);
    setError("");
    setInfo("");
    setPaymentUrl("");
    try {
      const payment = await createPayment(plan.id);
      resetSubscriptionCache();
      setPaymentUrl(payment.confirmation_url);
      window.open(payment.confirmation_url, "_blank", "noopener,noreferrer");
      setInfo("Ссылка на оплату подготовлена. После подтверждения доступ обновится в личном кабинете.");
    } catch (e: unknown) {
      const status = (e as { response?: { status?: number } })?.response?.status;
      if (status === 401) {
        navigate("/register");
        return;
      }
      setError("Не удалось подготовить оплату. Попробуйте позже.");
    } finally {
      setBusyId(null);
    }
  }

  if (loading) {
    return <section className={styles.section}><div className="container"><div className={styles.loading}>Загружаем варианты подписки…</div></div></section>;
  }

  return (
    <section className={styles.section}>
      <div className="container">
        {error && <div className={styles.messageError} role="alert">{error}</div>}
        {info && <div className={styles.messageInfo} role="status">{info} {paymentUrl && <a href={paymentUrl} target="_blank" rel="noreferrer">Открыть страницу оплаты</a>}</div>}
        {plans.length === 0 && !error ? (
          <div className={styles.empty}>Сейчас нет доступных тарифов. Вернитесь немного позже.</div>
        ) : (
          <div className={styles.list}>
            {plans.map((plan, index) => (
              <article key={plan.id} className={styles.planRow}>
                <div className={styles.planIdentity}>
                  <span className={styles.planLabel}>0{index + 1} · ПЛАН ДОСТУПА</span>
                  <h3 className={styles.name}>{plan.name}</h3>
                  <span className={styles.duration}>{plan.duration_days} дней подключения</span>
                </div>
                <div className={styles.pricingInfo}>
                  <div className={styles.price}>{formatPriceValue(plan)}<span>за период</span></div>
                  <div className={styles.priceNote}>Валюта и итоговые условия — на странице подтверждения оплаты.</div>
                </div>
                <ul className={styles.perks}>
                  <li><IconCheck size={15} /> {plan.duration_days} дней доступа</li>
                  <li><IconCheck size={15} /> Локации DE и US</li>
                  <li><IconCheck size={15} /> Совместимые устройства</li>
                </ul>
                <div className={styles.cta}>
                  <Button block loading={busyId === plan.id} onClick={() => handleBuy(plan)}>
                    Выбрать тариф <IconArrowUpRight size={16} />
                  </Button>
                </div>
              </article>
            ))}
          </div>
        )}
        <div className={styles.secureNote}><IconCreditCard size={16} /> Оплата проходит на защищённой странице платёжного оператора.</div>
      </div>
    </section>
  );
}
