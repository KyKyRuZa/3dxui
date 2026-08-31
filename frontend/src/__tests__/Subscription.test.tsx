import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { AuthProvider } from "@store/auth";
import Subscription from "@pages/Subscription";
import * as subscriptionApi from "@api/subscription";

vi.mock("@api/subscription");

const renderWithAuth = (ui: React.ReactElement) =>
  render(
    <BrowserRouter>
      <AuthProvider>{ui}</AuthProvider>
    </BrowserRouter>
  );

describe("Subscription", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    vi.mocked(subscriptionApi.activateSubscription).mockImplementation(
      () => new Promise(() => {})
    );
    renderWithAuth(<Subscription />);
    const loadingElements = screen.getAllByText("Загрузка…");
    expect(loadingElements.length).toBeGreaterThan(0);
  });

  it("renders active subscription", async () => {
    const future = Date.now() + 86400000;
    vi.mocked(subscriptionApi.activateSubscription).mockResolvedValueOnce({
      subscription_url: "https://panel.example.com/sub/abc",
      username: "tg_1",
      group: "Free",
      vless: "vless://uuid@host:443?security=reality",
      links: [],
      expires_at: future,
    });
    renderWithAuth(<Subscription />);
    await waitFor(() =>
      expect(screen.getByText(/Подписка активна до/)).toBeDefined()
    );
  });

  it("renders expired banner when expired", async () => {
    const past = Date.now() - 3600000;
    vi.mocked(subscriptionApi.activateSubscription).mockResolvedValueOnce({
      subscription_url: "https://panel.example.com/sub/abc",
      username: "tg_1",
      group: "Free",
      vless: "vless://uuid@host:443?security=reality",
      links: [],
      expires_at: past,
    });
    renderWithAuth(<Subscription />);
    await waitFor(() =>
      expect(screen.getByText(/Подписка истекла/)).toBeDefined()
    );
  });

  it("shows error on fetch failure", async () => {
    vi.mocked(subscriptionApi.activateSubscription).mockRejectedValueOnce(
      new Error("fail")
    );
    renderWithAuth(<Subscription />);
    await waitFor(() =>
      expect(screen.getByText("Не удалось активировать подписку")).toBeDefined()
    );
  });
});
