import { Outlet } from "react-router-dom";

export default function AppLayout() {
  return (
    <div className="layout">
      <nav>
        <a href="/dashboard">Dashboard</a>
      </nav>
      <main>
        <Outlet />
      </main>
    </div>
  );
}
