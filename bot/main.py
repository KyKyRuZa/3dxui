import os
import asyncio
import logging
import json
import signal
import time
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
PRICING_WEB_APP_URL = os.getenv("PRICING_WEB_APP_URL", WEB_APP_URL.rstrip("/") + "/pricing")
NOTIFY_INTERVAL = int(os.getenv("NOTIFY_INTERVAL", "3600"))
MONITORING_CHAT_ID = os.getenv("MONITORING_CHAT_ID", "")
ADMIN_IDS = {int(x) for x in os.getenv("ADMIN_IDS", "").split(",") if x.strip()}

bot = Bot(token=BOT_TOKEN, default=DefaultBotProperties(parse_mode=ParseMode.HTML))
dp = Dispatcher()
http_client: httpx.AsyncClient | None = None
_shutdown = False
notification_task: asyncio.Task | None = None
# Pending referrer codes captured from /start REFCODE, applied on first key purchase.
pending_refs: dict[int, str] = {}
_referral_link_cache: dict[int, tuple[str, float]] = {}
_LINK_CACHE_TTL = 300


def clear_referral_link_cache() -> None:
    _referral_link_cache.clear()


def api_headers() -> dict:
    return {"X-Bot-Secret": BOT_API_SECRET, "Content-Type": "application/json"}


async def send_monitoring_alert(text: str) -> None:
    if not MONITORING_CHAT_ID or not text:
        return
    try:
        await bot.send_message(MONITORING_CHAT_ID, text)
    except TelegramBadRequest:
        pass


async def backend_ensure_user(telegram_id: int, first_name: str | None = None, referral_code: str | None = None) -> dict:
    payload = {"telegram_id": telegram_id, "first_name": first_name or ""}
    if referral_code:
        payload["referral_code"] = referral_code
    resp = await http_client.post(
        f"{BACKEND_URL}/api/bot/user",
        headers=api_headers(),
        json=payload,
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()


async def backend_generate_bind_code(telegram_id: int, code: str) -> bool:
    """Verify a bind code and link Telegram to the user account. Returns True on success."""
    try:
        resp = await http_client.post(
            f"{BACKEND_URL}/api/codes/bind/verify",
            headers=api_headers(),
            json={"code": code, "telegram_id": telegram_id},
            timeout=30,
        )
        return resp.status_code == 200
    except Exception:  # noqa: BLE001
        return False


async def backend_generate_login_code(telegram_id: int) -> str | None:
    """Generate a login code for a Telegram user. Returns the code or None."""
    try:
        resp = await http_client.post(
            f"{BACKEND_URL}/api/codes/login",
            headers=api_headers(),
            json={"telegram_id": telegram_id},
            timeout=30,
        )
        if resp.status_code == 200:
            data = resp.json()
            return data.get("code")
        return None
    except Exception:  # noqa: BLE001
        return None


async def backend_claim_login_token(token: str, telegram_id: int) -> bool:
    """Claim a browser login token for this Telegram user. Returns True on success."""
    try:
        resp = await http_client.post(
            f"{BACKEND_URL}/api/bot/login-token/claim",
            headers=api_headers(),
            json={"token": token, "telegram_id": telegram_id},
            timeout=30,
        )
        return resp.status_code == 200
    except Exception:  # noqa: BLE001
        return False


async def backend_referral(telegram_id: int) -> dict | None:
    try:
        resp = await http_client.post(
            f"{BACKEND_URL}/api/bot/referral",
            headers=api_headers(),
            json={"telegram_id": telegram_id},
            timeout=30,
        )
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        return resp.json()
    except httpx.HTTPStatusError as e:
        logger.error("backend_referral failed: %s", e.response.status_code)
        return None
    except Exception as e:  # noqa: BLE001
        logger.exception("backend_referral error: %s", e)
        return None


async def referral_link(telegram_id: int) -> str | None:
    """Return this user's referral deep link, or None if unavailable."""
    cached = _referral_link_cache.get(telegram_id)
    if cached and time.monotonic() - cached[1] < _LINK_CACHE_TTL:
        return cached[0]
    try:
        data = await backend_referral(telegram_id)
    except Exception:  # noqa: BLE001
        return None
    if not data or not data.get("referral_code"):
        return None
    try:
        me = await bot.get_me()
    except Exception:  # noqa: BLE001
        return None
    link = f"https://t.me/{me.username}?start={data['referral_code']}"
    _referral_link_cache[telegram_id] = (link, time.monotonic())
    return link


def referral_anchor(link: str | None) -> str:
    """A pushy referral upsell appended to sales messages."""
    if not link:
        return ""
    return (
        "\n\n🤝 <b>Не хотите платить?</b> Пригласите друга по вашей ссылке и получите "
        f"<b>+7 дней бесплатно</b> за каждую его покупку тарифа:\n<code>{link}</code>"
    )


async def backend_get_user(telegram_id: int) -> dict | None:
    try:
        resp = await http_client.get(
            f"{BACKEND_URL}/api/bot/user/{telegram_id}",
            headers=api_headers(),
            timeout=30,
        )
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        return resp.json()
    except httpx.HTTPStatusError as e:
        logger.error("backend_get_user failed: %s", e.response.status_code)
        return None
    except Exception as e:  # noqa: BLE001
        logger.exception("backend_get_user error: %s", e)
        return None


async def backend_expiring(hours: int = 72) -> list[dict]:
    try:
        resp = await http_client.get(
            f"{BACKEND_URL}/api/bot/notifications/expiring",
            headers=api_headers(),
            params={"hours": hours},
            timeout=30,
        )
        resp.raise_for_status()
        return resp.json().get("users", [])
    except httpx.HTTPStatusError as e:
        logger.error("backend_expiring failed: %s", e.response.status_code)
        return []
    except Exception as e:  # noqa: BLE001
        logger.exception("backend_expiring error: %s", e)
        return []


async def backend_expired(hours: int = 24) -> list[dict]:
    try:
        resp = await http_client.get(
            f"{BACKEND_URL}/api/bot/notifications/expired",
            headers=api_headers(),
            params={"hours": hours},
            timeout=30,
        )
        resp.raise_for_status()
        return resp.json().get("users", [])
    except httpx.HTTPStatusError as e:
        logger.error("backend_expired failed: %s", e.response.status_code)
        return []
    except Exception as e:  # noqa: BLE001
        logger.exception("backend_expired error: %s", e)
        return []


async def backend_renewed() -> list[dict]:
    try:
        resp = await http_client.get(
            f"{BACKEND_URL}/api/bot/notifications/renewed",
            headers=api_headers(),
            timeout=30,
        )
        resp.raise_for_status()
        return resp.json().get("users", [])
    except httpx.HTTPStatusError as e:
        logger.error("backend_renewed failed: %s", e.response.status_code)
        return []
    except Exception as e:  # noqa: BLE001
        logger.exception("backend_renewed error: %s", e)
        return []


async def backend_notifications() -> list[dict]:
    try:
        resp = await http_client.get(
            f"{BACKEND_URL}/api/bot/notifications/pending",
            headers=api_headers(),
            timeout=30,
        )
        resp.raise_for_status()
        return resp.json().get("notifications", [])
    except httpx.HTTPStatusError as e:
        logger.error("backend_notifications failed: %s", e.response.status_code)
        return []
    except Exception as e:  # noqa: BLE001
        logger.exception("backend_notifications error: %s", e)
        return []


def main_menu_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="🛒 Купить тариф", web_app=types.WebAppInfo(url=PRICING_WEB_APP_URL))],
            [InlineKeyboardButton(text="🔑 Получить пробный ключ", callback_data="buy")],
            [InlineKeyboardButton(text="📊 Моя подписка", callback_data="status")],
            [InlineKeyboardButton(text="🤝 Реферальная программа", callback_data="referral")],
            [InlineKeyboardButton(text="🔧 Починить доступ к сайту", callback_data="fix")],
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


FIX_WINDOWS_PATH = "/app/add_hosts_windows.bat"
FIX_MACOS_LINUX_PATH = "/app/add_hosts_linux_macos.sh"


def fix_os_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="🪟 Windows", callback_data="fix_windows")],
            [InlineKeyboardButton(text="🍎 macOS", callback_data="fix_macos")],
            [InlineKeyboardButton(text="🐧 Linux", callback_data="fix_linux")],
        ]
    )


async def send_hosts_script(message: types.Message | None, path: str, caption: str) -> bool:
    try:
        with open(path, "rb") as f:
            data = f.read()
    except Exception:
        if message is not None:
            await message.answer("❌ Файл не найден. Свяжитесь с поддержкой.")
        return False
    if message is not None:
        await message.answer_document(
            types.BufferedInputFile(data, filename=os.path.basename(path)),
            caption=caption,
        )
    return True


def format_expiry(data: dict) -> str | None:
    exp = data.get("expires_at")
    if not exp:
        return None
    now = datetime.now(timezone.utc)
    dt = datetime.fromtimestamp(exp / 1000, tz=timezone.utc)
    days = (dt - now).total_seconds() / 86400
    if days >= 1:
        return f"⏳ Подписка активна до <b>{dt.strftime('%d.%m.%Y %H:%M UTC')}</b> (осталось ~{days:.0f} дн.)"
    if days > 0:
        return f"⏳ Подписка активна до <b>{dt.strftime('%d.%m.%Y %H:%M UTC')}</b> (осталось менее суток)"
    return f"🚨 <b>Подписка истекла {dt.strftime('%d.%m.%Y %H:%M UTC')}!</b> Вы снова без защиты. Нажмите 🔑 Купить ключ VPN прямо сейчас — и вернёте доступ за минуту."


def days_left_from_expiry(exp: int | None) -> int | None:
    if not exp:
        return None
    now = datetime.now(timezone.utc)
    dt = datetime.fromtimestamp(exp / 1000, tz=timezone.utc)
    return int((dt - now).total_seconds() / 86400)


async def get_user_menu_keyboard(telegram_id: int) -> InlineKeyboardMarkup:
    data = await backend_get_user(telegram_id)
    sub_buttons: list[list[InlineKeyboardButton]] = []
    top_buttons: list[InlineKeyboardButton] = []
    days_text = ""
    if data and data.get("provisioned") and data.get("expires_at"):
        days = days_left_from_expiry(data.get("expires_at"))
        if days is not None and days >= 0:
            days_text = f" (осталось {days} дн.)" if days > 0 else " (истекла!)"
        if days is None or days < 0:
            top_buttons = [
                InlineKeyboardButton(text="🛒 Купить тариф", web_app=types.WebAppInfo(url=PRICING_WEB_APP_URL)),
                InlineKeyboardButton(text="🔑 Получить пробный ключ", callback_data="buy"),
            ]
        else:
            top_buttons = [
                InlineKeyboardButton(text="📊 Моя подписка" + days_text, callback_data="status"),
                InlineKeyboardButton(text="🛒 Купить тариф", web_app=types.WebAppInfo(url=PRICING_WEB_APP_URL)),
            ]
    else:
        top_buttons = [
            InlineKeyboardButton(text="🛒 Купить тариф", web_app=types.WebAppInfo(url=PRICING_WEB_APP_URL)),
            InlineKeyboardButton(text="🔑 Получить пробный ключ", callback_data="buy"),
        ]
        sub_buttons = [
            [InlineKeyboardButton(text="📊 Моя подписка", callback_data="status")],
        ]

    return InlineKeyboardMarkup(
        inline_keyboard=[
            *([btn] for btn in top_buttons),
            *sub_buttons,
            [InlineKeyboardButton(text="🤝 Реферальная программа", callback_data="referral")],
            [InlineKeyboardButton(text="🔧 Починить доступ к сайту", callback_data="fix")],
            [InlineKeyboardButton(text="📖 Инструкция", callback_data="instructions")],
            [InlineKeyboardButton(text="🪝 Открыть WebApp", web_app=types.WebAppInfo(url=WEB_APP_URL))],
        ]
    )


def format_config_message(data: dict) -> str:
    lines = ["<b>🔐 Ваш VPN-ключ готов!</b>\n"]
    if data.get("subscription_url"):
        lines.append(f"🔗 <b>Подписка (для Hiddify/всех клиентов):</b>\n<code>{data['subscription_url']}</code>\n")
    vless = data.get("vless")
    if not vless:
        for link in data.get("links", []):
            if link.startswith("vless://"):
                vless = link
                break
    if vless:
        lines.append(f"🌐 <b>VLESS Reality ссылка:</b>\n<code>{vless}</code>\n")
    exp = format_expiry(data)
    if exp:
        lines.append(exp + "\n")
    if data.get("singbox"):
        lines.append("📦 Sing-box конфиг пришлю отдельным файлом ниже.")
    return "\n".join(lines)


async def deliver_key(message: types.Message, telegram_id: int, first_name: str | None):
    wait = await message.answer("⏳ Создаю ваш ключ, секунду…")
    try:
        ref = pending_refs.pop(telegram_id, None)
        data = await backend_ensure_user(telegram_id, first_name, ref)
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
        "✅ Готово! Установите клиент <b>Happ</b> и импортируйте подписку или конфиг выше. "
        "В Happ зайдите в <b>Настройки → ПИНГ</b> и выберите <b>TCP</b> для стабильной скорости.\n\n"
        "💡 Если вы на WiFi и сайт не открывается — включите мобильный хотспот, выполните эти шаги, "
        "а потом вернитесь на WiFi. Сам VPN работает на любом соединении.\n\n"
        "🔥 <b>Нужно больше дней без ограничений?</b> Нажмите <b>🛒 Купить тариф</b> — подписка активируется сразу после оплаты.\n"
        "🤝 <b>Хотите бесплатные дни?</b> Пригласите друга по реферальной ссылке — вы получите +7 дней за каждую его покупку.",
        reply_markup=await get_user_menu_keyboard(telegram_id),
    )


@dp.message(CommandStart())
async def cmd_start(message: types.Message) -> None:
    parts = message.text.split(maxsplit=1)
    if len(parts) > 1 and parts[1].strip():
        param = parts[1].strip()

        # Handle bind code: /start bind-XXXXXXXX
        if param.startswith("bind-") and len(param) > 5:
            code = param[5:].strip()
            if await backend_generate_bind_code(message.from_user.id, code):
                await message.answer(
                    "✅ <b>Telegram привязан к аккаунту!</b>\n\n"
                    "Теперь вы можете входить на сайт через код из бота.\n"
                    "Используйте /link для получения кода входа.",
                    reply_markup=await get_user_menu_keyboard(message.from_user.id),
                )
            else:
                await message.answer(
                    "❌ <b>Не удалось привязать Telegram.</b>\n\n"
                    "Код недействителен или истёк. Получите новый код на сайте.",
                    reply_markup=await get_user_menu_keyboard(message.from_user.id),
                )
            return

        # A 32-char hex token is a browser login deep link; claim it for this user.
        if len(param) == 32 and all(c in "0123456789abcdef" for c in param.lower()):
            if await backend_claim_login_token(param, message.from_user.id):
                await message.answer(
                    "✅ <b>Вход подтверждён!</b>\n\n"
                    "Вернитесь на сайт — вы уже авторизованы. Можно закрыть это окно.",
                    reply_markup=await get_user_menu_keyboard(message.from_user.id),
                )
                return
        else:
            pending_refs[message.from_user.id] = param
    await message.answer(
        "<b>Добро пожаловать в Walyny4 vpn! 🛡️</b>\n\n"
        "Получите готовый VPN-конфиг за минуту — обходите блокировки, сохраняйте приватность и возвращайте доступ к нужным сайтам.\n\n"
        "🛒 <b>Хотите полный доступ без ограничений?</b> Нажмите <b>Купить тариф</b> — подписка активируется автоматически после оплаты.\n"
        "🔑 Или начните с <b>пробного ключа</b> — он выдаётся бесплатно на 2 дня.\n\n"
        "💡 Если на WiFi не открывается сайт — включите мобильный хотспот и получите ключ через мобильную сеть. Сам VPN работает на любом соединении.",
        reply_markup=await get_user_menu_keyboard(message.from_user.id),
    )


@dp.message(Command("referral"))
async def cmd_referral(message: types.Message) -> None:
    data = await backend_referral(message.from_user.id)
    if not data or not data.get("referral_code"):
        await message.answer("Реферальная программа пока недоступна.", reply_markup=await get_user_menu_keyboard(message.from_user.id))
        return
    me = await bot.get_me()
    link = f"https://t.me/{me.username}?start={data['referral_code']}"
    await message.answer(
        f"🤝 <b>Реферальная программа</b>\n\n"
        f"Ваша ссылка: <code>{link}</code>\n"
        f"Приглашено: <b>{data['invited']}</b>\n"
        f"Начислено бонусных дней: <b>{data['earned_days']}</b>\n\n"
        f"За каждого друга, купившего платный тариф, вы получите +7 дней к подписке. "
        f"Друг, перешедший по ссылке, получает бонус к пробному периоду.",
        reply_markup=await get_user_menu_keyboard(message.from_user.id),
    )


@dp.message(Command("id"))
async def cmd_id(message: types.Message) -> None:
    await message.answer(f"Ваш Telegram ID: <code>{message.from_user.id}</code>")


@dp.message(Command("status"))
async def cmd_status(message: types.Message) -> None:
    data = await backend_get_user(message.from_user.id)
    if not data or not data.get("provisioned"):
        link = await referral_link(message.from_user.id)
        await message.answer(
            "🚨 <b>У вас нет активного VPN-ключа — вы уже в изоляции.</b>\n\n"
            "Пока без защиты, нужные сайты и сервисы для вас закрыты, а о ваших действиях известно больше, "
            "чем стоило бы.\n\n"
            "⚡ <b>Исправьте это одним тапом:</b> нажмите <b>🔑 Купить ключ VPN</b> — и через минуту у вас будет "
            "готовый конфиг для обхода блокировок (Hiddify / v2rayNG / Sing-box).\n\n"
            "🔥 Не откладывайте — каждый час без VPN это упущенная свобода. Верните доступ прямо сейчас!\n\n"
            "💡 Если на WiFi не открывается сайт — включите мобильный хотспот и получите ключ через мобильную сеть."
            + referral_anchor(link),
            reply_markup=await get_user_menu_keyboard(message.from_user.id),
        )
        return
    await message.answer(format_config_message(data), reply_markup=await get_user_menu_keyboard(message.from_user.id))


@dp.message(Command("buy"))
async def cmd_buy(message: types.Message) -> None:
    await deliver_key(message, message.from_user.id, message.from_user.first_name)


@dp.message(Command("notify"))
async def cmd_notify(message: types.Message) -> None:
    if not ADMIN_IDS or message.from_user.id not in ADMIN_IDS:
        await message.answer("❌ Эта команда доступна только администратору.")
        return
    await send_expiry_notifications()
    await message.answer("✅ Проверка истекающих подписок выполнена.")


@dp.message(Command("fix"))
async def cmd_fix(message: types.Message) -> None:
    await message.answer(
        "🔧 <b>Починить доступ к сайту</b>\n\n"
        "Сейчас у некоторых провайдеров не работает DNS для нашего сайта. "
        "Чтобы открыть сайт без VPN, подскажите компьютеру правильный IP через файл hosts.\n\n"
        "💡 <b>Совет:</b> если вы сейчас на WiFi и сайт не открывается — включите мобильный интернет/хотспот, "
        "выполните первоначальные действия (получите ключ, установите Happ, импортируйте конфиг), "
        "а потом вернитесь на WiFi. VPN-туннель работает на любом соединении.\n\n"
        "Выберите вашу систему:",
        reply_markup=fix_os_keyboard(),
    )


@dp.message(Command("link"))
async def cmd_link(message: types.Message) -> None:
    """Generate a login code for the user to authenticate on the website."""
    code = await backend_generate_login_code(message.from_user.id)
    if code:
        await message.answer(
            f"🔐 <b>Код для входа на сайт:</b> <code>{code}</code>\n\n"
            "Введите этот код на сайте в разделе «Войти через Telegram».\n"
            "Код действителен 5 минут.",
            reply_markup=await get_user_menu_keyboard(message.from_user.id),
        )
    else:
        await message.answer(
            "❌ Не удалось сгенерировать код. Убедитесь, что вы начали диалог с ботом.",
            reply_markup=await get_user_menu_keyboard(message.from_user.id),
        )


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
    if callback.data == "referral":
        data = await backend_referral(callback.from_user.id)
        if not data or not data.get("referral_code"):
            text = "Реферальная программа пока недоступна."
        else:
            me = await bot.get_me()
            link = f"https://t.me/{me.username}?start={data['referral_code']}"
            text = (
                f"🤝 <b>Реферальная программа</b>\n\n"
                f"Ваша ссылка: <code>{link}</code>\n"
                f"Приглашено: <b>{data['invited']}</b>\n"
                f"Начислено бонусных дней: <b>{data['earned_days']}</b>\n\n"
                f"За каждого друга, купившего платный тариф, вы получите +7 дней к подписке."
            )
        if callback.message is not None:
            await callback.message.answer(text, reply_markup=await get_user_menu_keyboard(callback.from_user.id))
        return
    if callback.data == "status":
        data = await backend_get_user(callback.from_user.id)
        if not data or not data.get("provisioned"):
            link = await referral_link(callback.from_user.id)
            text = (
                "🚨 <b>У вас нет активного VPN-ключа — вы уже в изоляции.</b>\n\n"
                "Пока без защиты, нужные сайты и сервисы для вас закрыты, а о ваших действиях известно больше, "
                "чем стоило бы.\n\n"
                "⚡ <b>Исправьте это одним тапом:</b> нажмите <b>🔑 Купить ключ VPN</b> — и через минуту у вас будет "
                "готовый конфиг для обхода блокировок (Hiddify / v2rayNG / Sing-box).\n\n"
                "🔥 Не откладывайте — каждый час без VPN это упущенная свобода. Верните доступ прямо сейчас!\n\n"
                "💡 Если на WiFi не открывается сайт — включите мобильный хотспот и получите ключ через мобильную сеть."
                + referral_anchor(link)
            )
        else:
            text = format_config_message(data)
        if callback.message is not None:
            await callback.message.answer(text, reply_markup=await get_user_menu_keyboard(callback.from_user.id))
        return
    if callback.data == "fix":
        if callback.message is not None:
            await callback.message.answer(
                "🔧 <b>Починить доступ к сайту</b>\n\n"
                "Сейчас у некоторых провайдеров не работает DNS для нашего сайта. "
                "Чтобы открыть сайт без VPN, подскажите компьютеру правильный IP через файл hosts.\n\n"
                "Выберите вашу систему:",
                reply_markup=fix_os_keyboard(),
            )
        return
    if callback.data == "fix_windows":
        caption = (
            "🪟 <b>Windows</b>\n\n"
            "1. Сохрани файл.\n"
            "2. Нажми правой кнопкой → «Запуск от имени администратора».\n"
            "3. Если спросит обновление — нажми Y.\n"
            "4. Дождись надписи «ГОТОВО!».\n\n"
            "Если не помогло — отключи VPN/прокси и перезагрузи компьютер."
        )
        ok = await send_hosts_script(callback.message, FIX_WINDOWS_PATH, caption)
        if ok and callback.message is not None:
            await callback.message.answer("✅ Готово!", reply_markup=await get_user_menu_keyboard(callback.from_user.id))
        return
    if callback.data == "fix_macos":
        caption = (
            "🍎 <b>macOS</b>\n\n"
            "1. Сохрани файл.\n"
            "2. Открой терминал, перейди в папку со файлом.\n"
            "3. Выполни: <code>chmod +x add_hosts_linux_macos.sh</code>\n"
            "4. Затем: <code>sudo ./add_hosts_linux_macos.sh</code>\n"
            "5. Введи пароль администратора.\n\n"
            "Если не помогло — отключи VPN/прокси и перезагрузи компьютер."
        )
        ok = await send_hosts_script(callback.message, FIX_MACOS_LINUX_PATH, caption)
        if ok and callback.message is not None:
            await callback.message.answer("✅ Готово!", reply_markup=await get_user_menu_keyboard(callback.from_user.id))
        return
    if callback.data == "fix_linux":
        caption = (
            "🐧 <b>Linux</b>\n\n"
            "1. Сохрани файл.\n"
            "2. Открой терминал, перейди в папку со файлом.\n"
            "3. Выполни: <code>chmod +x add_hosts_linux_macos.sh</code>\n"
            "4. Затем: <code>sudo ./add_hosts_linux_macos.sh</code>\n"
            "5. Введи пароль администратора.\n\n"
            "Если не помогло — перезагрузи систему."
        )
        ok = await send_hosts_script(callback.message, FIX_MACOS_LINUX_PATH, caption)
        if ok and callback.message is not None:
            await callback.message.answer("✅ Готово!", reply_markup=await get_user_menu_keyboard(callback.from_user.id))
        return
    if callback.data == "instructions":
        text = (
            "📖 <b>Как подключить VPN (Happ):</b>\n\n"
            "1. Установите клиент <b>Happ</b> на ваше устройство.\n"
            "2. Нажмите 🔑 Купить ключ VPN — получите ссылку подписки и конфиги.\n"
            "3. Импортируйте подписку (ссылку) в Happ одним тапом, либо файл Sing-box.\n"
            "4. В Happ откройте <b>Настройки → ПИНГ</b> и выберите <b>TCP</b> — это даст стабильную скорость.\n"
            "5. Включите VPN и наслаждайтесь свободным интернетом. 🚀\n\n"
            "💡 <b>Важно:</b> если на WiFi не открывается сайт — включите мобильный интернет/хотспот, "
            "выполните эти шаги, а потом вернитесь на WiFi. Туннель работает на любом соединении."
        )
        if callback.message is not None:
            await callback.message.answer(text, reply_markup=await get_user_menu_keyboard(callback.from_user.id))
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
        left = ""
        if expires_at:
            dt = datetime.fromtimestamp(expires_at / 1000, tz=timezone.utc)
            when = dt.strftime("%d.%m.%Y %H:%M UTC")
            days = (dt - datetime.now(timezone.utc)).total_seconds() / 86400
            left = f" (осталось ~{days:.0f} дн.)" if days >= 1 else " (осталось менее суток)"
        # Last-day / final-chance wording: more urgent when it's within a day.
        if days < 1:
            text = (
                "⏰ <b>Последний день вашей подписки!</b>\n\n"
                f"Доступ закроется уже <b>{when}</b>. После этого вы снова окажетесь за блокировками — "
                "и вернётесь к ним не сами по себе.\n\n"
                "🔥 <b>Продлите прямо сейчас:</b> нажмите <b>🔑 Купить ключ VPN</b> — это займёт меньше минуты "
                "и сохранит ваш доступ без перерыва. Не дайте себе оказаться в изоляции."
            )
        else:
            text = (
                "🔔 <b>Напоминание о подписке</b>\n\n"
                f"Ваша подписка истекает <b>{when}</b>{left}.\n"
                "Чтобы не потерять доступ — продлите её через 🔑 Купить ключ VPN."
            )
        text += referral_anchor(await referral_link(tg_id))
        try:
            await bot.send_message(tg_id, text, reply_markup=await get_user_menu_keyboard(tg_id))
            sent += 1
        except TelegramBadRequest as e:
            logger.warning("cannot notify %s: %s", tg_id, e)
    return sent


async def send_expired_notifications() -> int:
    try:
        expired = await backend_expired(hours=168)
    except Exception as e:  # noqa: BLE001
        logger.exception("failed to fetch expired users: %s", e)
        return 0

    sent = 0
    for item in expired:
        tg_id = item.get("telegram_id")
        expires_at = item.get("expires_at", 0)
        if not tg_id:
            continue
        when = ""
        if expires_at:
            dt = datetime.fromtimestamp(expires_at / 1000, tz=timezone.utc)
            when = dt.strftime("%d.%m.%Y %H:%M UTC")
        text = (
            "🚨 <b>Ваша подписка истекла.</b>\n\n"
            "Доступ к VPN сейчас отключён, но вернуть его очень просто.\n\n"
            "⚡ <b>Действуйте:</b>\n"
            "• <b>🛒 Купить тариф</b> — платёж через ЮKassa, подписка активируется автоматически.\n"
            "• <b>🔑 Пробный ключ</b> — получите бесплатный доступ на 2 дня, чтобы быстро вернуться в сеть.\n\n"
            "💡 Если на WiFi не открывается сайт — включите мобильный хотспот и выполните покупку через мобильную сеть."
        )
        text += referral_anchor(await referral_link(tg_id))
        try:
            await bot.send_message(tg_id, text, reply_markup=await get_user_menu_keyboard(tg_id))
            sent += 1
        except TelegramBadRequest as e:
            logger.warning("cannot notify %s: %s", tg_id, e)
    return sent


async def send_retargeting_notifications() -> int:
    try:
        expired = await backend_expired(hours=96)
    except Exception as e:  # noqa: BLE001
        logger.exception("failed to fetch retargeting users: %s", e)
        return 0

    now = datetime.now(timezone.utc)
    sent = 0
    for item in expired:
        tg_id = item.get("telegram_id")
        expires_at = item.get("expires_at", 0)
        if not tg_id or not expires_at:
            continue
        dt = datetime.fromtimestamp(expires_at / 1000, tz=timezone.utc)
        hours_since_expiry = (now - dt).total_seconds() / 3600
        if hours_since_expiry < 72 or hours_since_expiry > 96:
            continue
        text = (
            "🥹 <b>Мы скучаем по вам!</b>\n\n"
            "Ваш VPN до сих пор отключён, а мы уже подготовили для вас бонус.\n\n"
            "🎁 <b>Вернитесь и получите +2 дня бесплатно</b> при покупке любого тарифа.\n"
            "Это наш подарок за то, что вы снова с нами.\n\n"
            "🛒 Нажмите <b>Купить тариф</b> — бонус спишется автоматически."
        )
        text += referral_anchor(await referral_link(tg_id))
        try:
            await bot.send_message(tg_id, text, reply_markup=await get_user_menu_keyboard(tg_id))
            sent += 1
        except TelegramBadRequest as e:
            logger.warning("cannot notify %s: %s", tg_id, e)
    return sent


async def send_renewal_notifications() -> int:
    try:
        renewed = await backend_renewed()
    except Exception as e:  # noqa: BLE001
        logger.exception("failed to fetch renewed users: %s", e)
        return 0

    sent = 0
    for item in renewed:
        tg_id = item.get("telegram_id")
        expires_at = item.get("expires_at", 0)
        if not tg_id:
            continue
        when = ""
        if expires_at:
            dt = datetime.fromtimestamp(expires_at / 1000, tz=timezone.utc)
            when = dt.strftime("%d.%m.%Y %H:%M UTC")
        text = (
            "✅ <b>Подписка продлена!</b>\n\n"
            f"Новый срок действия — до <b>{when}</b>. "
            "Ваш VPN-ключ активен, доступ восстановлен.\n\n"
            "Если конфиг изменился — нажмите 🔑 Купить ключ VPN, чтобы получить свежие данные."
        )
        text += referral_anchor(await referral_link(tg_id))
        try:
            await bot.send_message(tg_id, text, reply_markup=await get_user_menu_keyboard(tg_id))
            sent += 1
        except TelegramBadRequest as e:
            logger.warning("cannot notify %s: %s", tg_id, e)
    return sent


def render_bot_notification(kind: str, data: dict) -> str | None:
    """Build the message text for a generic bot notification by its kind."""
    if kind == "referral_signup":
        name = (data.get("friend_name") or "").strip()
        who = f" <b>{name}</b>" if name else ""
        return (
            "🤝 По вашей реферальной ссылке зарегистрировался друг{who}!\n\n"
            "Когда он купит платный тариф, вы получите <b>+7 дней</b> к подписке бесплатно. "
            "Делитесь ссылкой дальше и приглашайте ещё больше друзей!"
        ).format(who=who)
    if kind == "referral_reward":
        days = data.get("reward_days", 7)
        return (
            f"🎁 <b>Промокод активирован!</b>\n\n"
            f"Вам начислено <b>+{days} дней</b> по реферальной ссылке. "
            "Ваша подписка продлена. Спасибо, что приводите друзей! 🚀"
        )
    if kind == "referral_paid_bonus":
        days = data.get("reward_days", 2)
        return (
            f"🎁 <b>Вам начислено +{days} дней за реферальный бонус!</b>\n\n"
            "Вы перешли по реферальной ссылке и оплатили тариф — бонус зачислен. "
            "Ваша подписка продлена. Спасибо, что.join нас! 🚀"
        )
    if kind == "payment_failed":
        return (
            "❌ <b>Оплата не прошла</b>\n\n"
            "К сожалению, платёж за тариф не был завершён (отменён или отклонён). "
            "Попробуйте оплатить ещё раз через 🔑 Купить ключ VPN — и доступ откроется сразу."
        )
    return None


async def send_bot_notifications() -> int:
    try:
        notifs = await backend_notifications()
    except Exception as e:  # noqa: BLE001
        logger.exception("failed to fetch bot notifications: %s", e)
        return 0

    sent = 0
    for item in notifs:
        tg_id = item.get("telegram_id")
        kind = item.get("kind")
        data = item.get("data") or {}
        if not tg_id or not kind:
            continue
        text = render_bot_notification(kind, data)
        if not text:
            continue
        try:
            await bot.send_message(tg_id, text, reply_markup=await get_user_menu_keyboard(tg_id))
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
            sent_expired = await send_expired_notifications()
            if sent_expired:
                logger.info("sent %d expired-subscription nudges", sent_expired)
            sent_retarget = await send_retargeting_notifications()
            if sent_retarget:
                logger.info("sent %d retargeting notifications", sent_retarget)
            sent_renewed = await send_renewal_notifications()
            if sent_renewed:
                logger.info("sent %d renewal notifications", sent_renewed)
            sent_bot = await send_bot_notifications()
            if sent_bot:
                logger.info("sent %d bot notifications", sent_bot)
        except Exception as e:  # noqa: BLE001
            logger.exception("notification loop error: %s", e)
            await send_monitoring_alert(f"❌ Notification loop error: {e}")


async def main() -> None:
    global http_client, _shutdown, notification_task
    http_client = httpx.AsyncClient()
    loop = asyncio.get_running_loop()

    def _signal_handler() -> None:
        global _shutdown
        _shutdown = True
        loop.call_soon_threadsafe(lambda: asyncio.create_task(dp.stop_polling()))

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _signal_handler)
        except NotImplementedError:
            pass

    try:
        notification_task = asyncio.create_task(notification_loop())
        await dp.start_polling(bot)
    except TelegramConflictError:
        logger.error("TelegramConflictError: another instance is already polling. Stopping.")
        raise SystemExit(0)
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
    finally:
        _shutdown = True
        if notification_task is not None:
            notification_task.cancel()
            try:
                await notification_task
            except asyncio.CancelledError:
                pass
        try:
            await send_monitoring_alert("⛔ Bot stopped")
        except Exception:  # noqa: BLE001
            pass
        try:
            await http_client.aclose()
        except Exception:  # noqa: BLE001
            pass
        try:
            await bot.session.close()
        except Exception:  # noqa: BLE001
            pass


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
