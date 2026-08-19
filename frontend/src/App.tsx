import { Suspense } from "react";
import { Routes, Route, Navigate } from "react-router-dom";
import { useAuth } from "./hooks/useAuth";
import Navbar from "./components/Navbar";
import Footer from "./components/Footer";
import DashboardShell from "./components/DashboardShell";
import Landing from "./pages/Landing";
import PricingPage from "./pages/PricingPage";
import Login from "./pages/Login";
import Register from "./pages/Register";
import DashboardOverview from "./pages/DashboardOverview";
import Subscription from "./pages/Subscription";
import Instructions from "./pages/Instructions";
import Settings from "./pages/Settings";
import styles from "./styles/global.module.css";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth();
  if (loading) {
    return (
      <div className="container" style={{ paddingTop: 100 }}>
        Загрузка…
      </div>
    );
  }
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <div className={styles.root}>
      <Navbar />
      <Suspense
        fallback={
          <div className="container" style={{ paddingTop: 100 }}>
            Загрузка…
          </div>
        }
      >
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/pricing" element={<PricingPage />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <DashboardShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<DashboardOverview />} />
            <Route path="subscription" element={<Subscription />} />
            <Route path="instructions" element={<Instructions />} />
            <Route path="settings" element={<Settings />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Suspense>
      <Footer />
    </div>
  );
}
