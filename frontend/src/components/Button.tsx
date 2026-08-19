import type { ButtonHTMLAttributes, ReactNode } from "react";
import styles from "./Button.module.css";

type Variant = "primary" | "secondary";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
  block?: boolean;
  children: ReactNode;
}

export function Button({
  variant = "primary",
  loading = false,
  block = false,
  className = "",
  disabled,
  children,
  ...props
}: ButtonProps) {
  const cls = [
    styles.button,
    styles[variant],
    block ? styles.block : "",
    loading ? styles.loading : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button className={cls} disabled={disabled || loading} {...props}>
      {loading && <span className={styles.spinner} aria-hidden />}
      <span className={styles.label}>{children}</span>
    </button>
  );
}

export default Button;
