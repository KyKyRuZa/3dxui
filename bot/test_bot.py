import os
import sys
import pytest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

os.environ.setdefault("BOT_TOKEN", "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11")
os.environ.setdefault("BOT_API_SECRET", "test-secret")
os.environ.setdefault("BACKEND_URL", "http://localhost:8080")
os.environ.setdefault("WEB_APP_URL", "https://example.com")
os.environ.setdefault("NOTIFY_INTERVAL", "3600")

from datetime import datetime, timezone
from unittest.mock import AsyncMock, patch, MagicMock

from main import (
    format_expiry,
    format_config_message,
    referral_anchor,
    main_menu_keyboard,
    fix_os_keyboard,
    referral_link,
    render_bot_notification,
    send_bot_notifications,
    send_expiry_notifications,
    send_expired_notifications,
    send_renewal_notifications,
    backend_claim_login_token,
    backend_generate_bind_code,
    backend_generate_login_code,
    notification_loop,
)


def test_format_expiry_active():
    future_ts = int((datetime.now(timezone.utc).timestamp() + 86400) * 1000)
    data = {"expires_at": future_ts}
    result = format_expiry(data)
    assert result is not None
    assert "Подписка активна до" in result
    assert "осталось" in result


def test_format_expiry_last_day():
    future_ts = int((datetime.now(timezone.utc).timestamp() + 3600) * 1000)
    data = {"expires_at": future_ts}
    result = format_expiry(data)
    assert result is not None
    assert "менее суток" in result


def test_format_expiry_expired():
    past_ts = int((datetime.now(timezone.utc).timestamp() - 3600) * 1000)
    data = {"expires_at": past_ts}
    result = format_expiry(data)
    assert result is not None
    assert "истекла" in result


def test_format_expiry_no_expiry():
    result = format_expiry({})
    assert result is None


def test_format_expiry_zero():
    result = format_expiry({"expires_at": 0})
    assert result is None


def test_referral_anchor_with_link():
    link = "https://t.me/AutoColorsBot?start=abc123"
    result = referral_anchor(link)
    assert "🤝" in result
    assert "+7 дней" in result
    assert link in result


def test_referral_anchor_without_link():
    result = referral_anchor(None)
    assert result == ""


def test_referral_anchor_empty_string():
    result = referral_anchor("")
    assert result == ""


def test_format_config_message_full():
    data = {
        "subscription_url": "https://panel.example.com/sub/abc123",
        "vless": "vless://uuid@host:443?security=reality",
        "singbox": '{"inbounds":[]}',
    }
    result = format_config_message(data)
    assert "🔐 Ваш VPN-ключ готов" in result
    assert "https://panel.example.com/sub/abc123" in result
    assert "vless://" in result
    assert "Sing-box конфиг" in result


def test_format_config_message_no_singbox():
    data = {
        "subscription_url": "https://panel.example.com/sub/abc123",
        "vless": "vless://uuid@host:443?security=reality",
    }
    result = format_config_message(data)
    assert "🔐 Ваш VPN-ключ готов" in result
    assert "vless://" in result


def test_format_config_message_vless_from_links():
    data = {
        "subscription_url": "https://panel.example.com/sub/abc123",
        "links": ["vless://uuid@host:443?security=reality"],
    }
    result = format_config_message(data)
    assert "vless://" in result


def test_format_config_message_no_vless():
    data = {
        "subscription_url": "https://panel.example.com/sub/abc123",
    }
    result = format_config_message(data)
    assert "🔐 Ваш VPN-ключ готов" in result
    assert "vless://" not in result


def test_format_config_message_with_expiry():
    future_ts = int((datetime.now(timezone.utc).timestamp() + 86400) * 1000)
    data = {
        "subscription_url": "https://panel.example.com/sub/abc123",
        "expires_at": future_ts,
    }
    result = format_config_message(data)
    assert "Подписка активна до" in result


def test_main_menu_keyboard_structure():
    kb = main_menu_keyboard()
    assert kb is not None
    buttons = kb.inline_keyboard
    assert len(buttons) >= 5
    texts = [btn.text for row in buttons for btn in row]
    assert any("Купить ключ" in t for t in texts)
    assert any("Моя подписка" in t for t in texts)
    assert any("Реферальная" in t for t in texts)
    assert any("Починить доступ" in t for t in texts)
    assert any("Инструкция" in t for t in texts)


def test_fix_os_keyboard_structure():
    kb = fix_os_keyboard()
    assert kb is not None
    buttons = kb.inline_keyboard
    assert len(buttons) == 3
    texts = [btn.text for row in buttons for btn in row]
    assert any("Windows" in t for t in texts)
    assert any("macOS" in t for t in texts)
    assert any("Linux" in t for t in texts)


def test_render_bot_notification_referral_signup_with_name():
    result = render_bot_notification("referral_signup", {"friend_name": "Alice"})
    assert result is not None
    assert "По вашей реферальной ссылке зарегистрировался друг" in result
    assert "<b>Alice</b>" in result
    assert "+7 дней" in result


def test_render_bot_notification_referral_signup_without_name():
    result = render_bot_notification("referral_signup", {})
    assert result is not None
    assert "По вашей реферальной ссылке зарегистрировался друг" in result


def test_render_bot_notification_referral_reward_default_days():
    result = render_bot_notification("referral_reward", {})
    assert result is not None
    assert "+7 дней" in result
    assert "начислено" in result


def test_render_bot_notification_referral_reward_custom_days():
    result = render_bot_notification("referral_reward", {"reward_days": 14})
    assert result is not None
    assert "+14 дней" in result


def test_render_bot_notification_payment_failed():
    result = render_bot_notification("payment_failed", {})
    assert result is not None
    assert "Оплата не прошла" in result
    assert "попробуйте оплатить ещё раз" in result.lower() or "Попробуйте оплатить ещё раз" in result


def test_render_bot_notification_unknown_kind():
    result = render_bot_notification("unknown_kind", {})
    assert result is None


@pytest.mark.asyncio
async def test_referral_link_success():
    mock_bot = AsyncMock()
    mock_bot.get_me.return_value = MagicMock(username="TestBot")

    with patch("main.bot", mock_bot), \
         patch("main.backend_referral", new_callable=AsyncMock) as mock_backend:
        mock_backend.return_value = {"referral_code": "testcode123"}
        result = await referral_link(12345)
        assert result == "https://t.me/TestBot?start=testcode123"
        mock_backend.assert_called_once_with(12345)


@pytest.mark.asyncio
async def test_referral_link_backend_failure():
    with patch("main.backend_referral", new_callable=AsyncMock) as mock_backend:
        mock_backend.side_effect = Exception("connection error")
        result = await referral_link(12345)
        assert result is None


@pytest.mark.asyncio
async def test_referral_link_no_code():
    mock_bot = AsyncMock()
    with patch("main.bot", mock_bot), \
         patch("main.backend_referral", new_callable=AsyncMock) as mock_backend:
        mock_backend.return_value = {"referral_code": ""}
        result = await referral_link(12345)
        assert result is None


@pytest.mark.asyncio
async def test_referral_link_bot_get_me_failure():
    mock_bot = AsyncMock()
    mock_bot.get_me.side_effect = Exception("telegram error")

    with patch("main.bot", mock_bot), \
         patch("main.backend_referral", new_callable=AsyncMock) as mock_backend:
        mock_backend.return_value = {"referral_code": "testcode123"}
        result = await referral_link(12345)
        assert result is None


@pytest.mark.asyncio
async def test_send_expiry_notifications_no_users():
    with patch("main.backend_expiring", new_callable=AsyncMock) as mock_expiring, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_expiring.return_value = []
        sent = await send_expiry_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_send_expiry_notifications_with_users():
    future_ts = int((datetime.now(timezone.utc).timestamp() + 172800) * 1000)
    with patch("main.backend_expiring", new_callable=AsyncMock) as mock_expiring, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_expiring.return_value = [{"telegram_id": 123, "expires_at": future_ts}]
        mock_ref.return_value = "https://t.me/TestBot?start=abc"
        sent = await send_expiry_notifications()
        assert sent == 1
        mock_send.assert_called_once()


@pytest.mark.asyncio
async def test_send_expiry_notifications_last_day():
    future_ts = int((datetime.now(timezone.utc).timestamp() + 3600) * 1000)
    with patch("main.backend_expiring", new_callable=AsyncMock) as mock_expiring, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_expiring.return_value = [{"telegram_id": 123, "expires_at": future_ts}]
        mock_ref.return_value = None
        sent = await send_expiry_notifications()
        assert sent == 1
        call_args = mock_send.call_args
        text = call_args[0][1]
        assert "Последний день" in text


@pytest.mark.asyncio
async def test_send_expiry_notifications_backend_error():
    with patch("main.backend_expiring", new_callable=AsyncMock) as mock_expiring:
        mock_expiring.side_effect = Exception("connection error")
        sent = await send_expiry_notifications()
        assert sent == 0


@pytest.mark.asyncio
async def test_send_expired_notifications_no_users():
    with patch("main.backend_expired", new_callable=AsyncMock) as mock_expired, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_expired.return_value = []
        sent = await send_expired_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_send_expired_notifications_with_users():
    past_ts = int((datetime.now(timezone.utc).timestamp() - 3600) * 1000)
    with patch("main.backend_expired", new_callable=AsyncMock) as mock_expired, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_expired.return_value = [{"telegram_id": 123, "expires_at": past_ts}]
        mock_ref.return_value = None
        sent = await send_expired_notifications()
        assert sent == 1
        call_args = mock_send.call_args
        text = call_args[0][1]
        assert "Доступ перекрыт" in text


@pytest.mark.asyncio
async def test_send_expired_notifications_backend_error():
    with patch("main.backend_expired", new_callable=AsyncMock) as mock_expired:
        mock_expired.side_effect = Exception("connection error")
        sent = await send_expired_notifications()
        assert sent == 0


@pytest.mark.asyncio
async def test_send_renewal_notifications_no_users():
    with patch("main.backend_renewed", new_callable=AsyncMock) as mock_renewed, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_renewed.return_value = []
        sent = await send_renewal_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_send_renewal_notifications_with_users():
    future_ts = int((datetime.now(timezone.utc).timestamp() + 86400) * 1000)
    with patch("main.backend_renewed", new_callable=AsyncMock) as mock_renewed, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_renewed.return_value = [{"telegram_id": 123, "expires_at": future_ts}]
        mock_ref.return_value = None
        sent = await send_renewal_notifications()
        assert sent == 1
        call_args = mock_send.call_args
        text = call_args[0][1]
        assert "Подписка продлена" in text


@pytest.mark.asyncio
async def test_send_renewal_notifications_backend_error():
    with patch("main.backend_renewed", new_callable=AsyncMock) as mock_renewed:
        mock_renewed.side_effect = Exception("connection error")
        sent = await send_renewal_notifications()
        assert sent == 0


@pytest.mark.asyncio
async def test_send_bot_notifications_no_notifications():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send:
        mock_notifs.return_value = []
        sent = await send_bot_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_send_bot_notifications_referral_signup():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send:
        mock_notifs.return_value = [
            {"telegram_id": 123, "kind": "referral_signup", "data": {"friend_name": "Alice"}}
        ]
        sent = await send_bot_notifications()
        assert sent == 1
        call_args = mock_send.call_args
        text = call_args[0][1]
        assert "По вашей реферальной ссылке зарегистрировался друг" in text


@pytest.mark.asyncio
async def test_send_bot_notifications_referral_reward():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send:
        mock_notifs.return_value = [
            {"telegram_id": 123, "kind": "referral_reward", "data": {"reward_days": 7}}
        ]
        sent = await send_bot_notifications()
        assert sent == 1
        call_args = mock_send.call_args
        text = call_args[0][1]
        assert "+7 дней" in text


@pytest.mark.asyncio
async def test_send_bot_notifications_payment_failed():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send:
        mock_notifs.return_value = [
            {"telegram_id": 123, "kind": "payment_failed", "data": {}}
        ]
        sent = await send_bot_notifications()
        assert sent == 1
        call_args = mock_send.call_args
        text = call_args[0][1]
        assert "Оплата не прошла" in text


@pytest.mark.asyncio
async def test_send_bot_notifications_unknown_kind():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send:
        mock_notifs.return_value = [
            {"telegram_id": 123, "kind": "unknown_kind", "data": {}}
        ]
        sent = await send_bot_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_send_bot_notifications_missing_telegram_id():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send:
        mock_notifs.return_value = [
            {"kind": "referral_signup", "data": {}}
        ]
        sent = await send_bot_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_send_bot_notifications_backend_error():
    with patch("main.backend_notifications", new_callable=AsyncMock) as mock_notifs:
        mock_notifs.side_effect = Exception("connection error")
        sent = await send_bot_notifications()
        assert sent == 0


@pytest.mark.asyncio
async def test_backend_claim_login_token_success():
    mock_response = MagicMock()
    mock_response.status_code = 200
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(return_value=mock_response)
        result = await backend_claim_login_token("token123", 12345)
        assert result is True
        mock_http.post.assert_called_once()


@pytest.mark.asyncio
async def test_backend_claim_login_token_failure():
    mock_response = MagicMock()
    mock_response.status_code = 404
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(return_value=mock_response)
        result = await backend_claim_login_token("bad_token", 12345)
        assert result is False


@pytest.mark.asyncio
async def test_backend_claim_login_token_exception():
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(side_effect=Exception("connection error"))
        result = await backend_claim_login_token("token123", 12345)
        assert result is False


@pytest.mark.asyncio
async def test_notification_loop_keyboard_interrupt():
    with patch("main.asyncio.sleep", side_effect=KeyboardInterrupt), \
         patch("main.send_expiry_notifications", new_callable=AsyncMock), \
         patch("main.send_expired_notifications", new_callable=AsyncMock), \
         patch("main.send_renewal_notifications", new_callable=AsyncMock):
        with pytest.raises(KeyboardInterrupt):
            await notification_loop()


@pytest.mark.asyncio
async def test_notification_loop_runs_once():
    sleep_count = 0

    async def mock_sleep(seconds):
        nonlocal sleep_count
        sleep_count += 1
        if sleep_count > 1:
            raise KeyboardInterrupt

    with patch("main.asyncio.sleep", side_effect=mock_sleep), \
         patch("main.send_expiry_notifications", new_callable=AsyncMock) as mock_expiry, \
         patch("main.send_expired_notifications", new_callable=AsyncMock) as mock_expired, \
         patch("main.send_renewal_notifications", new_callable=AsyncMock) as mock_renewed, \
         patch("main.send_bot_notifications", new_callable=AsyncMock) as mock_bot:
        with pytest.raises(KeyboardInterrupt):
            await notification_loop()
        mock_expiry.assert_called_once()
        mock_expired.assert_called_once()
        mock_renewed.assert_called_once()
        mock_bot.assert_called_once()


@pytest.mark.asyncio
async def test_backend_generate_bind_code_success():
    mock_response = MagicMock()
    mock_response.status_code = 200
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(return_value=mock_response)
        result = await backend_generate_bind_code(12345, "48271593")
        assert result is True


@pytest.mark.asyncio
async def test_backend_generate_bind_code_failure():
    mock_response = MagicMock()
    mock_response.status_code = 400
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(return_value=mock_response)
        result = await backend_generate_bind_code(12345, "bad_code")
        assert result is False


@pytest.mark.asyncio
async def test_backend_generate_bind_code_exception():
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(side_effect=Exception("connection error"))
        result = await backend_generate_bind_code(12345, "48271593")
        assert result is False


@pytest.mark.asyncio
async def test_backend_generate_login_code_success():
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"code": "48271593", "expires_in": 300}
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(return_value=mock_response)
        result = await backend_generate_login_code(12345)
        assert result == "48271593"


@pytest.mark.asyncio
async def test_backend_generate_login_code_failure():
    mock_response = MagicMock()
    mock_response.status_code = 404
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(return_value=mock_response)
        result = await backend_generate_login_code(99999)
        assert result is None


@pytest.mark.asyncio
async def test_backend_generate_login_code_exception():
    with patch("main.http_client") as mock_http:
        mock_http.post = AsyncMock(side_effect=Exception("connection error"))
        result = await backend_generate_login_code(12345)
        assert result is None
