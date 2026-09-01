import { Suspense, lazy } from "react";
import { Routes, Route, Navigate } from "react-router-dom";
import { useAuth } from "@hooks/useAuth";
import Navbar from "@components/Navbar";
import Footer from "@components/Footer";
import DashboardShell from "@components/DashboardShell";
import CookieConsent from "@components/CookieConsent";
import styles from "@styles/global.module.css";

const Landing = lazy(() => import("./pages/Landing"));
const PricingPage = lazy(() => import("./pages/PricingPage"));
const Login = lazy(() => import("./pages/Login"));
const Register = lazy(() => import("./pages/Register"));
const Privacy = lazy(() => import("./pages/Privacy"));
const DashboardOverview = lazy(() => import("./pages/DashboardOverview"));
const Subscription = lazy(() => import("./pages/Subscription"));
const Instructions = lazy(() => import("./pages/Instructions"));
const Settings = lazy(() => import("./pages/Settings"));

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth();
  if (loading) {
    return (
      <div className={`container ${styles.loadingState}`}>
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
          <div className={`container ${styles.loadingState}`}>
            Загрузка…
          </div>
        }
      >
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/pricing" element={<PricingPage />} />
          <Route path="/privacy" element={<Privacy />} />
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
      <CookieConsent />
    </div>
  );
}
