# Kitchen-API

Бэкенд-сервис для платформы доставки еды, объединяющий покупателей и заведения-партнеры. Архитектура построена на сочетании REST API (для клиентских сценариев и управления меню) и событийно-ориентированного подхода (EDA) на базе Apache Kafka и паттерна Transactional Outbox (для передачи заказов на кухню и отслеживания их приготовления).

## Features

- **Клиентский API (REST)**:
  - Каталог активных заведений и актуальное меню с учетом доступности (`is_available`).
  - Оформление заказа с валидацией позиций и серверным расчетом стоимости (Snapshot Pricing).
  - Просмотр деталей и истории заказов покупателя (`user_id`).
  - Отмена заказа клиентом до момента начала приготовления.
- **Партнерская интеграция и EDA (Apache Kafka + REST)**:
  - Аутентификация заведений по закрытому API-ключу (`X-API-KEY`).
  - Синхронизация меню заведения через идемпотентный REST-эндпоинт (`PUT /api/v1/partner/menu`, `UPSERT`).
  - Асинхронная отправка новых заказов на кухню через топик Kafka `orders.created`.
  - Асинхронное получение статусов приготовления от кухни через топик Kafka `orders.status.updates`.
  - Fallback REST-эндпоинты для просмотра заказов и смены статуса (`GET /api/v1/partner/orders`, `PATCH /api/v1/partner/orders/{id}/status`).
- **Паттерн Transactional Outbox**:
  - Атомарное сохранение заказа и события в таблицу `outbox_events` в единой транзакции PostgreSQL.
  - Фоновый воркер с конкурентной выборкой (`FOR UPDATE SKIP LOCKED`) для надежной отправки событий в Kafka без проблемы dual-write.
- **Конечный автомат (FSM)**:
  - Строгий контроль переходов состояний заказа (`created -> accepted -> cooking -> ready -> completed`, `rejected`, `cancelled`) с защитой от race conditions.
- **Аудит изменений (Audit Trail)**:
  - Версионирование смены статусов заказа в таблице `order_events` для истории и аналитики.
- **Автономный микросервис партнера (`restaurant-example`)**:
  - Готовая симуляция кухни заведения: синхронизирует меню по REST, слушает топик `orders.created`, параллельно в горутинах меняет статусы приготовления, отправляет их в Kafka и поддерживает graceful shutdown.

## Architecture

Сервис спроектирован по принципам чистой архитектуры (Clean Architecture): **Web** -> **Domain** <- **Repository**.
- **Domain**: изолированная бизнес-логика (`OrderService`, `PartnerService`), FSM-переходы и интерфейсы репозиториев.
- **Repository**: реализация на `jackc/pgx/v5` с транзакционным сохранением заказов, блокировками строк и очередью outbox.
- **Web**: HTTP-хендлеры, DTO, middleware авторизации.
- **Kafka & Workers**: фоновые пулы воркеров (`outboxWorkers` и `ordersWorkers`) для асинхронного взаимодействия с брокером Apache Kafka.

> Подробные диаграммы доступны в каталоге [`docs/`](docs/):
> - [C4 Level 2 Container Diagram](docs/c4-level2.md) — контейнеры (API, PostgreSQL, Kafka, Restaurant Service)
> - [C4 Level 3 Component Diagram](docs/c4-level3.md) — внутренние компоненты сервиса и воркеры
> - [CJM заведения (EDA)](docs/cjm-restaurant.md) — сценарий кухни при работе через Kafka
> - [CJM покупателя](docs/cjm-user.md) — сценарий оформления и отслеживания заказа

## Database

Реляционная база данных PostgreSQL 16.

> Схема БД и описание полей находится в [docs/db.md](docs/db.md).
> Ключевые оптимизации: частичный индекс `WHERE is_active = TRUE`, составной индекс меню `(restaurant_id, is_available)`, составной индекс заказов `(restaurant_id, status)` и частичный индекс очереди `idx_outbox_unprocessed ON (created_at) WHERE processed = false`.

## API & События

### REST API

Спецификация интерфейсов доступна в формате OpenAPI 3.0:  
**[OpenAPI Specs (api/openapi.yml)](api/openapi.yml)**

| Метод | Путь | Доступ | Описание |
| :--- | :--- | :--- | :--- |
| `GET` | `/health` | Публичный | Проверка работоспособности сервиса |
| `GET` | `/api/v1/restaurants` | Публичный | Список активных заведений |
| `GET` | `/api/v1/restaurants/{id}/menu` | Публичный | Меню заведения (доступные позиции) |
| `POST` | `/api/v1/orders` | Публичный (`user_id`) | Оформление нового заказа |
| `GET` | `/api/v1/orders/{id}` | Публичный | Детали и статус заказа |
| `GET` | `/api/v1/orders?user_id={id}` | Публичный (`user_id`) | Список всех заказов пользователя |
| `POST` | `/api/v1/orders/{id}/cancel` | Публичный (`user_id`) | Отмена заказа покупателем (до начала готовки) |
| `PUT` | `/api/v1/partner/menu` | `X-API-KEY` | Пакетный upsert меню и стоп-листа |
| `GET` | `/api/v1/partner/orders` | `X-API-KEY` | Заказы ресторана (фильтр по `status`) |
| `PATCH` | `/api/v1/partner/orders/{id}/status` | `X-API-KEY` | Обновление статуса заказа заведением |

### Топики Apache Kafka (EDA)

| Топик | Отправитель | Получатель | Ключ сообщения | Описание |
| :--- | :--- | :--- | :--- | :--- |
| `orders.created` | `kitchen-api` (Outbox) | `restaurant-example` | `order_id` | Новый заказ: состав, цены, ID ресторана |
| `orders.status.updates` | `restaurant-example` | `kitchen-api` (Orders Worker) | `order_id` | Смена статуса: `accepted`, `cooking`, `ready`, `completed`, `rejected` |

## Tests

### Unit Tests
- **Domain Layer (`internal/domain`)**:
  - Table-driven тесты конечного автомата `CanTransition`.
  - Тестирование бизнес-правил через mock-репозиторий (валидация корзины, проверка доступности позиций, права отмены).
- **Web Layer (`internal/web`)**:
  - Тестирование HTTP-хендлеров через `httptest`.
  - Проверка валидации DTO, middleware авторизации по `X-API-KEY`, статус-кодов.

### Integration Tests (Testcontainers)
- **Repository Layer (`internal/repository`)**:
  - Интеграционные тесты репозитория на базе контейнера PostgreSQL 16 с помощью **`testcontainers-go`** и автоматическим накатом SQL-миграций.
  - Проверка транзакционности: создание заказа, сохранение позиций, запись в `order_events` и `outbox_events`.

### Запуск тестов

```bash
# Запуск всех тестов (Unit + Integration через Testcontainers)
# Требуется Docker
go test -v ./...

# Запуск только unit-тестов
go test -v ./internal/domain/... ./internal/web/...

# Запуск интеграционных тестов репозитория
go test -v ./internal/repository/...
```

Проверка линтером (`golangci-lint`):
```bash
golangci-lint run ./...
```

## Run

Сервис и сопутствующие компоненты запускаются через Docker Compose.

### 1. Подготовка окружения
Скопируйте шаблон переменных окружения:
```bash
cp .env.template .env
```

### 2. Запуск контейнеров
```bash
docker compose up --build
```

В составе compose запускаются:
1. `database` — PostgreSQL 16 с проверкой healthcheck.
2. `kafkasvc` — брокер Apache Kafka.
3. `migrate` — применение SQL-миграций к БД.
4. `kafka-init` — автоматическое создание топиков `orders.created` и `orders.status.updates`.
5. `kitchen-api` — основной бэкенд-сервис на порту `8080`.
6. `restaurant-example` — сервис кухни заведения (синхронизирует меню и слушает заказы из Kafka).

Для остановки:
```bash
docker compose down
```

## Demo

После запуска контейнеров можно проверить полный цикл оформления и обработки заказа через `curl`.

### 1. Просмотр ресторанов и меню
```bash
# Список активных ресторанов
curl -s http://localhost:8080/api/v1/restaurants

# Меню заведения #1
curl -s http://localhost:8080/api/v1/restaurants/1/menu
```

### 2. Создание заказа покупателем
```bash
curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_42",
    "restaurant_id": 1,
    "items": [
      {"menu_item_id": 1, "quantity": 1},
      {"menu_item_id": 2, "quantity": 2}
    ]
  }'
```

### 3. Workflow обработки заказа на кухне
В логах Docker (`docker compose logs -f restaurant-example kitchen-api`) отобразится событийно-ориентированный цикл:
- `kitchen-api`: Outbox-воркер публикует созданный заказ в топик `orders.created`.
- `restaurant-example`: считывает заказ из Kafka и последовательно публикует статусы готовки (`accepted` -> `cooking` -> `ready` -> `completed`) в топик `orders.status.updates`.
- `kitchen-api`: воркер считывает статусы из Kafka и обновляет их в PostgreSQL.

### 4. Проверка статуса заказа
```bash
curl -s http://localhost:8080/api/v1/orders/1
```

### 5. Сценарий отмены клиентом (до готовки)
Клиент может отменить заказ, пока он в статусе `created` или `accepted`:
```bash
curl -s -X POST http://localhost:8080/api/v1/orders/1/cancel \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_42"}'
```
*Если кухня уже перевела заказ в статус `cooking`, API вернет `409 Conflict`.*

## Engineering decisions

### Ключевые архитектурные решения

1. **Серверная фиксация цен (Snapshot Pricing)**:
   Клиент отправляет только `menu_item_id` и `quantity`. Цены запрашиваются из БД на момент создания транзакции заказа и фиксируются в `order_items.unit_price_cents`. Это исключает возможность клиентской манипуляции суммами и гарантирует консистентность финансовой отчетности при последующих изменениях прайса рестораном.
2. **Строгая машина состояний и защита от гонок (FSM & Concurrency Control)**:
   Жизненный цикл заказа управляется строгой матрицей допустимых переходов (`CanTransition`). Все обновления статуса выполняются с блокировкой строки `SELECT ... FOR UPDATE`. Любые конфликтующие действия (например, попытка клиента отменить заказ, когда кухня уже перевела его в `cooking`) отклоняются с кодом `409 Conflict` (или отбрасываются воркером при обработке событий из Kafka).
3. **Audit Trail (событийный лог `order_events`)**:
   Каждое изменение статуса фиксируется в отдельной таблице `order_events` в рамках единой транзакции со сменой статуса в `orders`. Это дает полную прозрачность истории выполнения заказа без перегрузки основной сущности.
4. **Event-Driven Architecture и Transactional Outbox**:
   Взаимодействие кухни с платформой вынесено в Apache Kafka:
   - Передача заказов на кухню (`orders.created`) и отправка статусов приготовления (`orders.status.updates`) выполняются асинхронно через брокер сообщений. Это избавляет систему от поллинга и обеспечивает мгновенную доставку заказов на кухню.
   - Для надежной отправки применен паттерн **Transactional Outbox**: заказ и событие `outbox_events` сохраняются в рамках одной транзакции PostgreSQL. Это исключает проблему когда данные в БД записались, а при отправке в сеть произошел сбой. Фоновый воркер считывает события пачками с помощью `SELECT ... FOR UPDATE SKIP LOCKED`, отправляет в Kafka и помечает как обработанные, что позволяет параллелить воркеры без блокировок.
   - Сообщения в топиках партиционируются по ключу `order_id`, гарантируя строгую последовательность событий по каждому заказу.
5. **Транзакционный и идемпотентный Upsert меню**:
   Синхронизация позиций меню выполняется через запрос с ограничением `UNIQUE(restaurant_id, external_id)`. Это позволяет заведению безопасно повторно выгружать меню, обновляя цены и флаги стоп-листа без дублирования данных.
6. **Целочисленное хранение денежных сумм (Money Pattern)**:
   Все цены и суммы хранятся в целочисленном виде в минимальных неделимых единицах (`price_cents`, `unit_price_cents`, `total_price_cents` типа `bigint` / `int64`). Это исключает ошибки округления чисел с плавающей точкой (`float`) и гарантирует строгую точность расчетов.

### Допущенные упрощения (Scope MVP)
- **Идентификация клиентов**: в рамках MVP передается через параметр `user_id` без использования JWT.
- **Авторизация партнеров**: используется статический секретный ключ в HTTP-заголовке `X-API-KEY`, валидируемый через middleware.
