import api from "./axios";

export interface Plan {
  id: string;
  name: string;
  duration_days: number;
  price_minor: number;
  currency: string;
  group_name: string;
}

export interface PaymentResult {
  payment_id: string;
  confirmation_url: string;
  status: string;
}

export async function getPlans(): Promise<Plan[]> {
  const { data } = await api.get<{ plans: Plan[] }>("/billing/plans");
  return data.plans;
}

export async function createPayment(planId: string): Promise<PaymentResult> {
  const { data } = await api.post<PaymentResult>("/billing/create", {
    plan_id: planId,
  });
  return data;
}
