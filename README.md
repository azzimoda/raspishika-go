# raspishika-go

Телеграм-бот для удобного получения расписание студентов и преподавателей МПК ТИУ. 

**Внимание!** Этот бот **не имеет прямого отношения к колледжу** и является **моим личным проектом**. По всем вопросам следует обращатся [лично мне](#ссылки).

---

## Использование

### Docker

_В планах..._

### Ручная сборка

Клонируй репозиторий:
```
git clone https://github.com/azzimoda/raspishika-go.git
```

Установи драйверы и зависимости Playwright:
```
go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install --with-deps
```

Скомпилируй проект:
```
go build -v -o ./raspishika ./cmd/cli/main.go
```

Отредактируй `configs/config.yml`, добавив туда API-токен своего бота и включив нужные `features`, заменив `false` на `true`, затем запусти:
```
./raspishika
```

Все опции CLI доступы по команде `./raspishika -help`.

---

## Ссылки

"Официальный" бот, поддерживаемый мной: [@RaspishikaBot](https://t.me/RaspishikaBot)

Мой телеграм-канал: [@mazzaLLM](https://t.me/mazzaLLM)

Поддержи меня:

- Звёздами в [ТГК](https://t.me/mazzaLLM)
- Toncoin: `UQCFh_yK4yLHwfRWrn-inUNYqw5boabRLmDtm5SEZf8SbDO1`
- [YooMoney](https://yoomoney.ru/to/4100119212250883)
