import { Link } from "react-router-dom";
import {
  IconArrowUpRight,
  IconCheck,
  IconDeviceLaptop,
  IconLock,
  IconWorld,
} from "@tabler/icons-react";
import styles from "@styles/Hero.module.css";

export default function Hero() {
  return (
    <section className={styles.hero}>
      <div className={`container ${styles.inner}`}>
        <div className={styles.copy}>
          <div className={styles.eyebrow}>
            <span className={styles.eyebrowMark} />
            ЗАЩИЩЁННОЕ ПОДКЛЮЧЕНИЕ
          </div>
          <h1 className={styles.title}>
            Приватность — это часть <span>хорошего соединения.</span>
          </h1>
          <p className={styles.subtitle}>
            Шифрование трафика, смена IP-адреса и серверы в разных странах. Современная сетевая инфраструктура для повседневной работы — без сложной настройки.
          </p>
          <div className={styles.actions}>
            <Link to="/register" className={styles.primaryAction}>
              Начать бесплатно <IconArrowUpRight size={17} />
            </Link>
            <a className={styles.secondaryAction} href="#servers">
              Изучить сеть <span aria-hidden="true">↓</span>
            </a>
          </div>
          <div className={styles.note}>
            <span><IconCheck size={15} /> 3 дня пробного доступа</span>
            <i />
            <span><IconDeviceLaptop size={16} /> Одна подписка для устройств</span>
          </div>
        </div>

        <div className={styles.networkStage} aria-label="Схема сети: устройство подключается к серверам в Германии или США">
          <div className={styles.networkIntro}>
            <span>ГЛОБАЛЬНАЯ СЕТЬ</span>
            <strong>Одна точка входа. Две локации.</strong>
          </div>
          <div className={styles.networkFlow}>
            <div className={styles.sourceNode}>
              <span className={styles.sourceIcon}><IconDeviceLaptop size={22} stroke={1.7} /></span>
              <span><small>ИСТОЧНИК</small><strong>Ваше устройство</strong></span>
            </div>
            <div className={styles.routeLine} aria-hidden="true">
              <svg viewBox="0 0 360 100" preserveAspectRatio="none">
                <path d="M0 50 H120 C160 50 160 18 205 18 H360" />
                <path d="M0 50 H120 C160 50 160 82 205 82 H360" />
                <circle cx="120" cy="50" r="3" />
                <circle cx="205" cy="18" r="3" />
                <circle cx="205" cy="82" r="3" />
              </svg>
              <span className={styles.routeLabel}><IconLock size={12} /> ЗАШИФРОВАННЫЙ КАНАЛ</span>
            </div>
            <div className={styles.destinations}>
              <div className={styles.destination}>
                <span className={styles.countryCode}>DE</span>
                <span><strong>Germany</strong><small>Европа</small></span>
              </div>
              <div className={styles.destination}>
                <span className={styles.countryCode}>US</span>
                <span><strong>United States</strong><small>Северная Америка</small></span>
              </div>
            </div>
          </div>
          <div className={styles.networkFoot}>
            <span><i /> Современные протоколы</span>
            <span><IconWorld size={14} /> Управление локацией в одном профиле</span>
          </div>
        </div>
      </div>
      <div className={styles.heroBottom}>
        <div className="container">
          <span>ПРИВАТНОСТЬ</span><i />
          <span>СТАБИЛЬНОСТЬ</span><i />
          <span>ПРОСТОЕ УПРАВЛЕНИЕ</span>
        </div>
      </div>
    </section>
  );
}
