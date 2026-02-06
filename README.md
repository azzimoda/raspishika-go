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

### Docker

```sh
docker-compose run --rm raspishika # --help
```

Или

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
2. Отредактируйте конфигурацию [`configs/config.yml`](https://github.com/azzimoda/raspishika-go/blob/main/configs/config.yml), и создайте файл `.env`:
   ```sh
   RASPISHIKA_TELEGRAM_TOKEN=YOUR-TOKEN       # Токен основного бота
   RASPISHIKA_TELEGRAM_ADMIN_TOKEN=YOUR-TOKEN # Токен бота администратора
   RASPISHIKA_TELEGRAM_ADMIN_ID=YOUR-CHAT-ID  # Ваш `chat_id` для доступа к боту администратора
   ```
3. Скомпилируйте и запустите:
   ```sh
   go build -o ./raspishika ./cmd/cli/main.go
   ./raspishika
   ```

Все опции CLI доступы по команде `./raspishika -help`.

---

## Ссылки

"Официальный" бот, поддерживаемый мной: [@RaspishikaBot](https://RaspishikaBot.t.me)

Мой телеграм-канал: [@mazzaLLM](https://mazzaLLM.t.me)

Поддержи меня:

- Звёздами в [ТГК](https://mazzaLLM.t.me)
- Toncoin: `UQCFh_yK4yLHwfRWrn-inUNYqw5boabRLmDtm5SEZf8SbDO1`
- [YooMoney](https://yoomoney.ru/to/4100119212250883)
