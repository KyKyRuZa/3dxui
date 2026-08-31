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
    send_expiry_notifications,
    send_expired_notifications,
    send_renewal_notifications,
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
async def test_send_expiry_notifications_no_users():
    with patch("main.backend_expiring", new_callable=AsyncMock) as mock_expiring, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_expiring.return_value = []
        sent = await send_expiry_notifications()
        assert sent == 0
        mock_send.assert_not_called()


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
async def test_send_renewal_notifications_no_users():
    with patch("main.backend_renewed", new_callable=AsyncMock) as mock_renewed, \
         patch("main.bot.send_message", new_callable=AsyncMock) as mock_send, \
         patch("main.referral_link", new_callable=AsyncMock) as mock_ref:
        mock_renewed.return_value = []
        sent = await send_renewal_notifications()
        assert sent == 0
        mock_send.assert_not_called()


@pytest.mark.asyncio
async def test_notification_loop_keyboard_interrupt():
    with patch("main.asyncio.sleep", side_effect=KeyboardInterrupt), \
         patch("main.send_expiry_notifications", new_callable=AsyncMock), \
         patch("main.send_expired_notifications", new_callable=AsyncMock), \
         patch("main.send_renewal_notifications", new_callable=AsyncMock):
        with pytest.raises(KeyboardInterrupt):
            await notification_loop()
