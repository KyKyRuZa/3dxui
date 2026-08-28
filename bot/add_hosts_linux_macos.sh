#!/usr/bin/env python3
import os
import sys
import platform
import subprocess

# Параметры
IP = "2.26.138.90"
DOMAIN = "thenomoreblocks.com"

def get_hosts_path():
    system = platform.system()
    if system == "Windows":
        return os.path.join(os.environ["SystemRoot"], "System32", "drivers", "etc", "hosts")
    else:  # Linux, macOS, Darwin
        return "/etc/hosts"

def flush_dns():
    system = platform.system()
    try:
        if system == "Windows":
            subprocess.run(["ipconfig", "/flushdns"], check=True, capture_output=True)
        elif system == "Darwin":  # macOS
            subprocess.run(["sudo", "dscacheutil", "-flushcache"], check=True)
            subprocess.run(["sudo", "killall", "-HUP", "mDNSResponder"], check=True)
        elif system == "Linux":
            # Пробуем разные способы
            if subprocess.run(["systemctl", "is-active", "systemd-resolved"], capture_output=True).returncode == 0:
                subprocess.run(["sudo", "systemctl", "restart", "systemd-resolved"], check=True)
            elif subprocess.run(["service", "nscd", "status"], capture_output=True, shell=False).returncode == 0:
                subprocess.run(["sudo", "service", "nscd", "restart"], check=True)
            else:
                print("[ВНИМАНИЕ] Не удалось автоматически сбросить DNS-кэш. Перезагрузите систему.")
                return
        print("[OK] DNS-кэш сброшен.")
    except Exception as e:
        print(f"[ОШИБКА] Не удалось сбросить DNS-кэш: {e}")

def main():
    # Проверка прав (только для Unix)
    if platform.system() != "Windows" and os.geteuid() != 0:
        print("[ОШИБКА] Запустите скрипт с sudo: sudo python3 add_hosts.py")
        sys.exit(1)
    # Для Windows права обычно проверяются через запрос UAC, но мы можем попробовать записать в hosts
    hosts_path = get_hosts_path()
    if not os.access(hosts_path, os.W_OK):
        print("[ОШИБКА] Нет прав на запись в файл hosts. Запустите от имени администратора (Windows) или с sudo (macOS/Linux).")
        sys.exit(1)

    # Проверяем наличие записи
    try:
        with open(hosts_path, "r") as f:
            lines = f.readlines()
    except Exception as e:
        print(f"[ОШИБКА] Не удалось прочитать hosts: {e}")
        sys.exit(1)

    # Ищем домен
    found = False
    for i, line in enumerate(lines):
        if DOMAIN in line and not line.strip().startswith("#"):
            found = True
            print(f"[ВНИМАНИЕ] Запись для {DOMAIN} уже существует: {line.strip()}")
            choice = input("Хотите обновить IP-адрес (перезаписать)? (y/n): ")
            if choice.lower() == "y":
                # Удаляем старую запись
                lines = [l for l in lines if DOMAIN not in l or l.strip().startswith("#")]
                found = False  # чтобы добавить новую
            else:
                print("Отмена. Запись не изменена.")
                return
            break

    # Если записи нет или мы её удалили, добавляем
    if not found:
        with open(hosts_path, "a") as f:
            f.write(f"{IP}    {DOMAIN}\n")
        print(f"[OK] Запись добавлена: {IP}    {DOMAIN}")

    # Сброс DNS
    flush_dns()
    print(f"Готово! Теперь сайт {DOMAIN} должен открываться без VPN.")

if __name__ == "__main__":
    main()