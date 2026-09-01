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

	useEffect(() => {
		// Check if Telegram is linked (this would need a new API endpoint or include in user profile)
		// For now, we'll just show the bind option
	}, []);

	const handleGenerateBindCode = async () => {
		setLoadingBind(true);
		try {
			const res = await fetch("/api/codes/bind", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: `Bearer ${localStorage.getItem("access_token") || ""}`,
				},
			});
			const data = await res.json();
			if (!res.ok) {
				alert(data.error || "Не удалось сгенерировать код.");
				return;
			}
			setBindCode(data.code);
			setBotLink(data.bot_link);
		} catch {
			alert("Ошибка сети. Попробуйте позже.");
		} finally {
			setLoadingBind(false);
		}
	};

  const handleExport = async () => {
    setExporting(true);
    try {
      const res = await fetch("/api/user/data-export", {
        headers: {
          Authorization: `Bearer ${localStorage.getItem("access_token") || ""}`,
        },
      });
      if (!res.ok) throw new Error("export failed");
      const data = await res.json();
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `nomoreblocks-data-${Date.now()}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      alert("Не удалось экспортировать данные. Попробуйте позже.");
    } finally {
      setExporting(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      const res = await fetch("/api/user", {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${localStorage.getItem("access_token") || ""}`,
        },
      });
      if (!res.ok) throw new Error("delete failed");
      await logout();
      navigate("/");
    } catch {
      alert("Не удалось удалить аккаунт. Попробуйте позже.");
      setDeleting(false);
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.grid}>
		<div className={styles.card}>
          <h3 className={styles.title}>Аккаунт</h3>
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
          <h3 className={styles.title}>🔗 Привязка Telegram</h3>
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

        <div className={styles.card}>
          <h3 className={styles.title}>Персональные данные (152-ФЗ)</h3>
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
