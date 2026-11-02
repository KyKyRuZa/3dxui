import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import PricingCards from "@components/PricingCards";
import * as billingApi from "@api/billing";

vi.mock("@api/billing");

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
  });

  it("renders loading state initially", () => {
    vi.mocked(billingApi.getPlans).mockImplementation(() => new Promise(() => {}));
    renderPricingCards();
    expect(screen.getByText("Загружаем варианты подписки…")).toBeDefined();
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
      expect(screen.getAllByText(/30 дней/).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/90 дней/).length).toBeGreaterThan(0);
    });
  });

  it("does not display internal payment configuration", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    renderPricingCards();
    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
      expect(screen.queryByText(/тестовый режим|служебная конфигурация/i)).toBeNull();
    });
  });

  it("shows error when plans fail to load", async () => {
    vi.mocked(billingApi.getPlans).mockRejectedValueOnce(new Error("fail"));
    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Не удалось загрузить тарифы. Попробуйте обновить страницу.")).toBeDefined();
    });
  });

  it("calls createPayment when buy button is clicked", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(billingApi.createPayment).mockResolvedValueOnce({
      payment_id: "pay_123",
      confirmation_url: "https://checkout.example.com/pay/123",
      status: "pending",
    });

    const windowOpenSpy = vi.spyOn(window, "open").mockImplementation(() => window);

    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
    });

    const buyButtons = screen.getAllByText("Выбрать тариф");
    fireEvent.click(buyButtons[0]);

    await waitFor(() => {
      expect(billingApi.createPayment).toHaveBeenCalledWith("standard");
      expect(windowOpenSpy).toHaveBeenCalledWith("https://checkout.example.com/pay/123", "_blank", "noopener,noreferrer");
    });

    windowOpenSpy.mockRestore();
  });

  it("shows info message after successful payment creation", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(billingApi.createPayment).mockResolvedValueOnce({
      payment_id: "pay_123",
      confirmation_url: "https://checkout.example.com/pay/123",
      status: "pending",
    });
    vi.spyOn(window, "open").mockImplementation(() => window);

    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
    });

    fireEvent.click(screen.getAllByText("Выбрать тариф")[0]);

    await waitFor(() => {
      expect(screen.getByText(/Ссылка на оплату подготовлена/)).toBeDefined();
      expect(screen.getByText("Открыть страницу оплаты")).toBeDefined();
    });
  });

  it("shows error when payment creation fails", async () => {
    vi.mocked(billingApi.getPlans).mockResolvedValueOnce(mockPlans);
    vi.mocked(billingApi.createPayment).mockRejectedValueOnce(new Error("fail"));

    renderPricingCards();

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeDefined();
    });

    fireEvent.click(screen.getAllByText("Выбрать тариф")[0]);

    await waitFor(() => {
      expect(screen.getByText("Не удалось подготовить оплату. Попробуйте позже.")).toBeDefined();
    });
  });
});
