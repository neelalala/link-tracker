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
[application.conf](application.conf) – файл конфигурации микросервисов.

- telegram-token – токена для телеграмм-бота
- environment – исполняемое окружение
- logs-file – файл, куда печатать логи
- updates-interval-seconds – время между получением обновлений скраппером
- bot-url – адрес скрапперу для вызова методов бота
- scrapper-url – адрес боту для вызова методов скраппера
- bot-api-port – порт, на котором будет слушать бот api запросы
- scrapper-api-port – порт, на котором будет слушать скраппер api запросы
- api-protocol – протокол, по которому будут общаться микросервисы
- log-level – уровень логгирования, строка в формате, ожидаемом пакетом `slog`


Пример .env файла для работы:
```
APP_TELEGRAM_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
BOT_API_PORT=63342
SCRAPPER_API_PORT=63343
BOT_URL="http://bot:${BOT_API_PORT}"
SCRAPPER_URL="http://scrapper:${SCRAPPER_API_PORT}"
LOG_LEVEL="ERROR+2"
```