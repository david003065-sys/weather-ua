# ЖИВАЯ ПОГОДА — премиальный погодный сайт для городов Украины

Серверный рендеринг на Go, живая анимированная погода (CSS + JS), данные из Open‑Meteo
и кеширование в памяти. Города: Днепр, Киев, Павлоград, Вольногорск.

## Стек

- Go (`net/http`, `html/template`)
- Open‑Meteo API (без ключа, бесплатный)
- Чистый CSS (glassmorphism, анимированный фон)
- Немного JS для смены фонового состояния и инициализации Chart.js

## Структура проекта

- `cmd/server` — точка входа (пакет `main`): настройка маршрутов, статики и запуск (`Run()`).
- `cmd/tools/places_importer` — утилита генерации `data/places.db` из CSV.
- `internal/weather` — клиент Open‑Meteo, кеш, список городов, доменные типы.
- `internal/handlers` — HTTP‑обработчики, подготовка данных для шаблонов.
- `internal/places` — офлайн‑поиск по населенным пунктам Украины (SQLite).
- `templates/` — SSR‑шаблоны (`layout.html`, `index.html`, `city.html`).
- `static/` — стили, JS и favicon (`style.css`, `script.js`, `favicon.svg`).
- `data/` — база `places.db` с населенными пунктами (создаётся отдельно).

## Как запустить локально

1. Убедитесь, что установлен Go (1.22+).
2. Перейдите в папку проекта:

   ```bash
   cd "c:\Users\Laptopchik\OneDrive\Desktop\BSS"
   ```

3. Обновите зависимости (на всякий случай):

   ```bash
   go mod tidy
   ```

4. Запустите сервер:

   ```bash
   go run ./cmd/server
   ```

5. Откройте в браузере:

   ```text
   http://localhost:8080
   ```

Маршруты:

- `/` — главная с живым фоном и карточками 4 городов.
- `/city/dnipro`
- `/city/kyiv`
- `/city/pavlograd`
- `/city/volnogorsk`

## Как задеплоить на Render (бесплатный план)

### 1. Подготовка репозитория

1. Инициализируйте git в папке проекта (если ещё не сделали):

   ```bash
   cd "c:\Users\Laptopchik\OneDrive\Desktop\BSS"
   git init
   git add .
   git commit -m "Initial live weather app"
   ```

2. Создайте репозиторий на GitHub (например, `live-weather-ua`) **без** автогенерации файлов.

3. Свяжите локальный и удалённый репозиторий:

   ```bash
   git remote add origin https://github.com/<your-username>/live-weather-ua.git
   git branch -M main
   git push -u origin main
   ```

### 2. Создание Web Service на Render

1. Зайдите на `https://render.com` и авторизуйтесь.
2. Нажмите **New → Web Service**.
3. Выберите ваш репозиторий `live-weather-ua` с GitHub.
4. Настройки сервиса:
   - **Environment**: `Go`
   - **Region**: ближайший регион к аудитории.
   - **Branch**: `main`
   - **Build Command**:

     ```bash
     go build -o server ./cmd/server
     ```

   - **Start Command**:

     ```bash
     ./server
     ```

   - **Instance Type**: Free (бесплатный план).

5. Сохраните и запустите деплой.

Render автоматически передаст переменную окружения `PORT`, сервер читает её в
`cmd/server/server.go`, так что дополнительная настройка порта не нужна.

### 3. Публичная ссылка

После успешного деплоя Render покажет URL вида:

```text
https://live-weather-ua.onrender.com
```

Это ваш продакшен‑URL, который можно отдавать пользователям или поставить в description
репозитория на GitHub.

## Обновление приложения

1. Внесите изменения в код (`internal/`, `templates/`, `static/`).
2. Локально проверьте:

   ```bash
   go run ./cmd/server
   ```

3. Закоммитьте и запушьте:

   ```bash
   git add .
   git commit -m "Update weather UI / logic"
   git push
   ```

4. Render автоматически подтянет изменения (если включён Auto Deploy) или
   запустите деплой вручную из панели Render.

## Кеш и ограничения

- Данные по каждому городу кешируются в памяти на **10 минут**.
- При ошибке запроса к Open‑Meteo, если есть ещё актуальные данные в кеше,
  будут использованы они.
- Коды погодных условий Open‑Meteo конвертируются в понятные описания и
  иконки (☀️, ☁️, 🌧, ❄️ и т.д.).

## База населённых пунктов Украины (поиск и автодополнение)

Приложение поддерживает офлайн‑поиск по всем населённым пунктам Украины через SQLite.

### 1. Подготовить исходный файл

1. Создайте папку `data/source/` (если её нет).
2. Положите в неё CSV‑файл с колонками (разделитель `;`):

   - `name_uk` — название населённого пункта на украинском (обязательное поле).
   - `name_ru` — название на русском (опционально).
   - `oblast` — область (обязательное поле).
   - `raion` — район / громада (опционально).
   - `type` — тип (`місто`, `село`, `смт` и т.п.).
   - `lat` — широта.
   - `lon` — долгота.

   Опционально можно добавить колонку `alt_search` для альтернативного варианта поиска.

   Пример пути:

   ```text
   data/source/places.csv
   ```

### 2. Сгенерировать `data/places.db`

1. Запустите утилиту импорта:

   ```bash
   go run ./cmd/tools/places_importer \
     -input data/source/places.csv \
     -output data/places.db
   ```

2. Утилита:

   - создаст (или перезапишет) файл `data/places.db`,
   - создаст таблицу `places` и индекс по `search_name`,
   - нормализует названия (lower‑case, убирает апострофы/дефисы/пробелы, мягко унифицирует `е/є`, `и/і`).

### 3. Проверить, что поиск работает

1. Убедитесь, что сервер видит базу:

   - при старте в логах **не** должно быть ошибки `places db not found`,
   - вместо этого появится сообщение вида `places store initialized from C:\...\data\places.db`.

2. Протестируйте эндпоинт напрямую:

   ```bash
   curl "http://localhost:8080/api/places?q=льв&limit=5"
   curl "http://localhost:8080/api/places?q=киев&limit=5"
   ```

   В ответе должны быть объекты с `name` = `Львів` и `Київ` (поиск работает и по укр, и по русскому написанию).

3. Откройте главную страницу (`/`) и попробуйте:

   - начать вводить название (`Київ`, `Киев`, `Дніпро` и т.п.),
   - через 200–300 мс появится выпадающий список подсказок,
   - формат подсказки: `Назва — тип, область`,
   - выберите вариант мышкой или клавишами ↑/↓ + Enter,
   - вы попадёте на страницу `/place/{id}` с погодой по координатам выбранного населённого пункта.

## Города Украины из GeoNames (генерация CSV)

Для автоматического получения полного списка **городов Украины** (только `type = "місто"`) из GeoNames предусмотрен отдельный инструмент.

### 1. Скачать исходные файлы GeoNames

Скачайте следующие файлы с официального сайта GeoNames:

- Архив с полным дампом по Украине:

  - `UA.zip` — `https://download.geonames.org/export/dump/UA.zip`

- Альтернативные названия:

  - `alternateNamesV2.zip` — `https://download.geonames.org/export/dump/alternateNamesV2.zip`

- Административные единицы (области):

  - `admin1CodesASCII.txt` — `https://download.geonames.org/export/dump/admin1CodesASCII.txt`

### 2. Разложить файлы в проекте

1. Создайте папку:

   ```text
   data/geonames/
   ```

2. Разложите файлы так:

   - Из `UA.zip` извлеките `UA.txt` → `data/geonames/UA.txt`
   - Из `alternateNamesV2.zip` извлеките `alternateNamesV2.txt` → `data/geonames/alternateNamesV2.txt`
   - Файл `admin1CodesASCII.txt` положите в `data/geonames/admin1CodesASCII.txt`

Если какого‑то файла не будет, инструмент выведет понятную ошибку.

### 3. Сгенерировать CSV с городами

Запустите генератор:

```bash
cd "c:\Users\Laptopchik\OneDrive\Desktop\BSS"
go run ./cmd/tools/build_ua_cities_csv
```

Он:

- прочитает `UA.txt`, `admin1CodesASCII.txt`, `alternateNamesV2.txt`;
- выберет только записи с:
  - `featureClass == "P"`
  - `featureCode` в `PPL, PPLA, PPLA2, PPLA3, PPLA4, PPLC`;
- поставит:
  - `name_uk` — локальное имя из `UA.txt`,
  - `name_ru` — русское альтернативное имя из `alternateNamesV2.txt` (если нет — `name_uk`),
  - `oblast` — название области из `admin1CodesASCII.txt`,
  - `raion` — пустым,
  - `type` — всегда `"місто"`,
  - `lat` / `lon` — координаты из `UA.txt`.

Результат будет сохранён в:

```text
data/out/cities_ua.csv
```

Формат CSV:

```text
name_uk;name_ru;oblast;raion;type;lat;lon
```

Дальше вы можете:

- либо использовать `data/out/cities_ua.csv` как источник,
- либо скопировать/переименовать его в `data/source/places.csv` и прогнать утилиту `places_importer`, чтобы построить `data/places.db` для поиска.
