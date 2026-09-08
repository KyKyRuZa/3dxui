import { useState, useEffect } from "react";
import { useAuth } from "@hooks/useAuth";
import { useNavigate } from "react-router-dom";
import styles from "@styles/Settings.module.css";

export default function Settings() {
	const { user, logout } = useAuth();
	const navigate = useNavigate();
	const [exporting, setExporting] = useState(false);
	const [deleting, setDeleting] = useState(false);
	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
	const [bindCode, setBindCode] = useState<string | null>(null);
	const [botLink, setBotLink] = useState<string | null>(null);
	const [loadingBind, setLoadingBind] = useState(false);
	const [success, setSuccess] = useState("");

	useEffect(() => {
		if (!success) return;
		const t = setTimeout(() => setSuccess(""), 3000);
		return () => clearTimeout(t);
	}, [success]);

	const getAuthHeader = () => ({
		Authorization: `Bearer ${localStorage.getItem("access_token") || ""}`,
	});

  const handleGenerateBindCode = async () => {
    setLoadingBind(true);
    setBindCode(null);
    setBotLink(null);
    try {
      const res = await fetch("/api/codes/bind", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...getAuthHeader(),
        },
      });
      const data = await res.json();
      if (!res.ok) {
        setSuccess(data.error || "Не удалось сгенерировать код.");
        return;
      }
      setBindCode(data.code);
      setBotLink(data.bot_link);
    } catch {
      setSuccess("Ошибка сети. Попробуйте позже.");
    } finally {
      setLoadingBind(false);
    }
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      const res = await fetch("/api/user/data-export", {
        headers: getAuthHeader(),
      });
      if (!res.ok) throw new Error("export failed");
      const data = await res.json();
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `walyny4-vpn-data-${Date.now()}.json`;
      a.click();
      URL.revokeObjectURL(url);
      setSuccess("Данные экспортированы");
    } catch {
      setSuccess("Не удалось экспортировать данные.");
    } finally {
      setExporting(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      const res = await fetch("/api/user", {
        method: "DELETE",
        headers: getAuthHeader(),
      });
      if (!res.ok) throw new Error("delete failed");
      await logout();
      navigate("/");
    } catch {
      setSuccess("Не удалось удалить аккаунт.");
      setDeleting(false);
    }
  };

  return (
    <div className={styles.page}>
      {success && <div className={styles.success}>{success}</div>}

      <div className={styles.grid}>
        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <div className={styles.cardIcon}>👤</div>
            <div className={styles.cardTitle}>Аккаунт</div>
          </div>
          <div className={styles.form}>
            <div className={styles.field}>
              <label className={styles.label}>Пользователь</label>
              <input className={styles.input} value={user?.username ?? ""} readOnly />
            </div>
            <p className={styles.hint}>
              Вход и регистрация выполняются через Telegram или по паролю.
            </p>
          </div>
        </div>

        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <div className={styles.cardIcon}>🔗</div>
            <div className={styles.cardTitle}>Привязка Telegram</div>
          </div>
          <div className={styles.form}>
            <p className={styles.hint}>
              Привяжите Telegram для быстрого входа через бота и получения уведомлений.
            </p>
            {!bindCode ? (
              <div className={styles.actions}>
                <button
                  className={styles.tgButton}
                  onClick={handleGenerateBindCode}
                  disabled={loadingBind}
                >
                  {loadingBind ? "Генерация..." : "Привязать Telegram"}
                </button>
              </div>
            ) : (
              <div className={styles.bindCode}>
                <p className={styles.hint}>
                  Перейдите в бота и отправьте команду:
                </p>
                <code className={styles.codeBox}>/start bind-{bindCode}</code>
                {botLink && (
                  <a
                    className={styles.tgLink}
                    href={botLink}
                    target="_blank"
                    rel="noreferrer"
                  >
                    Открыть бота
                  </a>
                )}
                <button
                  className={styles.btnSecondary}
                  onClick={() => { setBindCode(null); setBotLink(null); }}
                >
                  Закрыть
                </button>
              </div>
            )}
          </div>
        </div>

        <div className={`${styles.card} ${styles.spanFull}`}>
          <div className={styles.cardHeader}>
            <div className={styles.cardIcon}>🛡️</div>
            <div className={styles.cardTitle}>Персональные данные (152-ФЗ)</div>
          </div>
          <div className={styles.form}>
            <p className={styles.hint}>
              Вы имеете право на доступ к своим персональным данным и их удаление
              в соответствии с Федеральным законом № 152-ФЗ «О персональных данных».
            </p>
            <div className={styles.actions}>
              <button
                className={styles.btnSecondary}
                onClick={handleExport}
                disabled={exporting}
              >
                {exporting ? "Экспорт..." : "Экспортировать данные"}
              </button>
              {!showDeleteConfirm ? (
                <button
                  className={styles.btnDanger}
                  onClick={() => setShowDeleteConfirm(true)}
                >
                  Удалить аккаунт
                </button>
              ) : (
                <div className={styles.confirm}>
                  <p>Вы уверены? Это действие необратимо.</p>
                  <div className={styles.confirmActions}>
                    <button
                      className={styles.btnDanger}
                      onClick={handleDelete}
                      disabled={deleting}
                    >
                      {deleting ? "Удаление..." : "Да, удалить"}
                    </button>
                    <button
                      className={styles.btnSecondary}
                      onClick={() => setShowDeleteConfirm(false)}
                    >
                      Отмена
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
