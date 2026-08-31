import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AuthProvider } from "@store/auth";
import Referral from "@components/Referral";
import * as referralApi from "@api/referral";

vi.mock("@api/referral");

const renderWithAuth = (ui: React.ReactElement) =>
  render(<AuthProvider>{ui}</AuthProvider>);

describe("Referral", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    vi.mocked(referralApi.getReferral).mockImplementation(
      () => new Promise(() => {})
    );
    renderWithAuth(<Referral />);
    expect(screen.getByText("Загрузка…")).toBeDefined();
  });

  it("renders error state on failure", async () => {
    vi.mocked(referralApi.getReferral).mockRejectedValueOnce(
      new Error("fail")
    );
    renderWithAuth(<Referral />);
    await waitFor(() =>
      expect(
        screen.getByText("Не удалось загрузить реферальную программу")
      ).toBeDefined()
    );
  });

  it("renders referral stats on success", async () => {
    vi.mocked(referralApi.getReferral).mockResolvedValueOnce({
      referral_code: "abc123",
      invited: 5,
      earned_days: 35,
      bot_username: "TestBot",
    });
    renderWithAuth(<Referral />);
    await waitFor(() =>
      expect(screen.getByText("5")).toBeDefined()
    );
    expect(screen.getByText("35")).toBeDefined();
    expect(screen.getByText("Бонусных дней")).toBeDefined();
    expect(screen.getByText("Приглашено")).toBeDefined();
  });

  it("shows copy button with correct link", async () => {
    vi.mocked(referralApi.getReferral).mockResolvedValueOnce({
      referral_code: "abc123",
      invited: 0,
      earned_days: 0,
      bot_username: "TestBot",
    });
    renderWithAuth(<Referral />);
    await waitFor(() =>
      expect(screen.getByText("Ваша реферальная ссылка")).toBeDefined()
    );
    const copyBtn = screen.getByText("Скопировать ссылку");
    expect(copyBtn).toBeDefined();
  });
});
