## Архитектура (C4, Level 3)

```mermaid
flowchart TB
    Client["Клиент / Браузер"]
    RS["Restaurant Service"]
    DB[("PostgreSQL")]
    Kafka[("Apache Kafka")]

    subgraph API ["Kitchen API Service"]
        WEB["web:<br/>HTTP-хендлеры, DTO, middleware"]
        GRPC["grpc:<br/>gRPC-сервер, SyncMenu, Health, UnaryAuthInterceptor"]
        DOM["domain:<br/>use-cases, валидация, FSM"]
        REPO["repository:<br/>pgxpool, SQL, транзакции"]
        OUTBOX["outboxWorkers:<br/>фоновый опрос outbox_events"]
        ORDERS["ordersWorkers:<br/>обработка статусов из Kafka"]
        KAFKA_PKG["kafka:<br/>producer & consumer (kafka-go)"]
    end

    Client -->|"REST"| WEB
    RS -->|"gRPC (SyncMenu / Health)"| GRPC
    RS -.->|"REST fallback (PUT /menu)"| WEB

    WEB -->|"вызов use-cases"| DOM
    GRPC -->|"вызов use-cases"| DOM
    DOM -->|"интерфейсы репозитория"| REPO
    REPO <-->|"SQL (pgx)"| DB

    OUTBOX -->|"GetOutboxEvents / MarkProcessed"| REPO
    OUTBOX -->|"Publish"| KAFKA_PKG
    KAFKA_PKG -->|"orders.created,<br/>orders.cancelled"| Kafka

    Kafka -->|"orders.status.updates"| KAFKA_PKG
    KAFKA_PKG -->|"ReadMessage"| ORDERS
    ORDERS -->|"UpdateOrderStatus"| REPO
```

### Внутренние компоненты сервиса

1. **`web`**: приём входящих HTTP-запросов, валидация DTO, проверка `X-API-KEY` заведений, вызов бизнес-логики.
2. **`grpc`**: gRPC-сервер (порт `:50051`) на базе Protobuf (`partner.v1.MenuService`). Реализует метод `SyncMenu`, интерцептор аутентификации `x-api-key` (`UnaryAuthInterceptor`), стандартный Health Check (`grpc.health.v1`). Вызывает методы `domain.PartnerUseCase`.
3. **`domain`**: ядро бизнес-логики (`OrderService`, `PartnerService`) и матрица допустимых переходов конечного автомата (`CanTransition`).
4. **`repository`**: реализация интерфейсов доступа к PostgreSQL на `jackc/pgx/v5`. Атомарно сохраняет заказы, отмены и записи в `outbox_events` в единой транзакции, блокирует строки через `FOR UPDATE` и выбирает пачки событий с `FOR UPDATE SKIP LOCKED`.
5. **`outboxWorkers`**: фоновая горутина, опрашивающая таблицу `outbox_events` каждые 5 секунд и отправляющая события создания (`orders.created`) и отмены (`orders.cancelled`) в соответствующие топики Kafka.
6. **`ordersWorkers`**: фоновая горутина, вычитывающая события смены статусов заказов из топика Kafka `orders.status.updates` и передающая их в репозиторий для обновления в БД.
7. **`kafka`**: обёртка над библиотекой `segmentio/kafka-go` для создания `Producer` (с поддержкой динамической маршрутизации по топикам) и `Consumer`.
