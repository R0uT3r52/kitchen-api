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

    Note over K,RS: Подписка на топик orders.created

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
        else Заказ отменён клиентом до начала готовки
            RS->>K: отправка в orders.status.updates [accepted]
            K-->>A: orders worker проверяет FSM: переход запрещен, статус остается cancelled
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
2. **Получение заказов**: вместо постоянных HTTP-запросов (поллинга) кухня подписана на топик Kafka `orders.created` и моментально получает созданные заказы.
3. **Обновление статусов**: кухня публикует этапы готовки (`accepted -> cooking -> ready -> completed`) в топик Kafka `orders.status.updates`. Воркер `kitchen-api` считывает их и обновляет запись в базе данных.
4. **Защита от конфликтов (FSM)**: если клиент уже отменил заказ в API, воркер `kitchen-api` при попытке перевести заказ в `accepted` отклоняет изменение через проверку конечного автомата, сохраняя статус `cancelled`.
