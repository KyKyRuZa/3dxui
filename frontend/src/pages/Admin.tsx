import { useEffect, useState } from "react";
import styles from "@styles/Admin.module.css";
import { adminApi, type PlanInput, type DiscountInput } from "@api/admin";

type Plan = PlanInput & { id: string };
type Discount = DiscountInput & { id: string };

export default function Admin() {
  const [authed, setAuthed] = useState(false);
  const [secret, setSecret] = useState("");
  const [loginError, setLoginError] = useState("");
  const [plans, setPlans] = useState<Plan[]>([]);
  const [discounts, setDiscounts] = useState<Discount[]>([]);
  const [loading, setLoading] = useState(false);
  const [planForm, setPlanForm] = useState<PlanInput>({
    id: "",
    name: "",
    duration_days: 30,
    price_minor: 0,
    currency: "RUB",
    group_name: "Free",
  });
  const [discountForm, setDiscountForm] = useState<DiscountInput>({
    id: "",
    code: "",
    plan_id: null,
    percent: 0,
    fixed_minor: 0,
    starts_at: "",
    expires_at: "",
    max_uses: 0,
    is_active: true,
  });

  const refresh = async () => {
    setLoading(true);
    try {
      const [plansRes, discountsRes] = await Promise.all([
        adminApi.get<{ plans: Plan[] }>("/plans"),
        adminApi.get<{ discounts: Discount[] }>("/discounts"),
      ]);
      setPlans(plansRes.data.plans ?? []);
      setDiscounts(discountsRes.data.discounts ?? []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!authed) return;
    refresh();
  }, [authed]);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoginError("");
    try {
      await adminApi.post("/login", { secret });
      setAuthed(true);
    } catch (e: any) {
      setLoginError(e?.response?.data?.error || "Ошибка входа");
    }
  };

  const handleLogout = async () => {
    await adminApi.post("/logout");
    setAuthed(false);
    setSecret("");
  };

  const savePlan = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = { ...planForm, id: planForm.id || `plan_${Date.now()}` };
    if (payload.id && plans.some((p) => p.id === payload.id)) {
      await adminApi.put(`/plans/${payload.id}`, payload);
    } else {
      await adminApi.post("/plans", payload);
      setPlanForm({ id: "", name: "", duration_days: 30, price_minor: 0, currency: "RUB", group_name: "Free" });
    }
    await refresh();
  };

  const saveDiscount = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = { ...discountForm, id: discountForm.id || `discount_${Date.now()}` };
    if (payload.id && discounts.some((d) => d.id === payload.id)) {
      await adminApi.put(`/discounts/${payload.id}`, payload);
    } else {
      await adminApi.post("/discounts", payload);
      setDiscountForm({
        id: "",
        code: "",
        plan_id: null,
        percent: 0,
        fixed_minor: 0,
        starts_at: "",
        expires_at: "",
        max_uses: 0,
        is_active: true,
      });
    }
    await refresh();
  };

  const removePlan = async (id: string) => {
    if (!confirm("Удалить тариф?")) return;
    await adminApi.delete(`/plans/${id}`);
    await refresh();
  };

  const removeDiscount = async (id: string) => {
    if (!confirm("Удалить скидку?")) return;
    await adminApi.delete(`/discounts/${id}`);
    await refresh();
  };

  const fmtPrice = (v: number) => `${(v / 100).toFixed(2)} ₽`;

  if (!authed) {
    return (
      <div className={`container ${styles.wrap}`}>
        <form className={styles.card} onSubmit={handleLogin}>
          <h1>Admin</h1>
          <p className={styles.hint}>Введите секрет доступа к админке.</p>
          <input
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            placeholder="Admin secret"
          />
          {loginError && <p className={styles.error}>{loginError}</p>}
          <button type="submit">Войти</button>
        </form>
      </div>
    );
  }

  return (
    <div className={`container ${styles.wrap}`}>
      <div className={styles.toolbar}>
        <h1>Admin</h1>
        <button onClick={handleLogout}>Выйти</button>
      </div>
      {loading && <p className={styles.hint}>Загрузка…</p>}

      <section className={styles.card}>
        <h2>Тарифы</h2>
        <form className={styles.form} onSubmit={savePlan}>
          <input
            placeholder="id"
            value={planForm.id}
            onChange={(e) => setPlanForm({ ...planForm, id: e.target.value })}
          />
          <input
            placeholder="Название"
            value={planForm.name}
            onChange={(e) => setPlanForm({ ...planForm, name: e.target.value })}
          />
          <input
            type="number"
            placeholder="Дней"
            value={planForm.duration_days}
            onChange={(e) => setPlanForm({ ...planForm, duration_days: Number(e.target.value) })}
          />
          <input
            type="number"
            placeholder="Цена (копейки)"
            value={planForm.price_minor}
            onChange={(e) => setPlanForm({ ...planForm, price_minor: Number(e.target.value) })}
          />
          <input
            placeholder="Валюта"
            value={planForm.currency}
            onChange={(e) => setPlanForm({ ...planForm, currency: e.target.value })}
          />
          <input
            placeholder="Группа"
            value={planForm.group_name}
            onChange={(e) => setPlanForm({ ...planForm, group_name: e.target.value })}
          />
          <button type="submit">Сохранить тариф</button>
        </form>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>id</th>
              <th>name</th>
              <th>days</th>
              <th>price</th>
              <th>currency</th>
              <th>group</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {plans.map((p) => (
              <tr key={p.id}>
                <td>{p.id}</td>
                <td>{p.name}</td>
                <td>{p.duration_days}</td>
                <td>{fmtPrice(p.price_minor)}</td>
                <td>{p.currency}</td>
                <td>{p.group_name}</td>
                <td>
                  <button onClick={() => removePlan(p.id)}>Удалить</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className={styles.card}>
        <h2>Скидки</h2>
        <form className={styles.form} onSubmit={saveDiscount}>
          <input
            placeholder="id"
            value={discountForm.id}
            onChange={(e) => setDiscountForm({ ...discountForm, id: e.target.value })}
          />
          <input
            placeholder="Код"
            value={discountForm.code}
            onChange={(e) => setDiscountForm({ ...discountForm, code: e.target.value })}
          />
          <input
            placeholder="plan_id или пусто"
            value={discountForm.plan_id ?? ""}
            onChange={(e) => setDiscountForm({ ...discountForm, plan_id: e.target.value || null })}
          />
          <input
            type="number"
            placeholder="Процент"
            value={discountForm.percent}
            onChange={(e) => setDiscountForm({ ...discountForm, percent: Number(e.target.value) })}
          />
          <input
            type="number"
            placeholder="Фикс (копейки)"
            value={discountForm.fixed_minor}
            onChange={(e) => setDiscountForm({ ...discountForm, fixed_minor: Number(e.target.value) })}
          />
          <input
            type="datetime-local"
            value={discountForm.starts_at}
            onChange={(e) => setDiscountForm({ ...discountForm, starts_at: e.target.value })}
          />
          <input
            type="datetime-local"
            value={discountForm.expires_at}
            onChange={(e) => setDiscountForm({ ...discountForm, expires_at: e.target.value })}
          />
          <input
            type="number"
            placeholder="max_uses"
            value={discountForm.max_uses}
            onChange={(e) => setDiscountForm({ ...discountForm, max_uses: Number(e.target.value) })}
          />
          <label className={styles.checkbox}>
            <input
              type="checkbox"
              checked={discountForm.is_active}
              onChange={(e) => setDiscountForm({ ...discountForm, is_active: e.target.checked })}
            />
            active
          </label>
          <button type="submit">Сохранить скидку</button>
        </form>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>id</th>
              <th>code</th>
              <th>plan_id</th>
              <th>%</th>
              <th>fixed</th>
              <th>starts</th>
              <th>expires</th>
              <th>max</th>
              <th>used</th>
              <th>active</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {discounts.map((d) => (
              <tr key={d.id}>
                <td>{d.id}</td>
                <td>{d.code}</td>
                <td>{d.plan_id}</td>
                <td>{d.percent}</td>
                <td>{d.fixed_minor}</td>
                <td>{new Date(d.starts_at).toLocaleString()}</td>
                <td>{new Date(d.expires_at).toLocaleString()}</td>
                <td>{d.max_uses}</td>
                <td>{d.used_count}</td>
                <td>{d.is_active ? "✅" : "❌"}</td>
                <td>
                  <button onClick={() => removeDiscount(d.id)}>Удалить</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
