import { Link } from "react-router-dom";
import {
  IconArrowUpRight,
  IconCheck,
  IconChevronDown,
  IconWorld,
  IconLock,
  IconShieldCheck,
} from "@tabler/icons-react";
import Hero from "@components/Hero";
import Features from "@components/Features";
import PricingCards from "@components/PricingCards";
import styles from "@styles/LandingSections.module.css";

const questions = [
  {
    q: "Как работает защищённое подключение?",
    a: "После импорта профиля приложение устанавливает зашифрованное соединение с выбранным сервером. Внешний IP-адрес будет соответствовать выбранной локации.",
  },
  {
    q: "Какие локации доступны?",
    a: "На данный момент доступны серверные локации в Германии (DE) и США (US).",
  },
  {
    q: "На каких устройствах можно использовать подписку?",
    a: "Подписку можно добавить в совместимое приложение на смартфоне, планшете или компьютере. Инструкции по основным платформам находятся в личном кабинете.",
  },
  {
    q: "Как начать пользоваться сервисом?",
    a: "Создайте аккаунт, активируйте пробный период, затем откройте раздел «Подписка» и импортируйте профиль по QR-коду или ссылке.",
  },
];

export default function Landing() {
  return (
    <div>
      <Hero />
      <Features />

      <section className={`section ${styles.networkSection}`} id="servers">
        <div className="container">
          <div className={styles.splitHeading}>
            <div>
              <div className={styles.kicker}>ГЕОГРАФИЯ ПОДКЛЮЧЕНИЯ</div>
              <h2>Две локации. Один понятный выбор.</h2>
            </div>
            <p>Выберите серверную локацию в зависимости от задач. Смена локации доступна в совместимом клиентском приложении.</p>
          </div>
          <div className={styles.locations}>
            <article className={styles.locationCard}>
              <div className={styles.locationCode}>DE <IconWorld size={18} /></div>
              <div className={styles.locationName}>Германия</div>
              <div className={styles.locationMeta}>Европа · Germany</div>
              <div className={styles.locationBottom}><span /> Серверная локация</div>
            </article>
            <article className={styles.locationCard}>
              <div className={styles.locationCode}>US <IconWorld size={18} /></div>
              <div className={styles.locationName}>США</div>
              <div className={styles.locationMeta}>Северная Америка · United States</div>
              <div className={styles.locationBottom}><span /> Серверная локация</div>
            </article>
            <div className={styles.locationNote}>
              <IconLock size={20} />
              <div><strong>Защищённый канал связи</strong><span>Шифрование трафика при передаче между устройством и сервером.</span></div>
            </div>
          </div>
        </div>
      </section>

      <section className={`section ${styles.stepsSection}`}>
        <div className="container">
          <div className={styles.centerHeading}>
            <div className={styles.kicker}>БЫСТРЫЙ СТАРТ</div>
            <h2>Подключение за три шага</h2>
            <p>Всё нужное находится в личном кабинете — от активации доступа до готового профиля.</p>
          </div>
          <div className={styles.steps}>
            <article className={styles.step}>
              <span className={styles.stepNumber}>01</span>
              <IconShieldCheck size={22} />
              <h3>Создайте аккаунт</h3>
              <p>Зарегистрируйтесь и активируйте пробный период на 3 дня.</p>
            </article>
            <article className={styles.step}>
              <span className={styles.stepNumber}>02</span>
              <IconWorld size={22} />
              <h3>Выберите локацию</h3>
              <p>Откройте раздел подписки и получите персональную ссылку подключения.</p>
            </article>
            <article className={styles.step}>
              <span className={styles.stepNumber}>03</span>
              <IconCheck size={22} />
              <h3>Импортируйте профиль</h3>
              <p>Используйте QR-код или ссылку в совместимом приложении.</p>
            </article>
          </div>
          <div className={styles.stepCta}>
            <Link to="/register">Создать аккаунт <IconArrowUpRight size={17} /></Link>
          </div>
        </div>
      </section>

      <section className={`section ${styles.pricingSection}`} id="pricing">
        <div className="container">
          <div className={styles.centerHeading}>
            <div className={styles.kicker}>ПОДПИСКА</div>
            <h2>Начните с трёх дней</h2>
            <p>Оцените подключение и управление доступом до выбора долгосрочного тарифа.</p>
          </div>
          <div className={styles.trialBanner}>
            <div className={styles.trialIcon}><IconCheck size={20} /></div>
            <div><strong>3 дня пробного доступа</strong><span>Для знакомства с сервисом. Активация после создания аккаунта.</span></div>
            <Link to="/register">Начать <IconArrowUpRight size={16} /></Link>
          </div>
        </div>
        <PricingCards />
      </section>

      <section className={`section ${styles.faqSection}`} id="faq">
        <div className={`container ${styles.faqLayout}`}>
          <div className={styles.faqIntro}>
            <div className={styles.kicker}>FAQ</div>
            <h2>Частые вопросы</h2>
            <p>Коротко о подключении, устройствах и доступных локациях.</p>
            <Link to="/dashboard/instructions">Инструкции по настройке <IconArrowUpRight size={16} /></Link>
          </div>
          <div className={styles.faqList}>
            {questions.map((item) => (
              <details key={item.q} className={styles.faqItem}>
                <summary>{item.q}<IconChevronDown size={18} /></summary>
                <p>{item.a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
