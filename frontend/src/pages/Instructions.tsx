import { IconBrandApple, IconBrandWindows, IconBrandUbuntu, IconBrandAndroid, IconStar } from "@tabler/icons-react";
import styles from "@styles/Instructions.module.css";

const platforms = [
  {
    icon: <IconBrandAndroid size={24} />,
    title: "Android",
    steps: [
      "Установите Happ из Google Play.",
      "Откройте Happ → вкладка «Профили» → «+».",
      "Отсканируйте QR со страницы «Подписка» или вставьте VLESS-ссылку.",
      "Нажмите «Подключить» — появится статус «Активен».",
    ],
  },
  {
    icon: <IconBrandApple size={24} />,
    title: "iOS / iPadOS",
    steps: [
      "Установите Happ из App Store.",
      "Откройте Happ → вкладка «Подписки» → «Добавить».",
      "Отсканируйте QR-код со страницы «Подписка» или вставьте ссылку подписки.",
      "Подключите профиль — появится зелёная галочка.",
    ],
  },
  {
    icon: <IconBrandWindows size={24} />,
    title: "Windows",
    steps: [
      "Скачайте Happ для Windows и установите.",
      "Откройте Happ → «Профили» → «Импорт».",
      "Отсканируйте QR-код или вставьте VLESS-ссылку со страницы «Подписка».",
      "Двойной клик по профилю → «Подключиться».",
    ],
  },
  {
    icon: <IconBrandUbuntu size={24} />,
    title: "macOS / Linux",
    steps: [
      "Скачайте Happ для вашей ОС и установите.",
      "Запустите Happ → «Добавить профиль».",
      "Импортируйте через QR или ссылку подписки.",
      "Включите профиль — статус «Активна».",
    ],
  },
];

const recommended = "Для работы используется только Happ — универсальный клиент VLESS. Скачайте его на нужное устройство, импортируйте профиль и нажмите «Подключиться». Если приложение не открывается автоматически после установки — проверьте права в настройках системы.";

export default function Instructions() {
  return (
    <div className={`section ${styles.sectionFlush}`}>
      <div className={styles.grid}>
        {platforms.map((it) => (
          <div key={it.title} className={styles.item}>
            <div className={styles.itemHeader}>
              <div className={styles.itemIcon}>{it.icon}</div>
              <div className={styles.itemTitle}>{it.title}</div>
            </div>
            <ol className={styles.itemSteps}>
              {it.steps.map((s, i) => (
                <li key={i}>{s}</li>
              ))}
            </ol>
          </div>
        ))}
      </div>

      <div className={styles.recommended}>
        <span className={styles.recommendedIcon}><IconStar size={24} /></span>
        {recommended}
      </div>
    </div>
  );
}
