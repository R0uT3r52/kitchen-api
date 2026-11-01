# CJM заведения

## Путь заведения (EDA)

```mermaid
sequenceDiagram
    actor R as Заведение
    participant RS as Restaurant Service
    participant K as Apache Kafka
    participant A as Kitchen API

    R->>RS: запуск сервиса (конфиг, api_key)
    RS->>A: PUT /api/v1/partner/menu (REST)
    A-->>RS: 200 OK (меню синхронизировано)

    Note over K,RS: Подписка на топики orders.created и orders.cancelled

    alt Поступление нового заказа
        A->>K: Outbox воркер публикует заказ в orders.created
        K-->>RS: чтение события OrderCreatedDTO

        alt Заведение готовит заказ (стандартный цикл)
            RS->>K: отправка в orders.status.updates [accepted]
            K-->>A: orders worker обновляет статус в БД на accepted
            RS->>K: отправка в orders.status.updates [cooking]
            K-->>A: orders worker обновляет статус в БД на cooking
            RS->>K: отправка в orders.status.updates [ready]
            K-->>A: orders worker обновляет статус в БД на ready
            RS->>K: отправка в orders.status.updates [completed]
            K-->>A: orders worker обновляет статус в БД на completed
        else Заведение отклоняет заказ
            RS->>K: отправка в orders.status.updates [rejected]
            K-->>A: orders worker обновляет статус в БД на rejected
        else Заказ отменён клиентом во время готовки
            A->>K: Outbox воркер публикует отмену в orders.cancelled
            K-->>RS: чтение события OrderStatusCancelledDTO
            Note over RS: context.CancelFunc прерывает пайплайн готовки, дальнейшие статусы не отправляются
        end
    end

    opt Обновление меню / Стоп-лист
        R->>RS: блюдо закончилось на кухне
        RS->>A: PUT /api/v1/partner/menu (is_available=false для позиции)
        A-->>RS: 200 OK (меню обновлено)
    end
```

### Пояснения к сценарию

1. **Синхронизация меню**: при старте сервис кухни отправляет текущее меню через идемпотентный REST-запрос (`PUT /api/v1/partner/menu`) с заголовком `X-API-KEY`.
2. **Получение заказов и отмен**: вместо постоянных HTTP-запросов (поллинга) кухня подписана на топики Kafka `orders.created` (создание заказов) и `orders.cancelled` (отмена заказов) и обрабатывает события асинхронно.
3. **Обновление статусов**: кухня публикует этапы готовки (`accepted -> cooking -> ready -> completed`) в топик Kafka `orders.status.updates`. Воркер `kitchen-api` считывает их и обновляет запись в базе данных.
4. **Обработка отмены заказа и защита FSM**: при отмене заказа клиентом событие через Outbox попадает в топик `orders.cancelled`. Сервис кухни по вычитке отмены вызывает `cancelFunc()` контекста активного заказа, немедленно прерывая горутину готовки и прекращая отправку статусов в API. Если статус `accepted` уже успел отправиться до обработки отмены, воркер `kitchen-api` защищён конечным автоматом (FSM) и сохраняет статус `cancelled`.
