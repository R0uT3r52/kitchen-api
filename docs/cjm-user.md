# CJM пользователя

## Путь пользователя

```mermaid
sequenceDiagram
    actor U as Пользователь
    participant C as Web-клиент
    participant A as Kitchen API

    U->>C: открывает сервис
    C->>A: GET /api/v1/restaurants
    A-->>C: список активных заведений

    U->>C: выбирает заведение
    C->>A: GET /api/v1/restaurants/{id}/menu
    A-->>C: меню с доступными позициями

    U->>C: собирает заказ и отправляет
    C->>A: POST /api/v1/orders

    alt Позиция недоступна / заведение закрыто
        A-->>C: 400 Bad Request (товар в стоп-листе)
        C-->>U: сообщение об ошибке, предложение обновить корзину
    else Заказ успешно создан
        A-->>C: 201 Created (status=created)
        Note over A: Заказ и событие outbox_events сохранены в БД
        C-->>U: заказ создан, отправлен на кухню

        loop Отслеживание статуса заказа
            C->>A: GET /api/v1/orders/{id}
            A-->>C: текущий статус заказа
        end

        alt Успешное выполнение
            Note over A,C: статус обновляется из Kafka: accepted -> cooking -> ready -> completed
            C-->>U: заказ готов к получению / завершён
        else Заведение отклонило заказ
            Note over A,C: статус из Kafka: rejected
            C-->>U: заказ отклонён заведением
        else Пользователь отменил заказ
            U->>C: отменить заказ (до начала готовки)
            C->>A: POST /api/v1/orders/{id}/cancel
            A-->>C: 200 OK (status=cancelled)
            C-->>U: заказ отменён
        end
    end
```

### Пояснения к сценарию

1. Клиент взаимодействует с сервисом через стандартный REST API.
2. При успешном создании заказа (`POST /api/v1/orders`) запись заказа и событие для кухни сохраняются в PostgreSQL атомарно в одной транзакции (паттерн Transactional Outbox).
3. Изменения статуса заказа на кухне приходят в `kitchen-api` асинхронно через шину Kafka и сохраняются в БД. Клиент видит актуальный статус при опросе `GET /api/v1/orders/{id}`.
4. Отмена клиентом доступна, пока заказ находится в статусе `created` или `accepted` (до перехода в `cooking`).
