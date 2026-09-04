import axios from "axios";

export const adminApi = axios.create({
  baseURL: "/api/admin",
  withCredentials: true,
});

export type PlanInput = {
  id?: string;
  name: string;
  duration_days: number;
  price_minor: number;
  currency?: string;
  group_name?: string;
};

export type DiscountInput = {
  id?: string;
  code: string;
  plan_id?: string | null;
  percent: number;
  fixed_minor: number;
  starts_at: string;
  expires_at: string;
  max_uses: number;
  used_count?: number;
  is_active: boolean;
};
