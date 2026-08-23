# YouGile REST API v2 — Reference

**Источник**: https://ru.yougile.com/api-json (официальный OpenAPI spec, загружен 2026-05-19)
**UI**: https://ru.yougile.com/api-v2#/
**Base URL**: `https://ru.yougile.com/api-v2`
**Auth**: `Authorization: Bearer {apiKey}`
**Rate Limit**: 50 req/min

---

## 1. Авторизация

### 1.1 Получить список компаний

```
POST /auth/companies
Content-Type: application/json

{ "login": "email", "password": "pass", "name": "companyName" }
```

**Request**: `CredentialsWithNameDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `login` | string | да | Логин пользователя |
| `password` | string | да | Пароль пользователя |
| `name` | string | нет | Название компании |

**Response**: массив `CompanyListDtoBase`:
| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | ID компании |
| `name` | string | Название компании |
| `isAdmin` | boolean | Права администратора |

### 1.2 Создать ключ API

```
POST /auth/keys
Content-Type: application/json

{ "login": "email", "password": "pass", "companyId": "uuid" }
```

**Request**: `CredentialsWithCompanyDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `login` | string | да | Логин пользователя |
| `password` | string | да | Пароль пользователя |
| `companyId` | string | да | ID компании |

**Response**: `AuthKeyDto` = `{ "key": "string" }`

### 1.3 Получить список ключей

```
POST /auth/keys/get
Content-Type: application/json

{ "login": "password", "password": "pass", "companyId?": "uuid" }
```

**Request**: `CredentialsWithCompanyOptionalDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `login` | string | да | Логин пользователя |
| `password` | string | да | Пароль пользователя |
| `companyId` | string | нет | ID компании |

**Response**: массив `AuthKeyWithDetailsDto`
| Поле | Тип | Описание |
|------|-----|----------|
| `key` | string | Ключ авторизации |
| `companyId` | string | ID компании |
| `timestamp` | number | Время создания |
| `deleted` | boolean | Ключ удалён |

### 1.4 Удалить ключ

```
DELETE /auth/keys/{key}
```

---

## 2. Сотрудники (Users)

### 2.1 Получить список

```
GET /users?companyId=uuid
```

**Response**: `UserListDto`
| Поле | Тип | Описание |
|------|-----|----------|
| `paging` | PagingMetadata | Пагинация |
| `content` | [UserListDtoBase] | Массив пользователей |

**UserListDtoBase / UserDto**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `email` | string | да | Почтовый ящик |
| `realName` | string | да | ФИО |
| `status` | string | да | `online` / `offline` |
| `lastActivity` | number | да | Время последнего действия |
| `isAdmin` | boolean | нет | Права администратора |

### 2.2 Пригласить сотрудника

```
POST /users
Content-Type: application/json

{ "email": "user@example.com", "isAdmin": false }
```

**Request**: `CreateUserDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `email` | string | да | Почтовый ящик |
| `isAdmin` | boolean | нет | Права администратора |

### 2.3 Текущий пользователь

```
GET /users/me
```
**Response**: `UserDto`

### 2.4 Получить по ID

```
GET /users/{id}
```

### 2.5 Изменить

```
PUT /users/{id}
Content-Type: application/json

{ "isAdmin": true }
```

**Request**: `UpdateUserDto`
| Поле | Тип | Описание |
|------|-----|----------|
| `isAdmin` | boolean | Права администратора |

### 2.6 Удалить

```
DELETE /users/{id}
```

---

## 3. Компания (Company)

### 3.1 Получить детали

```
GET /companies{*companyId}
```

> **Важно**: URL содержит wildcard — запрос формируется как `/companies09a0fdb3-...` **без слеша** между `companies` и ID. Это баг или особенность spec'а, требует проверки на реальном API.

**Response**: `CompanyDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `title` | string | да | Название компании |
| `timestamp` | number | да | Время создания |
| `apiData` | object | нет | Вспомогательные данные (структура не специфицирована) |
| `deleted` | boolean | нет | Статус удаления |

### 3.2 Изменить

```
PUT /companies{*companyId}
Content-Type: application/json

{ "title": "New Name" }
```

**Request**: `UpdateCompanyDto`
| Поле | Тип | Описание |
|------|-----|----------|
| `title` | string | Название компании |
| `apiData` | object | Вспомогательные данные |
| `deleted` | boolean | Статус удаления |

---

## 4. Файлы (Upload)

> ⚠️ Эндпоинт существует, но в текущей версии не используется. Формат multipart/form-data требует экспериментальной проверки. Исключён из плана реализации.

---

## 5. Проекты (Projects)

### 5.1 Получить список

```
GET /projects
```

**Response**: `ProjectListDto`
| Поле | Тип | Описание |
|------|-----|----------|
| `paging` | PagingMetadata | Пагинация |
| `content` | [ProjectListDtoBase] | Массив проектов |

**ProjectListDtoBase / ProjectDto**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `title` | string | да | Название проекта |
| `timestamp` | number | да | Время создания |
| `users` | object | нет | `{ userId: role }`, где role: `worker` / `admin` / `observer` / `customer` |
| `deleted` | boolean | нет | Статус удаления |

### 5.2 Создать

```
POST /projects
Content-Type: application/json

{ "title": "Project Name", "users": { "userId": "admin" } }
```

**Request**: `CreateProjectDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | да | Название проекта |
| `users` | object | нет | `{ userId: role }` |

### 5.3 Получить по ID

```
GET /projects/{id}
```

### 5.4 Изменить

```
PUT /projects/{id}
Content-Type: application/json

{ "title": "New Name", "users": {...}, "deleted": false }
```

**Request**: `UpdateProjectDto`
| Поле | Тип | Описание |
|------|-----|----------|
| `title` | string | Название проекта |
| `users` | object | `{ userId: role }` |
| `deleted` | boolean | Статус удаления |

---

## 6. Роли проекта (Project Roles)

### 6.1 Получить список ролей

```
GET /projects/{projectId}/roles
```

**Response**: `ProjectRoleListDto` → `content`: `[ProjectRoleListDtoBase]`

**ProjectRoleDto / ProjectRoleListDtoBase**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `name` | string | да | Название роли |
| `description` | string | нет | Описание роли |
| `permissions` | ProjectPermissionsDto | да | Права в проекте |

### 6.2 Создать роль

```
POST /projects/{projectId}/roles
Content-Type: application/json

{ "name": "Role Name", "description": "...", "permissions": {...} }
```

**Request**: `CreateProjectRoleDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `name` | string | да | Название роли |
| `description` | string | нет | Описание роли |
| `permissions` | ProjectPermissionsDto | да | Права в проекте |

### 6.3 Получить по ID

```
GET /projects/{projectId}/roles/{id}
```

### 6.4 Изменить

```
PUT /projects/{projectId}/roles/{id}
Content-Type: application/json

{ "name": "...", "description": "...", "permissions": {...} }
```

### 6.5 Удалить

```
DELETE /projects/{projectId}/roles/{id}
```

### 6.6 Структура прав (Permissions)

**ProjectPermissionsDto**:
| Поле | Тип | Описание |
|------|-----|----------|
| `editTitle` | boolean | |
| `delete` | boolean | |
| `addBoard` | boolean | |
| `boards` | BoardPermissionsDto | Права на доски |
| `children` | ChildrenDto | |

**BoardPermissionsDto**:
| Поле | Тип | Описание |
|------|-----|----------|
| `editTitle` | boolean | |
| `delete` | boolean | |
| `move` | boolean | |
| `showStickers` | boolean | |
| `editStickers` | boolean | |
| `addColumn` | boolean | |
| `columns` | ColumnPermissionsDto | Права на колонки |
| `settings` | boolean | |

**ColumnPermissionsDto**:
| Поле | Тип | Описание |
|------|-----|----------|
| `editTitle` | boolean | |
| `delete` | boolean | |
| `move` | string | |
| `addTask` | boolean | |
| `allTasks` | TaskPermissionsDto | |
| `withMeTasks` | TaskPermissionsDto | |
| `myTasks` | TaskPermissionsDto | |
| `createdByMeTasks` | TaskPermissionsDto | |

**TaskPermissionsDto** (все поля обязательные):
| Поле | Тип | Описание |
|------|-----|----------|
| `show` | boolean | |
| `delete` | boolean | |
| `editTitle` | boolean | |
| `editDescription` | boolean | |
| `complete` | boolean | |
| `close` | boolean | |
| `assignUsers` | string | |
| `connect` | boolean | |
| `editSubtasks` | string | |
| `editStickers` | boolean | |
| `editPins` | boolean | |
| `move` | string | |
| `sendMessages` | boolean | |
| `sendFiles` | boolean | |
| `editWhoToNotify` | string | |

---

## 7. Отделы (Departments)

### 7.1 Получить список

```
GET /departments
```

**Response**: `DepartmentListDto` → `content`: `[DepartmentListDtoBase]`

**DepartmentDto / DepartmentListDtoBase**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `title` | string | да | Название отдела |
| `parentId` | string | нет | `"-"` или ID родительского отдела |
| `users` | object | нет | `{ userId: "manager" | "member" }` |
| `deleted` | boolean | нет | Статус удаления |

### 7.2 Создать

```
POST /departments
Content-Type: application/json

{ "title": "Department", "parentId": "-", "users": {...} }
```

**Request**: `CreateDepartmentDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | да | Название отдела |
| `parentId` | string | нет | `"-"` или ID родителя |
| `users` | object | нет | `{ userId: "manager" | "member" }` |

### 7.3 Получить по ID

```
GET /departments/{id}
```

### 7.4 Изменить

```
PUT /departments/{id}
Content-Type: application/json

{ "title": "...", "parentId": "...", "users": {...}, "deleted": false }
```

**Request**: `UpdateDepartmentDto` (те же поля + `deleted`)

---

## 8. Доски (Boards)

### 8.1 Получить список

```
GET /boards?projectId=uuid
```

**Response**: `BoardListDto` → `content`: `[BoardListDtoBase]`

**BoardDto / BoardListDtoBase**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `title` | string | да | Название доски |
| `projectId` | string | да | ID проекта |
| `stickers` | StickersDto | нет | Настройки стикеров доски |
| `deleted` | boolean | нет | Статус удаления |

**StickersDto**:
| Поле | Тип | Описание |
|------|-----|----------|
| `assignee` | boolean | Исполнитель |
| `deadline` | boolean | Дедлайн |
| `repeat` | boolean | Регулярная задача |
| `stopwatch` | boolean | Секундомер |
| `timeTracking` | boolean | Таймтрекинг |
| `timer` | boolean | Таймер |
| `custom` | object | **Пользовательские стикеры доски** (формат не специфицирован) |

### 8.2 Создать

```
POST /boards
Content-Type: application/json

{ "title": "Board Name", "projectId": "uuid", "stickers": {...} }
```

**Request**: `CreateBoardDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | да | Название доски |
| `projectId` | string | да | ID проекта |
| `stickers` | StickersDto | нет | Настройки стикеров |

### 8.3 Получить по ID

```
GET /boards/{id}
```

### 8.4 Изменить

```
PUT /boards/{id}
Content-Type: application/json

{ "title": "...", "projectId": "...", "stickers": {...}, "deleted": false }
```

**Request**: `UpdateBoardDto`

---

## 9. Колонки (Columns)

### 9.1 Получить список

```
GET /columns?boardId=uuid
```

**Response**: `ColumnListDto` → `content`: `[ColumnListDtoBase]`

**ColumnDto / ColumnListDtoBase**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `title` | string | да | Название колонки |
| `boardId` | string | да | ID доски |
| `color` | number | нет | Цвет колонки (числовой код: 1 = #7B869E) |
| `deleted` | boolean | нет | Статус удаления |

### 9.2 Создать

```
POST /columns
Content-Type: application/json

{ "title": "Column Name", "boardId": "uuid", "color": 1 }
```

**Request**: `CreateColumnDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | да | Название колонки |
| `boardId` | string | да | ID доски |
| `color` | number | нет | Цвет колонки |

### 9.3 Получить по ID

```
GET /columns/{id}
```

### 9.4 Изменить

```
PUT /columns/{id}
Content-Type: application/json

{ "title": "...", "boardId": "...", "color": 1, "deleted": false }
```

---

## 10. Задачи (Tasks)

### 10.1 Получить список (прямой порядок)

```
GET /task-list?boardId=uuid&columnId=uuid&limit=100&offset=0
```

> ⚠️ Проверено на реальном API (2026-08-23): `/tasks` **не принимает `boardId`** — только `columnId`.
> Запрос с `boardId` в query → `400 {"message":["property boardId should not exist"]}` (NestJS forbidNonWhitelisted).
> Аналогично `/task-list`: только `columnId`.

### 10.2 Получить список (обратный порядок)

```
GET /tasks?boardId=uuid&columnId=uuid&limit=100
```

**Response**: `TaskListDto` → `content`: `[TaskListDtoBase]`

**TaskDto / TaskListDtoBase**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `title` | string | да | Название задачи |
| `timestamp` | number | да | Время создания |
| `columnId` | string | нет | ID колонки |
| `description` | string | нет | Описание задачи |
| `completed` | boolean | нет | Выполнена |
| `completedTimestamp` | number | нет | Время выполнения |
| `archived` | boolean | нет | В архиве |
| `archivedTimestamp` | number | нет | Время архивации |
| `deleted` | boolean | нет | Удалена |
| `assigned` | [string] | нет | Массив ID исполнителей |
| `createdBy` | string | нет | ID создателя |
| `stickers` | object | нет | `{ stickerId: value }` — см. п. 17.1 "Формат стикеров в задаче" |
| `deadline` | Deadline | нет | Дедлайн |
| `subtasks` | [string] | нет | Массив ID подзадач |
| `color` | string | нет | Цвет карточки (`task-primary`, `task-gray`, `task-red`, `task-pink`, `task-yellow`) |
| `checklists` | [CheckList] | нет | Чеклисты |
| `stopwatch` | Stopwatch | нет | Секундомер |
| `timer` | Timer | нет | Таймер |
| `timeTracking` | TimeTracking | нет | Таймтрекинг |
| `deal` | DealReadDto | нет | Данные CRM сделки |
| `extensionData` | object | нет | Данные расширений |
| `idTaskCommon` | string | нет | Сквозной ID по компании |
| `idTaskProject` | string | нет | ID внутри проекта |
| `type` | string | нет | Тип сущности |

### 10.3 Создать

```
POST /tasks
Content-Type: application/json

{
  "title": "Task Name",
  "columnId": "uuid",
  "description": "...",
  "deadline": { "deadline": 1736637798215 },
  "completed": false,
  "assigned": ["userId1"],
  "stickers": { "stickerId": "value" },
  "subtasks": ["childTaskId"],
  "checklists": [{ "title": "Checklist", "items": [{ "title": "Item", "isCompleted": false }] }],
  "color": "task-primary"
}
```

**Request**: `CreateTaskDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | **да** | Название задачи |
| `columnId` | string | нет | ID колонки |
| `description` | string | нет | Описание |
| `deadline` | Deadline/create | нет | `{ deadline: timestamp_ms }` |
| `completed` | boolean | нет | Выполнена |
| `archived` | boolean | нет | В архиве |
| `assigned` | [string] | нет | Массив ID исполнителей |
| `stickers` | object | нет | `{ stickerId: value }` |
| `subtasks` | [string] | нет | Массив ID подзадач |
| `checklists` | [CheckList] | нет | Чеклисты |
| `color` | string | нет | Цвет карточки |
| `stopwatch` | CreateStopwatch | нет | Секундомер |
| `timer` | CreateTimer | нет | Таймер |
| `timeTracking` | TimeTracking | нет | Таймтрекинг |
| `deal` | DealDataDto | нет | CRM сделка |
| `extensionData` | object | нет | Данные расширения |
| `idTaskCommon` | string | нет | Сквозной ID |
| `idTaskProject` | string | нет | ID внутри проекта |

### 10.4 Получить по ID

```
GET /tasks/{id}
```

### 10.5 Изменить

```
PUT /tasks/{id}
Content-Type: application/json

{
  "title": "...",
  "columnId": "uuid",        // "-" для удаления из колонки
  "completed": true,
  "deadline": { ... },
  "stickers": { ... },
  "subtasks": [...],
  "deleted": false
}
```

**Request**: `UpdateTaskDto` (все поля опциональны, кроме отмеченных внутри DTO)
| Поле | Тип | Описание |
|------|-----|----------|
| `title` | string | |
| `columnId` | string | `"-"` удаляет задачу из колонки |
| `description` | string | |
| `completed` | boolean | |
| `archived` | boolean | |
| `deleted` | boolean | Удалить |
| `deadline` | UpdateDeadline | |
| `stickers` | object | |
| `subtasks` | [string] | |
| `assigned` | [string] | |
| `checklists` | [CheckList] | Передаётся полностью |
| `color` | string | |
| `stopwatch` | UpdateStopwatch | |
| `timer` | UpdateTimer | |
| `timeTracking` | UpdateTimeTracking | |
| `deal` | DealDataDto | |
| `extensionData` | object | |
| `idTaskCommon` | string | |
| `idTaskProject` | string | |

### 10.6 Подписчики чата задачи

**Получить**:
```
GET /tasks/{id}/chat-subscribers
```
Response: `TaskChatSubscribersDto` = `{ content: [...] }`

**Изменить**:
```
PUT /tasks/{id}/chat-subscribers
Content-Type: application/json

["userId1", "userId2"]
```
Request: массив строк (ID пользователей)

---

## 11. Стикер с набором состояний (String Sticker)

Стикер строкового типа с предопределёнными состояниями (dropdown/select).

### 11.1 Получить список

```
GET /string-stickers?limit=100&offset=0
```

**Response**: `StringStickerWithStatesListDto`

**StringStickerWithStatesDto**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `name` | string | да | Имя стикера |
| `icon` | string | нет | Иконка стикера |
| `states` | [StringStickerStateDto] | нет | Состояния |
| `deleted` | boolean | нет | Статус удаления |

### 11.2 Создать

```
POST /string-stickers
Content-Type: application/json

{ "name": "Priority", "icon": "...", "states": [{ "name": "High", "color": "#ff0000" }] }
```

**Request**: `CreateStringStickerDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `name` | string | да | Имя стикера |
| `icon` | string | нет | Иконка |
| `states` | [StringStickerStateNoIdDto] | нет | Состояния (без ID) |

### 11.3 Получить по ID

```
GET /string-stickers/{id}
```

### 11.4 Изменить

```
PUT /string-stickers/{id}
Content-Type: application/json

{ "name": "...", "icon": "...", "deleted": false }
```

**StringStickerStateDto**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID состояния |
| `name` | string | да | Имя состояния |
| `color` | string | нет | Цвет (hex) |
| `deleted` | boolean | нет | Статус удаления |

---

## 12. Состояния стикера String (String Sticker States)

### 12.1 Создать состояние

```
POST /string-stickers/{stickerId}/states
Content-Type: application/json

{ "name": "High", "color": "#ff0000" }
```

### 12.2 Получить по ID

```
GET /string-stickers/{stickerId}/states/{stickerStateId}
```

### 12.3 Изменить состояние

```
PUT /string-stickers/{stickerId}/states/{stickerStateId}
Content-Type: application/json

{ "name": "...", "color": "...", "deleted": false }
```

---

## 13. Стикер спринта (Sprint Sticker)

Стикер для отметки принадлежности задачи к спринту с датами.

### 13.1 Получить список

```
GET /sprint-stickers
```

**SprintStickerWithStatesDto**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID объекта |
| `name` | string | да | Имя стикера |
| `states` | [SprintStickerStateDto] | нет | Состояния |
| `deleted` | boolean | нет | Статус удаления |

### 13.2 Создать

```
POST /sprint-stickers
Content-Type: application/json

{ "name": "Sprint 1", "states": [{ "name": "Week 1", "begin": 1711234567, "end": 1711834567 }] }
```

**Request**: `CreateSprintStickerDto`
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `name` | string | да | Имя стикера |
| `states` | [SprintStickerStateNoIdDto] | нет | Состояния |

### 13.3 Получить по ID

```
GET /sprint-stickers/{id}
```

### 13.4 Изменить

```
PUT /sprint-stickers/{id}
Content-Type: application/json

{ "name": "...", "deleted": false }
```

---

## 14. Состояния стикера спринта (Sprint Sticker States)

### 14.1 Создать состояние

```
POST /sprint-stickers/{stickerId}/states
Content-Type: application/json

{ "name": "Week 1", "begin": 1711234567, "end": 1711834567 }
```

### 14.2 Получить по ID

```
GET /sprint-stickers/{stickerId}/states/{stickerStateId}
```

### 14.3 Изменить состояние

```
PUT /sprint-stickers/{stickerId}/states/{stickerStateId}
Content-Type: application/json

{ "name": "...", "begin": ..., "end": ..., "deleted": false }
```

**SprintStickerStateDto**:
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `id` | string | да | ID состояния |
| `name` | string | да | Имя спринта |
| `begin` | number | нет | **Начало в секундах от 01.01.1970** ⚠️ |
| `end` | number | нет | **Конец в секундах от 01.01.1970** ⚠️ |
| `deleted` | boolean | нет | Статус удаления |

> ⚠️ `begin`/`end` передаются в **секундах**, а не в миллисекундах, в отличие от `deadline` в задачах.

---

## 15. Стикеры (Stickers) — старый эндпоинт

> **Подтверждён**: работает. Это старый эндпоинт управления кастомными полями доски.
> Наряду с ним существуют новые типизированные эндпоинты: `/string-stickers` (см. раздел 11) и `/sprint-stickers` (см. раздел 13).

### 15.1 Получить список стикеров доски

```
GET /stickers?boardId=uuid
```

**Response**: массив объектов:
| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | ID стикера |
| `title` | string | Название |
| `type` | string | `string` / `select` / `number` / `date` / `user` |
| `boardId` | string | ID доски |
| `options` | array | Для `select`: массив `{id, title, color?}` |

### 15.2 Создать стикер

```
POST /stickers
Content-Type: application/json

{ "title": "Priority", "type": "select", "boardId": "uuid", "options": [...] }
```

### 15.3 Изменить стикер

```
PUT /stickers/{id}
Content-Type: application/json

{ "title": "...", "type": "...", "options": [...] }
```

---

## 16. Верификация: анализ легаси-кода

Все выводы ниже сделаны на основе реального кода из `yougile_scripts/task-assistant/daily_review.py`.

### 16.1 Формат стикеров в задаче

Поле `stickers` у задачи — это `{ stickerUUID: value }`, где `value` зависит от типа:

| Тип стикера | Формат value | Пример из легаси |
|-------------|-------------|------------------|
| `select` | UUID выбранной опции | `"659a6c507fc7"` (High) |
| `string` | произвольный текст | `"@Институт"`, `"Сдать Матвеева"` |
| `number` | число | `50` |

### 16.2 Формат deadline

```python
# Чтение из ответа API
dl = t.get("deadline", {}).get("deadline")  # → 1736637798215 (ms)

# Сравнение — timestamp в ms
now_ms = int(time.time() * 1000)
is_overdue = dl < now_ms

# Конвертация в дни
overdue_days = (now_ms - dl) / (86400 * 1000)
```

- `deadline` — объект `{ deadline: timestamp_ms }`
- Значение — **timestamp в миллисекундах**

### 16.3 Пагинация в легаси

Ни один скрипт не обрабатывает `paging.next`. Всегда берётся первая страница (`limit=100`). Клиент MCP должен сам управлять `offset`: увеличивать на `limit`, пока `next === true`.

### 16.4 Чего нет в легаси, но есть в OpenAPI spec

- `/users/me`, `/users/{id}` (GET/PUT/DELETE) — не вызывались
- `/projects/{id}` (GET/PUT) — не вызывались
- `/departments` — не вызывались
- `/companies` — не вызывались
- `/upload-file` — не вызывался
- `/webhooks` — не вызывались
- `/group-chats`, `/chats/{chatId}/messages` — не вызывались
- `/task-list` (прямой порядок) — не вызывался
- `/tasks/{id}/chat-subscribers` — не вызывался

### 16.5 Скрипты, упомянутые в SKILL.md, но не существующие

| Упомянут | Статус |
|----------|--------|
| `list-projects.py` | ❌ не существует |
| `list-boards.py` | ❌ не существует |
| `list-columns.py` | ❌ не существует |
| `create-task.py` | ❌ не существует |
| `update-task.py` | ❌ не существует |
| `create-board.py` | ✅ существует |
| `list-tasks.py` | ✅ существует |

---

## 17. Пагинация (общая)

**PagingMetadata** (все поля обязательные):
| Поле | Тип | Описание |
|------|-----|----------|
| `count` | number | Количество элементов в ответе |
| `limit` | number | Максимум элементов на страницу |
| `offset` | number | Индекс первого элемента |
| `next` | boolean | `true` если есть ещё страницы |

Формат ответа list-эндпоинтов:
```json
{
  "paging": { "count": 50, "limit": 100, "offset": 0, "next": true },
  "content": [ ... ]
}
```

---

## 18. Вспомогательные DTO

### Deadline
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `deadline` | number | да | Timestamp в миллисекундах |
| `blockedPoints` | [string] | да | `["Начало"]` или `["Конец"]` |
| `links` | [string] | да | Связанные задачи |
| `startDate` | number | нет | Timestamp начала |
| `withTime` | boolean | нет | Показывать время |
| `history` | [string] | нет | История изменений |

### UpdateDeadline
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `deadline` | number | нет | Timestamp ms |
| `blockedPoints` | [string] | **да** | |
| `links` | [string] | **да** | |
| `deleted` | boolean | нет | `true` = открепить стикер |
| `empty` | boolean | нет | `true` = прикрепить без значения |
| `startDate` | number | нет | |
| `withTime` | boolean | нет | |
| `history` | [string] | нет | |

### CheckList
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | да | Название списка |
| `items` | [CheckListItem] | да | Элементы |

### CheckListItem
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `title` | string | да | Название |
| `isCompleted` | boolean | да | Выполнен |

### Stopwatch
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `running` | boolean | да | Статус |
| `seconds` | number | да | Прошло секунд |
| `atMoment` | number | да | Момент замера |

### Timer
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `running` | boolean | да | Статус |
| `seconds` | number | да | Осталось секунд |
| `since` | number | да | Timestamp отсчёта |

### TimeTracking
| Поле | Тип | Обязательное | Описание |
|------|-----|-------------|----------|
| `plan` | number | да | Запланировано часов |
| `work` | number | да | Затрачено часов |

---

## 19. Неопределённости (требуют проверки на реальном API)

1. **`/companies{*companyId}`** — формат URL неясен: нужен слеш или нет? (spec: `/companies09a0fdb3...`). В легаси не используется.
2. **`custom` в StickersDto доски** — что возвращает поле `stickers.custom` в ответе `GET /boards/{id}`? Легаси не показывает.
3. ~~**Старый `/stickers` vs новый `/string-stickers`**~~ — **частично закрыто (2026-08-23)**: легенда с опциями (`states` с короткими hex-ID типа `659a6c507fc7`, НЕ UUID) доступна только через `GET /string-stickers?boardId=`; старый `GET /stickers` возвращает id/тип без опций. В задаче значение select-стикера = ID состояния (короткий hex). Репозиторий берёт легенду из `/string-stickers`, значения в `PUT /tasks/{id}` — `{"stickers": {"<stickerId>": "<stateId>"}}` — проверено живым вызовом.
4. **`/upload-file`** — исключён из плана как ненужный.
5. **Формат ответа при ошибке** — что приходит в body при 400/401/404/429? OpenAPI не специфицирует.
6. ~~**`GET /tasks?boardId=` без `columnId`**~~ — **закрыто (2026-08-23)**: API возвращает `400 "property boardId should not exist"`; `/tasks` принимает только `columnId`. Полный обход доски = обязательно по колонкам (см. §10.1).
