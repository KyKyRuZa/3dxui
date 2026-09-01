import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import AuthForm from "@pages/AuthForm";
import { AuthProvider } from "@store/auth";
import * as configApi from "@api/config";
import * as authApi from "@api/auth";

vi.mock("@api/config");
vi.mock("@api/auth");

const renderAuthForm = () =>
  render(
    <MemoryRouter>
      <AuthProvider>
        <AuthForm />
      </AuthProvider>
    </MemoryRouter>
  );

describe("AuthForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(authApi.refresh).mockRejectedValueOnce(new Error("not logged in"));
    vi.mocked(configApi.getPublicConfig).mockResolvedValue({
      yookassa_test_mode: false,
      bot_username: "TestBot",
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders login title and subtitle", () => {
    renderAuthForm();
    expect(screen.getByText("Вход через Telegram")).toBeDefined();
    expect(screen.getByText(/Регистрация и вход выполняются только через Telegram/)).toBeDefined();
  });

  it("renders Telegram login button when no loginUrl", () => {
    renderAuthForm();
    expect(screen.getByText("Войти через Telegram")).toBeDefined();
  });

  it("fetches bot username from public config on mount", async () => {
    renderAuthForm();
    await waitFor(() => {
      expect(configApi.getPublicConfig).toHaveBeenCalled();
    });
    expect(screen.getByText(/@TestBot/)).toBeDefined();
  });

  it("falls back to default bot username on config error", async () => {
    vi.mocked(configApi.getPublicConfig).mockRejectedValueOnce(new Error("fail"));
    renderAuthForm();
    await waitFor(() => {
      expect(screen.getByText(/@AutoColorsBot/)).toBeDefined();
    });
  });

  it("shows link to bot when loginUrl is set", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ token: "abc", login_url: "https://t.me/TestBot?start=abc", expires_in: 300 }), { status: 200 })
    );

    renderAuthForm();
    // Check consent checkbox first
    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);
    const btn = screen.getByText("Войти через Telegram");
    fireEvent.click(btn);

    await waitFor(() => {
      expect(screen.getByText("Открыть Telegram-бота")).toBeDefined();
    });
  });

  it("shows waiting status after link is created", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ token: "abc", login_url: "https://t.me/TestBot?start=abc", expires_in: 300 }), { status: 200 })
    );

    renderAuthForm();
    // Check consent checkbox first
    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByText("Войти через Telegram"));

    await waitFor(() => {
      expect(screen.getByText(/Ожидание подтверждения в Telegram/)).toBeDefined();
    });
  });

  it("shows error when link creation fails", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response("error", { status: 500 })
    );

    renderAuthForm();
    // Check consent checkbox first
    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByText("Войти через Telegram"));

    await waitFor(() => {
      expect(screen.getByText(/Не удалось создать ссылку для входа/)).toBeDefined();
    });
  });

  it("shows error when consent not given", async () => {
    renderAuthForm();
    fireEvent.click(screen.getByText("Войти через Telegram"));

    await waitFor(() => {
      expect(screen.getByText(/необходимо согласие/)).toBeDefined();
    });
  });
});
