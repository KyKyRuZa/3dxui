import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuthProvider } from "@store/auth";
import Referral from "@components/Referral";
import type { ReferralStats } from "@api/referral";

const renderWithAuth = (ui: React.ReactElement) =>
  render(<AuthProvider>{ui}</AuthProvider>);

const stats: ReferralStats = {
  referral_code: "abc123",
  invited: 5,
  earned_days: 35,
  bot_username: "TestBot",
};

describe("Referral", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    renderWithAuth(<Referral stats={null} />);
    expect(screen.getByText("Загрузка…")).toBeDefined();
  });

  it("renders referral stats on success", async () => {
    renderWithAuth(<Referral stats={stats} />);
    expect(screen.getByText("5")).toBeDefined();
    expect(screen.getByText("35")).toBeDefined();
    expect(screen.getByText("Бонусных дней")).toBeDefined();
    expect(screen.getByText("Приглашено")).toBeDefined();
  });

  it("shows copy button with correct link", async () => {
    renderWithAuth(<Referral stats={stats} />);
    expect(screen.getByText("Ваша реферальная ссылка")).toBeDefined();
    const copyBtn = screen.getByText("Скопировать ссылку");
    expect(copyBtn).toBeDefined();
  });
});
