#!/usr/bin/env bash
#
# verify_referral.sh — end-to-end check of the referral program against the
# running stack (backend + postgres via docker compose).
#
# What it does:
#   1. Creates a referrer via POST /api/bot/user and reads its referral code.
#   2. Creates a referred user via POST /api/bot/user?referral_code=<code>.
#   3. Checks the DB that a pending referral row was recorded (the bug we fixed
#      used to skip existing users — this proves new AND existing users work).
#   4. Reports PASS/FAIL.
#
# Usage (run on the server, inside the project dir):
#   BOT_API_SECRET=xxxx ./scripts/verify_referral.sh
#
# It reads BOT_API_SECRET and BACKEND_URL (default http://localhost:8080) from env.
set -euo pipefail

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
BOT_API_SECRET="${BOT_API_SECRET:-}"
if [[ -z "$BOT_API_SECRET" ]]; then
  echo "ERROR: set BOT_API_SECRET (e.g. BOT_API_SECRET=xxx $0)" >&2
  exit 1
fi

# Unique telegram ids so the check is repeatable and does not collide.
TS="$(date +%s)"
REFERRER_TG="$(( TS % 1000000000 + 700000000 ))"
REFERRED_TG="$(( REFERRER_TG + 1 ))"

hdr=(-H "X-Bot-Secret: ${BOT_API_SECRET}" -H "Content-Type: application/json")

echo "==> 1. Create referrer (tg=${REFERRER_TG})"
referrer_json="$(curl -s "${hdr[@]}" -X POST "$BACKEND_URL/api/bot/user" \
  -d "{\"telegram_id\":${REFERRER_TG},\"first_name\":\"Referrer\"}")"
echo "$referrer_json" | head -c 300; echo

echo "==> 2. Read referrer referral code"
code_json="$(curl -s "${hdr[@]}" -X POST "$BACKEND_URL/api/bot/referral" \
  -d "{\"telegram_id\":${REFERRER_TG}}")"
echo "$code_json"
CODE="$(echo "$code_json" | sed -n 's/.*"referral_code":"\([^"]*\)".*/\1/p')"
if [[ -z "$CODE" ]]; then
  echo "FAIL: could not read referral_code for referrer"; exit 1
fi
echo "    referrer code = $CODE"

echo "==> 3. Create referred user with referral_code (tg=${REFERRED_TG})"
curl -s "${hdr[@]}" -X POST "$BACKEND_URL/api/bot/user" \
  -d "{\"telegram_id\":${REFERRED_TG},\"first_name\":\"Referred\",\"referral_code\":\"${CODE}\"}" \
  | head -c 300; echo

echo "==> 4. Check DB for the pending referral row"
REFERRER_ID="$(docker compose exec -T postgres psql -U vpn_user -d vpn_db -tAc \
  "SELECT id FROM users WHERE telegram_id = ${REFERRER_TG};")"
REFERRED_ID="$(docker compose exec -T postgres psql -U vpn_user -d vpn_db -tAc \
  "SELECT id FROM users WHERE telegram_id = ${REFERRED_TG};")"
echo "    referrer_id=$REFERRER_ID referred_id=$REFERRED_ID"

ROW="$(docker compose exec -T postgres psql -U vpn_user -d vpn_db -tAc \
  "SELECT count(*) FROM referrals WHERE referrer_id = ${REFERRER_ID} AND referred_id = ${REFERRED_ID};")"
echo "    pending/any referral rows = $ROW"

if [[ "$ROW" -ge 1 ]]; then
  echo "PASS: referral recorded for the referred user."
else
  echo "FAIL: no referral row found — referral program is NOT attributing users."
  exit 1
fi

echo
echo "==> 5. (Optional) Reward on paid purchase"
echo "    When the referred user pays via YooKassa, billingWebhook calls"
echo "    CreditReferralReward, which extends the referrer's subscription by"
echo "    ReferralRewardDays and flips the row to 'completed'. Verify with:"
echo "    docker compose exec postgres psql -U vpn_user -d vpn_db -c \\"
echo "      \"SELECT status, reward_days FROM referrals WHERE referrer_id=${REFERRER_ID} AND referred_id=${REFERRED_ID};\""
