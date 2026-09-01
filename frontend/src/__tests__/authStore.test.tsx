import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { AuthProvider, useAuth } from "@store/auth";
import * as authApi from "@api/auth";
import { setAccessToken } from "@api/axios";

vi.mock("@api/auth");
vi.mock("@api/axios", () => ({
  setAccessToken: vi.fn(),
  registerAuthFail: vi.fn(),
}));

function TestComponent() {
  const { isAuthenticated, user, loading } = useAuth();
  return (
    <div>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="authenticated">{String(isAuthenticated)}</span>
      <span data-testid="username">{user?.username ?? "none"}</span>
    </div>
  );
}

function renderWithAuth(ui: React.ReactElement) {
  return render(<AuthProvider>{ui}</AuthProvider>);
}

describe("AuthProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("window", window);
  });

  it("starts with loading true and unauthenticated", () => {
    vi.mocked(authApi.refresh).mockImplementation(() => new Promise(() => {}));
    renderWithAuth(<TestComponent />);
    expect(screen.getByTestId("loading").textContent).toBe("true");
    expect(screen.getByTestId("authenticated").textContent).toBe("false");
  });

  it("becomes authenticated after successful refresh", async () => {
    vi.mocked(authApi.refresh).mockResolvedValueOnce({
      access_token: "token123",
      user: { id: 1, username: "testuser", email: "", is_active: true, created_at: "" },
    });

    renderWithAuth(<TestComponent />);

    await waitFor(() => {
      expect(screen.getByTestId("authenticated").textContent).toBe("true");
      expect(screen.getByTestId("username").textContent).toBe("testuser");
    });

    expect(setAccessToken).toHaveBeenCalledWith("token123");
  });

  it("becomes unauthenticated after failed refresh", async () => {
    vi.mocked(authApi.refresh).mockRejectedValueOnce(new Error("fail"));
    Object.defineProperty(window, "Telegram", { value: undefined, writable: true });

    renderWithAuth(<TestComponent />);

    await waitFor(() => {
      expect(screen.getByTestId("loading").textContent).toBe("false");
      expect(screen.getByTestId("authenticated").textContent).toBe("false");
    });
  });

  it("falls back to Telegram WebApp initData on refresh failure", async () => {
    vi.mocked(authApi.refresh).mockRejectedValueOnce(new Error("fail"));
    vi.mocked(authApi.telegram).mockResolvedValueOnce({
      access_token: "tg_token",
      user: { id: 2, username: "tguser", email: "", is_active: true, created_at: "" },
    });

    Object.defineProperty(window, "Telegram", {
      value: { WebApp: { initData: "init_data_here" } },
      writable: true,
    });

    renderWithAuth(<TestComponent />);

    await waitFor(() => {
      expect(screen.getByTestId("authenticated").textContent).toBe("true");
      expect(screen.getByTestId("username").textContent).toBe("tguser");
    });

    expect(authApi.telegram).toHaveBeenCalledWith("init_data_here");
  });

  it("logout clears user and token", async () => {
    vi.mocked(authApi.refresh).mockResolvedValueOnce({
      access_token: "token123",
      user: { id: 1, username: "testuser", email: "", is_active: true, created_at: "" },
    });
    vi.mocked(authApi.logout).mockResolvedValueOnce(undefined);

    function LogoutTest() {
      const { logout } = useAuth();
      return <button onClick={() => logout()}>Logout</button>;
    }

    renderWithAuth(
      <>
        <TestComponent />
        <LogoutTest />
      </>
    );

    await waitFor(() => {
      expect(screen.getByTestId("authenticated").textContent).toBe("true");
    });

    await act(async () => {
      screen.getByText("Logout").click();
    });

    expect(authApi.logout).toHaveBeenCalled();
    expect(setAccessToken).toHaveBeenCalledWith(null);
  });
});
