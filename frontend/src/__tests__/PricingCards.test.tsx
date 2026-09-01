import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import PricingCards from "@components/PricingCards";
import * as billingApi from "@api/billing";
import * as configApi from "@api/config";

vi.mock("@api/billing");
vi.mock("@api/config");

const mockPlans: billingApi.Plan[] = [
  { id: "standard", name: "Standard", duration_days: 30, price_minor: 29900, currency: "RUB", group_name: "Free" },
  { id: "pro", name: "Pro", duration_days: 90, price_minor: 79900, currency: "RUB", group_name: "Free" },
];

const renderPricingCards = () =>
  render(
    <MemoryRouter>
      <PricingCards />
    </MemoryRouter>
  );

describe("PricingCards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(configApi.getPublicConfig).mockResolvedValue({
      yookassa_test_mode: false,
      bot_username: "TestBot",
    });
  });

  it("renders loading state initially", () => {
    vi.mocked(billingApi.getPlans).mockImplementation(() => new Promise(() => {}));
    renderPricingCards();
    expect(screen.getByText("Загрузка тарифов…")).toBeDefined();
  });

  it("renders plans after loading", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
      expect(screen.getByText("Pro")).toBeDefined();
    });
  });

  it("renders plan prices correctly", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText(/299,00/)).toBeDefined();
      expect(screen.getByText(/799,00/)).toBeDefined();
    });
  });

  it("renders plan durations", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText(/30 дней/)).toBeDefined();
      expect(screen.getByText(/90 дней/)).toBeDefined();
    });
  });

  it("shows test mode badge when yookassa_test_mode is true", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(configApi.getPublicConfig).mockResolvedValueOnce({
      yookassa_test_mode: true,
      bot_username: "TestBot",
    });
    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText(/Тестовый режим оплаты/)).toBeDefined();
    });
  });

  it("does not show test mode badge when yookassa_test_mode is false", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    renderPricingCards();

    await waitFor(() => {
      expect(screen.queryByText(/Тестовый режим оплаты/)).toBeNull();
    });
  });

  it("shows error when plans fail to load", async () => {
    vi.mocked(billingApi.getPlans).mockRejectedValueOnce(new Error("fail"));
    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Не удалось загрузить тарифы")).toBeDefined();
    });
  });

  it("calls createPayment when buy button is clicked", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(billingApi.createPayment).mockResolvedValueOnce({
      payment_id: "pay_123",
      confirmation_url: "https://yookassa.ru/pay/123",
      status: "pending",
    });

    const windowOpenSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
    });

    const buyButtons = screen.getAllByText("Купить");
    fireEvent.click(buyButtons[0]);

    await waitFor(() => {
      expect(billingApi.createPayment).toHaveBeenCalledWith("standard");
      expect(windowOpenSpy).toHaveBeenCalledWith("https://yookassa.ru/pay/123", "_blank", "noopener");
    });

    windowOpenSpy.mockRestore();
  });

  it("shows info message after successful payment creation", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(billingApi.createPayment).mockResolvedValueOnce({
      payment_id: "pay_123",
      confirmation_url: "https://yookassa.ru/pay/123",
      status: "pending",
    });
    vi.spyOn(window, "open").mockImplementation(() => null);

    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
    });

    fireEvent.click(screen.getAllByText("Купить")[0]);

    await waitFor(() => {
      expect(screen.getByText(/Открыта страница оплаты ЮKassa/)).toBeDefined();
    });
  });

  it("shows error when payment creation fails", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(billingApi.createPayment).mockRejectedValueOnce(new Error("fail"));

    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
    });

    fireEvent.click(screen.getAllByText("Купить")[0]);

    await waitFor(() => {
      expect(screen.getByText("Не удалось создать платёж. Попробуйте позже.")).toBeDefined();
    });
  });
});
