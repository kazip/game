игра про котика.

## Серверная часть

Добавлен выделенный сервер на Go, который хранит облики котов, таблицу очков и управляет мультиплеером. Сервер расположен в каталоге `server`.

Запуск (порт по умолчанию — 8080):

```bash
cd server
go run .
```

Фронтенд по умолчанию стучится в тот же origin, так что достаточно запустить сервер рядом со статикой или настроить `window.CAT_SERVER_URL` перед загрузкой скрипта.

## Автодеплой серверной части (GitHub Actions)

В репозиторий добавлен workflow `.github/workflows/deploy-server.yml`.
Он запускается при пуше в `main`, если изменились файлы в `server/**` (или сам workflow), и выкладывает новый бинарник по SSH.

### Нужные GitHub Secrets

- `SERVER_SSH_HOST` — хост сервера
- `SERVER_SSH_USER` — пользователь для SSH
- `SERVER_SSH_PORT` — порт SSH (опционально, по умолчанию `22`)
- `SERVER_SSH_PRIVATE_KEY` — приватный ключ для доступа по SSH
- `SERVER_DEPLOY_PATH` — базовая директория деплоя на сервере (например `/opt/catgame`)
- `SERVER_SYSTEMD_SERVICE` — имя systemd-сервиса (например `catgame.service`)

Опционально:

- `SERVER_ALLOWED_ORIGINS` — значение для `ALLOWED_ORIGINS` в runtime env
- `SERVER_BINARY_PROTOCOL_ENABLED` — значение для `BINARY_PROTOCOL_ENABLED` (`true/false`)

### Что должно быть на сервере

1. установлен systemd-сервис, который запускает бинарник из:
   - `${SERVER_DEPLOY_PATH}/current/catgame-server`
2. пользователь деплоя имеет право на запись в `${SERVER_DEPLOY_PATH}`
3. пользователь деплоя может выполнить:
   - `sudo systemctl restart ${SERVER_SYSTEMD_SERVICE}`
   - `sudo systemctl is-active --quiet ${SERVER_SYSTEMD_SERVICE}`
