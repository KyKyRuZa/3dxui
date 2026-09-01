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

  it("renders login title and form fields", () => {
    renderAuthForm();
    expect(screen.getByText("Вход в аккаунт")).toBeDefined();
    expect(screen.getByPlaceholderText("username")).toBeDefined();
    expect(screen.getByPlaceholderText("••••••")).toBeDefined();
  });

  it("renders register mode when switched", () => {
    renderAuthForm();
    fireEvent.click(screen.getByText("Зарегистрироваться"));
    expect(screen.getByText("Регистрация")).toBeDefined();
    expect(screen.getByRole("checkbox")).toBeDefined();
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
    const tgBtn = screen.getByText("Войти через Telegram");
    fireEvent.click(tgBtn);

    await waitFor(() => {
      expect(screen.getByText("Открыть Telegram-бота")).toBeDefined();
    });
  });

  it("shows waiting status after link is created", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ token: "abc", login_url: "https://t.me/TestBot?start=abc", expires_in: 300 }), { status: 200 })
    );

    renderAuthForm();
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
    fireEvent.click(screen.getByText("Войти через Telegram"));

    await waitFor(() => {
      expect(screen.getByText(/Не удалось создать ссылку/)).toBeDefined();
    });
  });

  it("shows error when register without consent", async () => {
    renderAuthForm();
    // Switch to register mode by clicking the link
    const switchLink = screen.getByText("Зарегистрироваться");
    fireEvent.click(switchLink);

    // Wait for register mode to render (checkbox appears)
    await waitFor(() => {
      expect(screen.getByRole("checkbox")).toBeDefined();
    });

    // Fill in username and password
    fireEvent.change(screen.getByPlaceholderText("username"), { target: { value: "newuser" } });
    fireEvent.change(screen.getByPlaceholderText("••••••"), { target: { value: "password123" } });

    // Click the register button (now visible)
    const registerBtn = screen.getByText("Зарегистрироваться");
    fireEvent.click(registerBtn);

    await waitFor(() => {
      expect(screen.getByText(/Для регистрации необходимо/)).toBeDefined();
    });
  });

  it("calls login API on form submit", async () => {
    const fetchMock = vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ access_token: "test_token", user: { id: 1, username: "test" } }), { status: 200 })
    );

    renderAuthForm();
    fireEvent.change(screen.getByPlaceholderText("username"), { target: { value: "testuser" } });
    fireEvent.change(screen.getByPlaceholderText("••••••"), { target: { value: "password123" } });
    fireEvent.click(screen.getByText("Войти"));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith("/api/auth/login", expect.objectContaining({
        method: "POST",
      }));
    });
  });

  it("calls register API on form submit", async () => {
    const fetchMock = vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ access_token: "test_token", user: { id: 1, username: "newuser" } }), { status: 200 })
    );

    renderAuthForm();
    // Switch to register mode
    fireEvent.click(screen.getByText("Зарегистрироваться"));
    fireEvent.change(screen.getByPlaceholderText("username"), { target: { value: "newuser" } });
    fireEvent.change(screen.getByPlaceholderText("••••••"), { target: { value: "password123" } });
    // Check consent
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByText("Зарегистрироваться"));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith("/api/auth/register", expect.objectContaining({
        method: "POST",
      }));
    });
  });
});
