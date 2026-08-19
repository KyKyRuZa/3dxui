import Hero from "../components/Hero";
import Features from "../components/Features";
import PricingCards from "../components/PricingCards";
import styles from "../styles/global.module.css";

export default function Landing() {
  return (
    <div className={styles.root}>
      <Hero />
      <Features />
      <PricingCards />
    </div>
  );
}
