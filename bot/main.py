import os
import asyncio
import logging
import json
from datetime import datetime, timezone
from io import BytesIO

import httpx
from aiogram import Bot, Dispatcher, types
from aiogram.filters import CommandStart, Command
from aiogram.enums import ParseMode
from aiogram.client.default import DefaultBotProperties
from aiogram.exceptions import TelegramConflictError, TelegramBadRequest
from aiogram.types import (
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    ReplyKeyboardMarkup,
    KeyboardButton,
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

BOT_TOKEN = os.getenv("BOT_TOKEN")
if not BOT_TOKEN:
    raise RuntimeError("BOT_TOKEN is required")

BACKEND_URL = os.getenv("BACKEND_URL", "http://backend:8080")
BOT_API_SECRET = os.getenv("BOT_API_SECRET", "")
WEB_APP_URL = os.getenv("WEB_APP_URL", "https://thenomoreblocks.com")
NOTIFY_INTERVAL = int(os.getenv("NOTIFY_INTERVAL", "3600"))

bot = Bot(token=BOT_TOKEN, default=DefaultBotProperties(parse_mode=ParseMode.HTML))
dp = Dispatcher()
http_client: httpx.AsyncClient | None = None


def api_headers() -> dict:
    return {"X-Bot-Secret": BOT_API_SECRET, "Content-Type": "application/json"}


async def backend_ensure_user(telegram_id: int, first_name: str | None = None) -> dict:
    resp = await http_client.post(
        f"{BACKEND_URL}/api/bot/user",
        headers=api_headers(),
        json={"telegram_id": telegram_id, "first_name": first_name or ""},
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()


async def backend_get_user(telegram_id: int) -> dict | None:
    resp = await http_client.get(
        f"{BACKEND_URL}/api/bot/user/{telegram_id}",
        headers=api_headers(),
        timeout=30,
    )
    if resp.status_code == 404:
        return None
    resp.raise_for_status()
    return resp.json()


async def backend_expiring(hours: int = 72) -> list[dict]:
    resp = await http_client.get(
        f"{BACKEND_URL}/api/bot/notifications/expiring",
        headers=api_headers(),
        params={"hours": hours},
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json().get("users", [])


def main_menu_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="🔑 Купить ключ VPN", callback_data="buy")],
            [InlineKeyboardButton(text="📊 Моя подписка", callback_data="status")],
            [InlineKeyboardButton(text="📖 Инструкция", callback_data="instructions")],
            [InlineKeyboardButton(text="🪝 Открыть WebApp", web_app=types.WebAppInfo(url=WEB_APP_URL))],
        ]
    )


def reply_open_keyboard() -> ReplyKeyboardMarkup:
    return ReplyKeyboardMarkup(
        keyboard=[
            [KeyboardButton(text="🚀 Открыть VPN", web_app=types.WebAppInfo(url=WEB_APP_URL))],
        ],
        resize_keyboard=True,
        one_time_keyboard=False,
    )


def format_config_message(data: dict) -> str:
    lines = ["<b>🔐 Ваш VPN-ключ готов!</b>\n"]
    if data.get("subscription_url"):
        lines.append(f"🔗 <b>Подписка (для Hiddify/всех клиентов):</b>\n<code>{data['subscription_url']}</code>\n")
    if data.get("vless"):
        lines.append(f"🌐 <b>VLESS Reality ссылка:</b>\n<code>{data['vless']}</code>\n")
    lines.append("📦 Sing-box конфиг пришлю отдельным файлом ниже.")
    return "\n".join(lines)


async def deliver_key(message: types.Message, telegram_id: int, first_name: str | None):
    wait = await message.answer("⏳ Создаю ваш ключ, секунду…")
    try:
        data = await backend_ensure_user(telegram_id, first_name)
    except httpx.HTTPStatusError as e:
        logger.error("ensure user failed: %s", e.response.text)
        await wait.delete()
        await message.answer("❌ Не удалось создать ключ. Попробуйте позже или свяжитесь с поддержкой.")
        return
    except Exception as e:  # noqa: BLE001
        logger.exception("ensure user error")
        await wait.delete()
        await message.answer("❌ Ошибка соединения с сервером. Попробуйте позже.")
        return

    await wait.delete()

    if not data.get("provisioned"):
        await message.answer("⚠️ Ключ создан, но подписка ещё не активирована. Попробуйте позже.")
        return

    await message.answer(format_config_message(data))

    if data.get("singbox"):
        file_bytes = data["singbox"].encode("utf-8")
        bio = BytesIO(file_bytes)
        bio.name = "singbox-config.json"
        await message.answer_document(
            types.BufferedInputFile(bio.getvalue(), filename="singbox-config.json"),
            caption="📦 Sing-box конфиг (импорт в приложение Sing-box).",
        )

    await message.answer(
        "✅ Готово! Установите клиент (Hiddify / v2rayNG / Sing-box) и импортируйте подписку или конфиг выше.",
        reply_markup=main_menu_keyboard(),
    )


@dp.message(CommandStart())
async def cmd_start(message: types.Message) -> None:
    await message.answer(
        "<b>Добро пожаловать в NoMoreBlocks VPN! 🛡️</b>\n\n"
        "Я выдаю и доставляю ваши VPN-ключи прямо сюда в Telegram.\n"
        "Нажмите <b>🔑 Купить ключ VPN</b>, чтобы получить конфиг для обхода блокировок.",
        reply_markup=main_menu_keyboard(),
    )


@dp.message(Command("id"))
async def cmd_id(message: types.Message) -> None:
    await message.answer(f"Your Telegram ID: <code>{message.from_user.id}</code>")


@dp.message(Command("status"))
async def cmd_status(message: types.Message) -> None:
    data = await backend_get_user(message.from_user.id)
    if not data or not data.get("provisioned"):
        await message.answer("У вас ещё нет активного ключа. Нажмите 🔑 Купить ключ VPN.", reply_markup=main_menu_keyboard())
        return
    await message.answer(format_config_message(data), reply_markup=main_menu_keyboard())


@dp.message(Command("buy"))
async def cmd_buy(message: types.Message) -> None:
    await deliver_key(message, message.from_user.id, message.from_user.first_name)


@dp.message(Command("notify"))
async def cmd_notify(message: types.Message) -> None:
    await send_expiry_notifications()
    await message.answer("✅ Проверка истекающих подписок выполнена.")


@dp.message(lambda m: m.web_app_data is not None)
async def web_app_data(message: types.Message) -> None:
    data = message.web_app_data.data
    await message.answer(
        f"Получены данные из мини-приложения:\n<pre>{data}</pre>",
        parse_mode=ParseMode.HTML,
    )


@dp.callback_query()
async def callbacks(callback: types.CallbackQuery):
    await callback.answer()
    if callback.data == "buy":
        if callback.message is not None:
            await deliver_key(callback.message, callback.from_user.id, callback.from_user.first_name)
        return
    if callback.data == "status":
        data = await backend_get_user(callback.from_user.id)
        if not data or not data.get("provisioned"):
            text = "У вас ещё нет активного ключа. Нажмите 🔑 Купить ключ VPN."
        else:
            text = format_config_message(data)
        if callback.message is not None:
            await callback.message.answer(text, reply_markup=main_menu_keyboard())
        return
    if callback.data == "instructions":
        text = (
            "📖 <b>Как подключить VPN:</b>\n\n"
            "1. Установите клиент: <b>Hiddify</b> (iOS/Android/Desktop), <b>v2rayNG</b> (Android) или <b>Sing-box</b>.\n"
            "2. Нажмите 🔑 Купить ключ VPN — получите ссылку подписки и конфиги.\n"
            "3. Импортируйте <b>подписку</b> (ссылку) в приложение одним тапом, либо файл Sing-box.\n"
            "4. Включите VPN и наслаждайтесь свободным интернетом. 🚀"
        )
        if callback.message is not None:
            await callback.message.answer(text, reply_markup=main_menu_keyboard())
        return


async def send_expiry_notifications() -> int:
    try:
        expiring = await backend_expiring(hours=72)
    except Exception as e:  # noqa: BLE001
        logger.exception("failed to fetch expiring users: %s", e)
        return 0

    sent = 0
    for item in expiring:
        tg_id = item.get("telegram_id")
        expires_at = item.get("expires_at", 0)
        if not tg_id:
            continue
        when = "скоро"
        if expires_at:
            dt = datetime.fromtimestamp(expires_at / 1000, tz=timezone.utc)
            when = dt.strftime("%d.%m.%Y %H:%M UTC")
        text = (
            "🔔 <b>Напоминание о подписке</b>\n\n"
            f"Ваша подписка истекает <b>{when}</b>.\n"
            "Чтобы не потерять доступ — продлите её заранее через 🔑 Купить ключ VPN."
        )
        try:
            await bot.send_message(tg_id, text, reply_markup=main_menu_keyboard())
            sent += 1
        except TelegramBadRequest as e:
            logger.warning("cannot notify %s: %s", tg_id, e)
    return sent


async def notification_loop() -> None:
    while True:
        await asyncio.sleep(NOTIFY_INTERVAL)
        try:
            sent = await send_expiry_notifications()
            if sent:
                logger.info("sent %d expiry notifications", sent)
        except Exception as e:  # noqa: BLE001
            logger.exception("notification loop error: %s", e)


async def main() -> None:
    global http_client
    http_client = httpx.AsyncClient()
    try:
        asyncio.create_task(notification_loop())
        await dp.start_polling(bot)
    except TelegramConflictError:
        logger.error("TelegramConflictError: another instance is already polling. Stopping.")
        raise SystemExit(0)
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
    finally:
        await http_client.aclose()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
