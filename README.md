# Распиши-ка

Телеграм-бот для удобного получения расписание студентов и преподавателей МПК ТИУ. 

> [!important]
> Этот бот **не имеет прямого отношения к Многопрофильному колледжу ТИУ** и является **моим личным проектом**. По всем вопросам следует обращатся [ко мне лично](#ссылки).

---

## Стек

- Go
  - `go-telegram/bot`
  - `mattn/go-sqlite3`
  - `playwright-community/playwright-go`
  - `spf13/viper`
- Node.js
  - Playwright v0.5200.0
- SQLite3

## Использование

### Минимальная конфигурация

Для работоспособности основного бота как минимум необходимо задать его API-токен в `.env` файле:

```sh
RASPISHIKA_TELEGRAM_TOKEN="your_token"
```

### Docker

```sh
docker-compose run --rm raspishika # --help
```

или

```sh
docker build -t raspishika .
docker run --rm \
    -v ./.env:/app/.env \
    -v ./configs:/app/configs \
    -v ./database/db.sqlite3:/app/database/db.sqlite3 \
    raspishika # --help
```

### Ручная сборка

1. Установите Playwright версии `v0.5200.0`:
   ```sh
   go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install --with-deps
   ```
2. Скомпилируйте и запустите:
   ```sh
   go build -o ./raspishika ./cmd/cli/main.go
   ./raspishika
   ```

## Конфигурация

Приоритетность конфигурации:

1. CLI аргументы,
2. переменные окружения,
3. файлы.

### Файлы

Используемые файлы конфигурации:

- `configs/config.yml` — основная конфигурация (по умолчанию);
- `configs/commands.yml` — команды ботов (`my_commands`);
- `.env` — секреты: API-токены и `chat_id` админа.

По умолчанию все дополнительные `features` отключены.

### Переменные окружения

Используемые переменные окружения:

- `RASPISHIKA_TELEGRAM_TOKEN` — API-токен основного бота;
- `RASPISHIKA_TELEGRAM_ADMIN_TOKEN` — API-токен бота администратора;
- `RASPISHIKA_TELEGRAM_ADMIN_CHAT_ID` — `chat_id` администратора;
- `RASPISHIKA_CONFIG_FILE` — путь к файлу конфигурации;
- `RASPISHIKA_COMMANDS_FILE` — путь к файлу команд ботов.

### CLI

Также доступны CLI аргументы для некоторых настроек, список которых доступен по флагу `--help`.


---

## Ссылки

"Официальный" бот, поддерживаемый мной: [@RaspishikaBot](https://RaspishikaBot.t.me)

Мой телеграм-канал: [@mazzaLLM](https://mazzaLLM.t.me)

Поддержи меня:

- Звёздами в [ТГК](https://mazzaLLM.t.me)
- Toncoin: `UQCFh_yK4yLHwfRWrn-inUNYqw5boabRLmDtm5SEZf8SbDO1`
- [YooMoney](https://yoomoney.ru/to/4100119212250883)
