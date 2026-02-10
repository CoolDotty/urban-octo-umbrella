import { Outlet } from "react-router-dom";
import styles from "./AppLayout.module.css";

export default function AppLayout() {
  return (
    <div className={styles.app}>
      <header className={styles.hero}>
        <p className={styles.eyebrow}>PocketBase + Go + React</p>
        <h1>Urban Octo Umbrella</h1>
        <p className={styles.lede}>
          Single-binary deployment with PocketBase embedded in Go and a React
          frontend served from the same executable.
        </p>
      </header>
      <Outlet />
    </div>
  );
}
