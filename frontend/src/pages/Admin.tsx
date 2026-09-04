import { useEffect, useState } from "react";
import styles from "@styles/Admin.module.css";
import { adminApi, type PlanInput, type DiscountInput } from "@api/admin";

type Plan = PlanInput & { id: string };
type Discount = DiscountInput & { id: string };

export default function Admin() {
  const [authed, setAuthed] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loginError, setLoginError] = useState("");
  const [plans, setPlans] = useState<Plan[]>([]);
  const [discounts, setDiscounts] = useState<Discount[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [editingPlanId, setEditingPlanId] = useState<string | null>(null);
  const [editingDiscountId, setEditingDiscountId] = useState<string | null>(null);

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

  const notify = (msg: string, kind: "error" | "success" = "success") => {
    setError(kind === "error" ? msg : "");
    setSuccess(kind === "success" ? msg : "");
    setTimeout(() => {
      setError("");
      setSuccess("");
    }, 4000);
  };

  const refresh = async () => {
    setLoading(true);
    try {
      const [plansRes, discountsRes] = await Promise.all([
        adminApi.get<{ plans: Plan[] }>("/plans"),
        adminApi.get<{ discounts: Discount[] }>("/discounts"),
      ]);
      setPlans(plansRes.data.plans ?? []);
      setDiscounts(discountsRes.data.discounts ?? []);
    } catch (e: any) {
      notify(e?.response?.data?.error || "Не удалось загрузить данные", "error");
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
      await adminApi.post("/login", { username, password });
      setAuthed(true);
    } catch (e: any) {
      setLoginError(e?.response?.data?.error || "Ошибка входа");
    }
  };

  const handleLogout = async () => {
    await adminApi.post("/logout");
    setAuthed(false);
    setUsername("");
    setPassword("");
    setEditingPlanId(null);
    setEditingDiscountId(null);
  };

  const startEditPlan = (p: Plan) => {
    setEditingPlanId(p.id);
    setPlanForm({
      id: p.id,
      name: p.name,
      duration_days: p.duration_days,
      price_minor: p.price_minor,
      currency: p.currency,
      group_name: p.group_name,
    });
  };

  const resetPlanForm = () => {
    setEditingPlanId(null);
    setPlanForm({ id: "", name: "", duration_days: 30, price_minor: 0, currency: "RUB", group_name: "Free" });
  };

  const savePlan = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      if (editingPlanId) {
        await adminApi.put(`/plans/${editingPlanId}`, planForm);
        notify("Тариф обновлён");
      } else {
        const payload = { ...planForm, id: planForm.id || `plan_${Date.now()}` };
        await adminApi.post("/plans", payload);
        notify("Тариф создан");
        setPlanForm({ id: "", name: "", duration_days: 30, price_minor: 0, currency: "RUB", group_name: "Free" });
      }
      await refresh();
      resetPlanForm();
    } catch (e: any) {
      notify(e?.response?.data?.error || "Не удалось сохранить тариф", "error");
    } finally {
      setSaving(false);
    }
  };

  const removePlan = async (id: string) => {
    if (!confirm("Удалить тариф? Это может сломать оплату, если он уже используется.")) return;
    try {
      await adminApi.delete(`/plans/${id}`);
      notify("Тариф удалён");
      if (editingPlanId === id) resetPlanForm();
      await refresh();
    } catch (e: any) {
      notify(e?.response?.data?.error || "Не удалось удалить тариф", "error");
    }
  };

  const startEditDiscount = (d: Discount) => {
    setEditingDiscountId(d.id);
    const dt = (v: string) => (v ? v.slice(0, 16) : "");
    setDiscountForm({
      id: d.id,
      code: d.code,
      plan_id: d.plan_id,
      percent: d.percent,
      fixed_minor: d.fixed_minor,
      starts_at: dt(d.starts_at),
      expires_at: dt(d.expires_at),
      max_uses: d.max_uses,
      is_active: d.is_active,
    });
  };

  const resetDiscountForm = () => {
    setEditingDiscountId(null);
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
  };

  const saveDiscount = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      if (editingDiscountId) {
        await adminApi.put(`/discounts/${editingDiscountId}`, discountForm);
        notify("Скидка обновлена");
      } else {
        const payload = { ...discountForm, id: discountForm.id || `discount_${Date.now()}` };
        await adminApi.post("/discounts", payload);
        notify("Скидка создана");
        resetDiscountForm();
      }
      await refresh();
      resetDiscountForm();
    } catch (e: any) {
      notify(e?.response?.data?.error || "Не удалось сохранить скидку", "error");
    } finally {
      setSaving(false);
    }
  };

  const removeDiscount = async (id: string) => {
    if (!confirm("Удалить скидку?")) return;
    try {
      await adminApi.delete(`/discounts/${id}`);
      notify("Скидка удалена");
      if (editingDiscountId === id) resetDiscountForm();
      await refresh();
    } catch (e: any) {
      notify(e?.response?.data?.error || "Не удалось удалить скидку", "error");
    }
  };

  const fmtPrice = (v: number) => `${(v / 100).toFixed(2)} ₽`;
  const fmtDate = (v: string) => (v ? new Date(v).toLocaleString("ru-RU") : "—");

  if (!authed) {
    return (
      <div className={`container ${styles.wrap}`}>
        <div className={`container ${styles.wrap}`}>
          <form className={styles.card} onSubmit={handleLogin}>
            <h1>Admin</h1>
            <p className={styles.hint}>Введите учётные данные администратора.</p>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Username"
              autoComplete="username"
            />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Password"
              autoComplete="current-password"
            />
            {loginError && <p className={styles.error}>{loginError}</p>}
            <button type="submit" disabled={!username || !password}>
              Войти
            </button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.toolbar}>
        <div>
          <h1>Admin</h1>
          <p className={styles.hint}>Управление тарифами и скидками.</p>
        </div>
        <button onClick={handleLogout}>Выйти</button>
      </div>

      {error && <div className={styles.toastError}>{error}</div>}
      {success && <div className={styles.toastSuccess}>{success}</div>}
      {loading && <p className={styles.hint}>Загрузка…</p>}

      <section className={styles.card}>
        <div className={styles.sectionHeader}>
          <h2>Тарифы</h2>
          <span className={styles.badge}>{plans.length}</span>
        </div>
        <form className={styles.form} onSubmit={savePlan}>
          <input
            placeholder="id (оставьте пустым для автогенерации)"
            value={planForm.id}
            onChange={(e) => setPlanForm({ ...planForm, id: e.target.value })}
          />
          <input
            placeholder="Название"
            value={planForm.name}
            onChange={(e) => setPlanForm({ ...planForm, name: e.target.value })}
            required
          />
          <input
            type="number"
            placeholder="Дней"
            value={planForm.duration_days}
            onChange={(e) => setPlanForm({ ...planForm, duration_days: Number(e.target.value) })}
            required
            min={1}
          />
          <input
            type="number"
            placeholder="Цена (копейки)"
            value={planForm.price_minor}
            onChange={(e) => setPlanForm({ ...planForm, price_minor: Number(e.target.value) })}
            required
            min={0}
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
          <div className={styles.formActions}>
            <button type="submit" disabled={saving}>
              {editingPlanId ? "Сохранить изменения" : "Создать тариф"}
            </button>
            {editingPlanId && (
              <button type="button" className={styles.secondary} onClick={resetPlanForm}>
                Отмена
              </button>
            )}
          </div>
        </form>

        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>id</th>
                <th>name</th>
                <th>days</th>
                <th>price</th>
                <th>currency</th>
                <th>group</th>
                <th className={styles.actionsCell}>actions</th>
              </tr>
            </thead>
            <tbody>
              {plans.map((p) => (
                <tr key={p.id} className={editingPlanId === p.id ? styles.rowActive : ""}>
                  <td>{p.id}</td>
                  <td>{p.name}</td>
                  <td>{p.duration_days}</td>
                  <td>{fmtPrice(p.price_minor)}</td>
                  <td>{p.currency}</td>
                  <td>{p.group_name}</td>
                  <td className={styles.actionsCell}>
                    <button onClick={() => startEditPlan(p)}>Редактировать</button>
                    <button className={styles.danger} onClick={() => removePlan(p.id)}>
                      Удалить
                    </button>
                  </td>
                </tr>
              ))}
              {!plans.length && (
                <tr>
                  <td colSpan={7} className={styles.empty}>
                    Тарифы не найдены
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className={styles.card}>
        <div className={styles.sectionHeader}>
          <h2>Скидки</h2>
          <span className={styles.badge}>{discounts.length}</span>
        </div>
        <form className={styles.form} onSubmit={saveDiscount}>
          <input
            placeholder="id (оставьте пустым для автогенерации)"
            value={discountForm.id}
            onChange={(e) => setDiscountForm({ ...discountForm, id: e.target.value })}
          />
          <input
            placeholder="Код"
            value={discountForm.code}
            onChange={(e) => setDiscountForm({ ...discountForm, code: e.target.value })}
            required
          />
          <input
            placeholder="plan_id или пусто для глобальной"
            value={discountForm.plan_id ?? ""}
            onChange={(e) => setDiscountForm({ ...discountForm, plan_id: e.target.value || null })}
          />
          <input
            type="number"
            placeholder="Процент 0-100"
            value={discountForm.percent}
            onChange={(e) => setDiscountForm({ ...discountForm, percent: Number(e.target.value) })}
            required
            min={0}
            max={100}
          />
          <input
            type="number"
            placeholder="Фикс (копейки)"
            value={discountForm.fixed_minor}
            onChange={(e) => setDiscountForm({ ...discountForm, fixed_minor: Number(e.target.value) })}
            required
            min={0}
          />
          <input
            type="datetime-local"
            value={discountForm.starts_at}
            onChange={(e) => setDiscountForm({ ...discountForm, starts_at: e.target.value })}
            required
          />
          <input
            type="datetime-local"
            value={discountForm.expires_at}
            onChange={(e) => setDiscountForm({ ...discountForm, expires_at: e.target.value })}
            required
          />
          <input
            type="number"
            placeholder="max_uses"
            value={discountForm.max_uses}
            onChange={(e) => setDiscountForm({ ...discountForm, max_uses: Number(e.target.value) })}
            required
            min={0}
          />
          <label className={styles.checkbox}>
            <input
              type="checkbox"
              checked={discountForm.is_active}
              onChange={(e) => setDiscountForm({ ...discountForm, is_active: e.target.checked })}
            />
            active
          </label>
          <div className={styles.formActions}>
            <button type="submit" disabled={saving}>
              {editingDiscountId ? "Сохранить изменения" : "Создать скидку"}
            </button>
            {editingDiscountId && (
              <button type="button" className={styles.secondary} onClick={resetDiscountForm}>
                Отмена
              </button>
            )}
          </div>
        </form>

        <div className={styles.tableWrap}>
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
                <th className={styles.actionsCell}>actions</th>
              </tr>
            </thead>
            <tbody>
              {discounts.map((d) => (
                <tr key={d.id} className={editingDiscountId === d.id ? styles.rowActive : ""}>
                  <td>{d.id}</td>
                  <td>{d.code}</td>
                  <td>{d.plan_id || "global"}</td>
                  <td>{d.percent}</td>
                  <td>{d.fixed_minor}</td>
                  <td>{fmtDate(d.starts_at)}</td>
                  <td>{fmtDate(d.expires_at)}</td>
                  <td>{d.max_uses}</td>
                  <td>{d.used_count}</td>
                  <td>{d.is_active ? "✅" : "❌"}</td>
                  <td className={styles.actionsCell}>
                    <button onClick={() => startEditDiscount(d)}>Редактировать</button>
                    <button className={styles.danger} onClick={() => removeDiscount(d.id)}>
                      Удалить
                    </button>
                  </td>
                </tr>
              ))}
              {!discounts.length && (
                <tr>
                  <td colSpan={11} className={styles.empty}>
                    Скидки не найдены
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
