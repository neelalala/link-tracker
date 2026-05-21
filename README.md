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
- [bot.conf](cmd/bot/bot.conf) – файл конфигурации бота.
- [scrapper.conf](cmd/scrapper/scrapper.conf) – файл конфигурации Scrapper сервиса.

Пример .env файла для работы:
```
TELEGRAM_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11

POSTGRES_PORT=5432
POSTGRES_USER=postgres_user
POSTGRES_PASSWORD=postgres_password
POSTGRES_DB=postgres_db
POSTGRES_URL="postgres:${POSTGRES_PORT}"
PGDATA=/var/lib/postgresql/data/pgdata
DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_URL}/${POSTGRES_DB}?sslmode=disable"
```