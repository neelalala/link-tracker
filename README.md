# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

Запуск через [docker compose](compose.yaml):
```sh
docker compose build
docker compose up -d
```

[Dockerfile](./cmd/bot/Dockerfile) для бота

[Dockerfile](./cmd/scrapper/Dockerfile) для скраппера

Конфигурация:

[bot.conf](cmd/bot/bot.conf) – файл конфигурации бота.

[scrapper.conf](cmd/scrapper/scrapper.conf) – файл конфигурации Scrapper сервиса.

Пример .env файла для работы:
```
APP_TELEGRAM_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
BOT_API_PORT=63342
SCRAPPER_API_PORT=63343
BOT_URL="bot:${BOT_API_PORT}"
SCRAPPER_URL="scrapper:${SCRAPPER_API_PORT}"
BOT_API_PROTOCOL="http"
SCRAPPER_API_PROTOCOL="grpc"
SCRAPPER_LOG_LEVEL="DEBUG"
BOT_LOG_LEVEL="ERROR+4"
```