import styles from "./LoadingCard.module.css";

export default function LoadingCard() {
  return (
    <section className={styles.card}>
      <h2>Authentication</h2>
      <p className="muted">Loading session...</p>
    </section>
  );
}
