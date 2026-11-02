## Архитектура (C4, Level 2)

```mermaid
flowchart TB
    User["Пользователь"]
    Owner["Владелец заведения"]

    Client["Web-клиент"]
    RS["Restaurant Service<br/>(симуляция кухни)"]

    subgraph Platform ["Платформа Авито.Кухня"]
        API["Kitchen API<br/>(REST API + gRPC + фоновые воркеры)"]
        DB[("PostgreSQL<br/>(заказы, меню, outbox)")]
        Kafka[("Apache Kafka<br/>(брокер сообщений)")]
    end

    User --> Client
    Client -->|"REST (JSON)"| API
    API <-->|"SQL (pgx)"| DB

    API -->|"публикация orders.created,<br/>orders.cancelled (outbox)"| Kafka
    Kafka -->|"чтение orders.status.updates<br/>(orders worker)"| API

    Owner --> RS
    RS -->|"gRPC: SyncMenu / Health<br/>(:50051, x-api-key)<br/>(REST fallback)"| API
    Kafka -->|"чтение orders.created,<br/>orders.cancelled"| RS
    RS -->|"отправка orders.status.updates"| Kafka
```

### Контейнеры

- **Web-клиент**: интерфейс покупателя для просмотра меню, оформления заказов и проверки их статуса.
- **Kitchen API**: основной Go-сервис. Обрабатывает входящие HTTP- (порт `8080`) и gRPC-запросы (порт `50051`), выполняет бизнес-логику и запускает фоновые горутины (Outbox Worker для отправки заказов и отмен в Kafka и Orders Worker для вычитки смены статусов из Kafka).
- **PostgreSQL**: реляционная БД для хранения заведений, позиций меню, заказов, аудита статусов (`order_events`) и таблицы очередей (`outbox_events`).
- **Apache Kafka**: брокер сообщений для асинхронного обмена событиями между платформой и кухней ресторана:
  - `orders.created` — события создания новых заказов.
  - `orders.cancelled` — события отмены заказов клиентами для остановки готовки.
  - `orders.status.updates` — события смены статусов приготовления от заведений.
- **Restaurant Service (`restaurant-example`)**: автономный Go-сервис кухни. При старте проверяет готовность gRPC (Health Check), синхронизирует меню по gRPC (`SyncMenu`), затем слушает `orders.created` и `orders.cancelled` из Kafka, параллельно в горутинах меняет статусы приготовления, мгновенно останавливает готовку при получении отмены и пушит статусы в `orders.status.updates`.
