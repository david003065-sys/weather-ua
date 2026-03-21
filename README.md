# ЖИВАЯ ПОГОДА — премиальный погодный сайт для городов Украины

Серверный рендеринг на Go, живая анимированная погода (CSS + JS), данные из **WeatherAPI.com**
(переменные `WEATHER_API_PROVIDER=weatherapi`, `WEATHERAPI_KEY`) и кеширование в памяти. Города: Днепр, Киев, Павлоград, Вольногорск.

## Стек

- Go (`net/http`, `html/template`)
- WeatherAPI.com: `WEATHER_API_PROVIDER=weatherapi` (по умолчанию), `WEATHERAPI_KEY`, опционально `WEATHERAPI_BASE_URL`
- Чистый CSS (glassmorphism, анимированный фон)
- Немного JS для смены фонового состояния и инициализации Chart.js

## Структура проекта

- `cmd/server` — точка входа (пакет `main`): настройка маршрутов, статики и запуск (`Run()`).
- `cmd/tools/places_importer` — утилита генерации `data/places.db` из CSV.
- `internal/bootstrap` — автоматическая подготовка данных (скачивает GeoNames и создаёт `data/places.db`, если его нет).
- `internal/weather` — клиент WeatherAPI.com, кеш, список городов, доменные типы.
- `internal/handlers` — HTTP‑обработчики, подготовка данных для шаблонов.
- `internal/places` — офлайн‑поиск по населенным пунктам Украины (SQLite); нормализация типа НП из GeoNames — `NormalizeSettlementType` в `settlement_type.go`.
- `templates/` — SSR‑шаблоны (`layout.html`, `index.html`, `city.html`).
- `static/` — `style.css` (база), **`atmosphere.css`** (иммерсивный UI поверх), `script.js`, `pwa.js`, `favicon.svg`; PWA: `manifest.json` как **`/manifest.webmanifest`**, SW — **`/sw.js`**. Иконки: **`go run ./cmd/tools/gen_pwa_icons`**.
- `data/` — база `places.db` с населенными пунктами (создаётся отдельно).

## Как запустить локально

### Быстрый env setup (локально + Render)

- Локально приложение автоматически пытается загрузить `.env` (через `godotenv`) перед чтением переменных окружения.
- На Render `.env` не используется: переменные берутся из Environment сервиса.
- Обязательная переменная: `WEATHERAPI_KEY` (должна быть задана как **Secret** в Render).
- `WEATHER_API_PROVIDER` по умолчанию `weatherapi`, но рекомендуется явно задать `WEATHER_API_PROVIDER=weatherapi`.

1. Убедитесь, что установлен Go (1.22+).
2. Перейдите в папку проекта:

   ```bash
   cd "c:\Users\Laptopchik\OneDrive\Desktop\BSS"
   ```

3. Обновите зависимости (на всякий случай):

   ```bash
   go mod tidy
   ```

4. Создайте локальный `.env` (можно копировать из `.env.example`) и задайте провайдера/ключ API:

   Пример `.env`:

   ```env
   WEATHER_API_PROVIDER=weatherapi
   WEATHERAPI_KEY=your_key_here
   ```

   Также можно задать в shell:

   **PowerShell (Windows):**

   ```powershell
   $env:WEATHER_API_PROVIDER = "weatherapi"
   $env:WEATHERAPI_KEY = "ваш_ключ_с_weatherapi.com"
   ```

   **bash:**

   ```bash
   export WEATHER_API_PROVIDER=weatherapi
   export WEATHERAPI_KEY="your_key"
   ```

   Опционально: `WEATHERAPI_BASE_URL` (по умолчанию `https://api.weatherapi.com/v1`). Другие значения `WEATHER_API_PROVIDER` пока не поддерживаются.

5. Запустите сервер:

   ```bash
   go run ./cmd/server
   ```

   При первом запуске, если файла `data/places.db` ещё нет, сервер автоматически:

   - скачает необходимые дампы GeoNames (`UA.zip`, `alternateNamesV2.zip`, `admin1CodesASCII.txt`),
   - сгенерирует CSV с городами Украины,
   - создаст SQLite‑базу `data/places.db` с таблицей `places` и индексом по `search_name`.

   В зависимости от скорости сети шаг с загрузкой GeoNames может занять несколько минут, это нормальное поведение.

6. Откройте в браузере:

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
   go build -o app ./cmd/server
   ```

   - **Start Command**:

   ```bash
   ./app
   ```

   - **Instance Type**: Free (бесплатный план).

5. В **Environment** добавьте `WEATHER_API_PROVIDER=weatherapi` и `WEATHERAPI_KEY` (ключ с [weatherapi.com](https://www.weatherapi.com/)).
   При необходимости — `WEATHERAPI_BASE_URL`.

6. Сохраните и запустите деплой.

Render автоматически передаст переменную окружения `PORT`, сервер читает её в
`cmd/server/server.go`, так что дополнительная настройка порта не нужна.

`WEATHERAPI_KEY` храните в Render только как Secret env var (без коммита в репозиторий).

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

- Данные по каждому городу кешируются в памяти на **15 минут** (см. `weather.NewClient` в `cmd/server/server.go`).
- При ошибке или лимите WeatherAPI, если в кеше есть валидный ответ, отдаётся **устаревший** кэш.
- Коды условий WeatherAPI маппятся на WMO‑подобные коды для тех же описаний и иконок в UI
  (☀️, ☁️, 🌧, ❄️ и т.д.).

## Автоматическая тема (light/dark)

Тема оформления (светлая/тёмная) выбирается автоматически по времени суток
для выбранного города. При рендеринге страницы сервер передаёт в скрипт
`static/theme.js` данные о восходе/закате (`sunrise`, `sunset`) и смещение
таймзоны (`utc_offset_seconds`) из ответа WeatherAPI. На клиенте:

- режим **Auto** (по умолчанию) использует локальное время города и границу
  `восход–закат` для выбора темы;
- если данных по солнцу нет, применяется fallback: тёмная тема с 20:00 до 06:00;
- пользователь может явно выбрать режим **Light/Dark/Auto** в переключателе
  в шапке, выбор сохраняется в `localStorage` (`weather:themeMode`) и
  применяется на всех страницах.

## База населённых пунктов Украины (поиск и автодополнение)

Приложение поддерживает офлайн‑поиск по всем населённым пунктам Украины через SQLite.
По умолчанию база `data/places.db` создаётся **автоматически** при первом запуске сервера:
пакет `internal/bootstrap` скачивает необходимые дампы GeoNames, генерирует CSV с городами
и импортирует его в SQLite. Никакой ручной подготовки данных делать не нужно.

После импорта в БД выставляется `PRAGMA user_version` (см. `places.SettlementTypeSchemaVersion`):
при обновлении правил типов версия увеличивается, и при следующем старте сервер пересоберёт `places.db`.

### Тип населённого пункта (GeoNames → `places.type` → UI)

Источник ошибки «город показан как посёлок»: в GeoNames код **PPL** означает любой населённый пункт,
не только село/СМТ; многие украинские **міста** (в т.ч. Вольногорськ) идут как **PPL** без **PPLA\***,
а старая логика записывала все **PPL** как `селище` → в RU UI это «посёлок». Код **PPLC** (столица)
ошибочно маппился в `село`.

Сейчас нормализация в `internal/places/settlement_type.go`:

- **PPLC**, **PPLA**, **PPLA2**, **PPLA3**, **PPLA4** → `місто` (столица / админцентры).
- **PPL** → по полю **population** из `UA.txt`: ≥ 5000 — `місто`, 500–4999 — `селище`, иначе — `село`.

В шаблонах и API тип из БД переводится в `handlers.deriveTypeNames` (`місто` / `город` / `city` и т.д.).

### Ручное управление данными (опционально)

Если вы хотите полностью контролировать входные данные (например, использовать
собственный CSV вместо GeoNames), можно собрать базу самостоятельно.

1. Подготовьте CSV `data/source/places.csv` с колонками (разделитель `;`):

   - `name_uk` — название населённого пункта на украинском (обязательное поле).
   - `name_ru` — название на русском (опционально).
   - `oblast` — область (обязательное поле).
   - `raion` — район / громада (опционально).
   - `type` — тип для БД: желательно `місто`, `селище` или `село` (как после GeoNames; «смт» вручную задайте как `селище`).
   - `lat` — широта.
   - `lon` — долгота.

2. Сгенерируйте `data/places.db` с помощью импортера:

   ```bash
   go run ./cmd/tools/places_importer \
     -input data/source/places.csv \
     -output data/places.db
   ```

3. Перезапустите сервер и проверьте поиск:

   ```bash
   curl "http://localhost:8080/api/places?q=льв&limit=5&lang=uk"
   curl "http://localhost:8080/api/places?q=киев&limit=5&lang=ru"
   ```

   В ответе должны быть объекты с корректными `name_uk/name_ru` и `oblast_*`.

## Города Украины из GeoNames (генерация CSV, advanced)

Для тонкой настройки составного CSV (например, если вы хотите пересобрать его с другим
набором фильтров) можно воспользоваться отдельным инструментом.

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
