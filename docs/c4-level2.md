## Архитектура (C4, Level 2)

```mermaid
flowchart TB
    User["Пользователь"]
    Owner["Владелец заведения"]

    Client["Web-клиент"]
    RS["Restaurant Service<br/>(симуляция кухни)"]

    subgraph Platform ["Платформа Авито.Кухня"]
        API["Kitchen API<br/>(REST API + фоновые воркеры)"]
        DB[("PostgreSQL<br/>(заказы, меню, outbox)")]
        Kafka[("Apache Kafka<br/>(брокер сообщений)")]
    end

    User --> Client
    Client -->|"REST (JSON)"| API
    API <-->|"SQL (pgx)"| DB

    API -->|"публикация orders.created<br/>(outbox worker)"| Kafka
    Kafka -->|"чтение orders.status.updates<br/>(orders worker)"| API

    Owner --> RS
    RS -->|"REST: синхронизация меню<br/>(X-API-KEY)"| API
    Kafka -->|"чтение orders.created"| RS
    RS -->|"отправка orders.status.updates"| Kafka
```

### Контейнеры

- **Web-клиент**: интерфейс покупателя для просмотра меню, оформления заказов и проверки их статуса.
- **Kitchen API**: основной Go-сервис. Обрабатывает входящие HTTP-запросы, выполняет бизнес-логику и запускает фоновые горутины (Outbox Worker для отправки заказов в Kafka и Orders Worker для вычитки смены статусов из Kafka).
- **PostgreSQL**: реляционная БД для хранения заведений, позиций меню, заказов, аудита статусов (`order_events`) и таблицы очередей (`outbox_events`).
- **Apache Kafka**: брокер сообщений для асинхронного обмена событиями между платформой и кухней ресторана:
  - `orders.created` — события создания новых заказов.
  - `orders.status.updates` — события смены статусов приготовления от заведений.
- **Restaurant Service (`restaurant-example`)**: автономный Go-сервис кухни. При старте синхронизирует меню по REST, затем слушает `orders.created` из Kafka и асинхронно пушит статусы готовки в `orders.status.updates`.
