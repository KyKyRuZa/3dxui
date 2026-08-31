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
# Pending referrer codes captured from /start REFCODE, applied on first key purchase.
pending_refs: dict[int, str] = {}


def api_headers() -> dict:
    return {"X-Bot-Secret": BOT_API_SECRET, "Content-Type": "application/json"}


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


async def backend_referral(telegram_id: int) -> dict | None:
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


async def referral_link(telegram_id: int) -> str | None:
    """Return this user's referral deep link, or None if unavailable."""
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
    return f"https://t.me/{me.username}?start={data['referral_code']}"


def referral_anchor(link: str | None) -> str:
    """A pushy referral upsell appended to sales messages."""
    if not link:
        return ""
    return (
        "\n\n🤝 <b>Не хотите платить?</b> Пригласите друга по вашей ссылке и получите "
        f"<b>+7 дней бесплатно</b> за каждую его покупку тарифа:\n<code>{link}</code>"
    )


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


async def backend_expired(hours: int = 24) -> list[dict]:
    resp = await http_client.get(
        f"{BACKEND_URL}/api/bot/notifications/expired",
        headers=api_headers(),
        params={"hours": hours},
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json().get("users", [])


async def backend_renewed() -> list[dict]:
    resp = await http_client.get(
        f"{BACKEND_URL}/api/bot/notifications/renewed",
        headers=api_headers(),
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json().get("users", [])


async def backend_notifications() -> list[dict]:
    resp = await http_client.get(
        f"{BACKEND_URL}/api/bot/notifications/pending",
        headers=api_headers(),
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json().get("notifications", [])


def main_menu_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="🔑 Купить ключ VPN", callback_data="buy")],
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


async def send_hosts_script(message: types.Message | None, path: str, caption: str) -> None:
    try:
        with open(path, "rb") as f:
            data = f.read()
    except Exception:
        if message is not None:
            await message.answer("❌ Файл не найден. Свяжитесь с поддержкой.")
        return
    if message is not None:
        await message.answer_document(
            types.BufferedInputFile(data, filename=os.path.basename(path)),
            caption=caption,
        )


def format_expiry(data: dict) -> str | None:
    exp = data.get("expires_at")
    if not exp:
        return None
    now = datetime.now(timezone.utc)
    dt = datetime.fromtimestamp(exp / 1000, tz=timezone.utc)
    days = (dt - now).total_seconds() / 86400
    if days > 1:
        return f"⏳ Подписка активна до <b>{dt.strftime('%d.%m.%Y %H:%M UTC')}</b> (осталось ~{days:.0f} дн.)"
    if days > 0:
        return f"⏳ Подписка активна до <b>{dt.strftime('%d.%m.%Y %H:%M UTC')}</b> (осталось менее суток)"
    return f"🚨 <b>Подписка истекла {dt.strftime('%d.%m.%Y %H:%M UTC')}!</b> Вы снова без защиты. Нажмите 🔑 Купить ключ VPN прямо сейчас — и вернёте доступ за минуту."


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
        "а потом вернитесь на WiFi. Сам VPN работает на любом соединении.",
        reply_markup=main_menu_keyboard(),
    )


@dp.message(CommandStart())
async def cmd_start(message: types.Message) -> None:
    parts = message.text.split(maxsplit=1)
    if len(parts) > 1 and parts[1].strip():
        pending_refs[message.from_user.id] = parts[1].strip()
    await message.answer(
        "<b>Добро пожаловать в NoMoreBlocks VPN! 🛡️</b>\n\n"
        "Я выдаю и доставляю ваши VPN-ключи прямо сюда в Telegram.\n"
        "Нажмите <b>🔑 Купить ключ VPN</b>, чтобы получить конфиг для обхода блокировок.",
        reply_markup=main_menu_keyboard(),
    )


@dp.message(Command("referral"))
async def cmd_referral(message: types.Message) -> None:
    data = await backend_referral(message.from_user.id)
    if not data or not data.get("referral_code"):
        await message.answer("Реферальная программа пока недоступна.", reply_markup=main_menu_keyboard())
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
        reply_markup=main_menu_keyboard(),
    )


@dp.message(Command("id"))
async def cmd_id(message: types.Message) -> None:
    await message.answer(f"Your Telegram ID: <code>{message.from_user.id}</code>")


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
            reply_markup=main_menu_keyboard(),
        )
        return
    await message.answer(format_config_message(data), reply_markup=main_menu_keyboard())


@dp.message(Command("buy"))
async def cmd_buy(message: types.Message) -> None:
    await deliver_key(message, message.from_user.id, message.from_user.first_name)


@dp.message(Command("notify"))
async def cmd_notify(message: types.Message) -> None:
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
            await callback.message.answer(text, reply_markup=main_menu_keyboard())
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
            await callback.message.answer(text, reply_markup=main_menu_keyboard())
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
        await send_hosts_script(callback.message, FIX_WINDOWS_PATH, caption)
        if callback.message is not None:
            await callback.message.answer("✅ Готово!", reply_markup=main_menu_keyboard())
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
        await send_hosts_script(callback.message, FIX_MACOS_LINUX_PATH, caption)
        if callback.message is not None:
            await callback.message.answer("✅ Готово!", reply_markup=main_menu_keyboard())
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
        await send_hosts_script(callback.message, FIX_MACOS_LINUX_PATH, caption)
        if callback.message is not None:
            await callback.message.answer("✅ Готово!", reply_markup=main_menu_keyboard())
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
            await bot.send_message(tg_id, text, reply_markup=main_menu_keyboard())
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
            "🚨 <b>Доступ перекрыт — вы снова в изоляции.</b>\n\n"
            f"С <b>{when}</b> нужные сайты и сервисы для вас снова закрыты, а каждый час без защиты — "
            "это упущенная свобода и лишние риски.\n\n"
            "⚡ <b>Верните всё одним тапом:</b> нажмите <b>🔑 Купить ключ VPN</b> — и через минуту у вас будет "
            "свежий конфиг для обхода блокировок (Hiddify / v2rayNG / Sing-box).\n\n"
            "🔥 Не откладывайте: пока вы раздумываете, доступ не вернётся сам. "
            "Действуйте прямо сейчас и снова будьте везде."
        )
        text += referral_anchor(await referral_link(tg_id))
        try:
            await bot.send_message(tg_id, text, reply_markup=main_menu_keyboard())
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
            await bot.send_message(tg_id, text, reply_markup=main_menu_keyboard())
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
            "🤝 <b>По вашей реферальной ссылке зарегистрировался друг{who}!</b>\n\n"
            "Когда он купит платный тариф, вы получите <b>+7 дней</b> к подписке бесплатно. "
            "Делитесь ссылкой дальше и приглашайте ещё больше друзей!"
        ).format(who=who)
    if kind == "referral_reward":
        days = data.get("reward_days") or 7
        return (
            f"🎁 <b>Вам начислено +{days} дней!</b>\n\n"
            "Друг купил тариф по вашей ссылке — бонус зачислен, ваша подписка продлена. "
            "Спасибо, что приводите друзей! 🚀"
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
            sent_expired = await send_expired_notifications()
            if sent_expired:
                logger.info("sent %d expired-subscription nudges", sent_expired)
            sent_renewed = await send_renewal_notifications()
            if sent_renewed:
                logger.info("sent %d renewal notifications", sent_renewed)
            sent_bot = await send_bot_notifications()
            if sent_bot:
                logger.info("sent %d bot notifications", sent_bot)
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
