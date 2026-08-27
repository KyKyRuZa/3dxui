import api from "./axios";

export interface ReferralStats {
  referral_code: string;
  invited: number;
  earned_days: number;
  bot_username: string;
}

export async function getReferral(): Promise<ReferralStats> {
  const { data } = await api.get<ReferralStats>("/referral");
  return data;
}
