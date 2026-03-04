package places

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Place struct {
	ID      int64   `json:"id"`
	// Name is the default display name (usually Ukrainian). It mirrors NameUK.
	Name    string  `json:"name"`
	NameUK  string  `json:"name_uk"`
	NameRU  string  `json:"name_ru"`
	Type    string  `json:"type"`
	Oblast  string  `json:"oblast"`
	Raion   *string `json:"raion,omitempty"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type Store struct {
	db     *sql.DB
	getStmt *sql.Stmt
	useFTS bool

	cache *searchCache
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL", path))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	// diagnostics: log tables, schema and example row
	logDBDiagnostics(db)

	useFTS := ensureFTS(db)

	getStmt, err := db.Prepare(`
		SELECT id, name_uk, name_ru, type, oblast, raion, lat, lon
		FROM places
		WHERE id = ?
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{
		db:     db,
		getStmt: getStmt,
		useFTS: useFTS,
		cache:  newSearchCache(256, 60*time.Second),
	}, nil
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var err error
	if s.getStmt != nil {
		_ = s.getStmt.Close()
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	return err
}

func (s *Store) Search(ctx context.Context, q string, limit int) ([]Place, error) {
	if s == nil {
		return nil, errors.New("places store not initialized")
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return []Place{}, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	qNorm := strings.ToLower(q)
	if len([]rune(qNorm)) < 2 {
		return []Place{}, nil
	}

	qLatin := Normalize(q)
	if qLatin == "" {
		return []Place{}, nil
	}

	key := fmt.Sprintf("%s|%s|%d", qNorm, qLatin, limit)
	if cached, ok := s.cache.Get(key); ok {
		return cached, nil
	}

	// 1) попробовать FTS5
	var result []Place
	var err error
	if s.useFTS {
		result, err = s.searchFTS(ctx, qNorm, qLatin, limit)
		if err != nil {
			log.Printf("[places] FTS search failed, fallback to LIKE: %v", err)
		}
	}

	// 2) если FTS отключён или ничего не нашлось — fallback LIKE
	if !s.useFTS || len(result) == 0 {
		result, err = s.searchFallbackLike(ctx, qNorm, qLatin, limit)
		if err != nil {
			return nil, err
		}
	}

	s.cache.Set(key, result)
	return result, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (*Place, error) {
	if s == nil {
		return nil, errors.New("places store not initialized")
	}

	var p Place
	var raion sql.NullString
	var nameRU sql.NullString
	err := s.getStmt.QueryRowContext(ctx, id).Scan(
		&p.ID,
		&p.NameUK,
		&nameRU,
		&p.Type,
		&p.Oblast,
		&raion,
		&p.Lat,
		&p.Lon,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Name = p.NameUK
	if nameRU.Valid {
		p.NameRU = nameRU.String
	}
	if raion.Valid {
		v := raion.String
		p.Raion = &v
	}
	return &p, nil
}

// Nearest находит ближайший населённый пункт к заданным координатам.
func (s *Store) Nearest(ctx context.Context, lat, lon float64) (*Place, error) {
	if s == nil {
		return nil, errors.New("places store not initialized")
	}
	const sqlNearest = `
SELECT id, name_uk, name_ru, type, oblast, raion, lat, lon
FROM places
ORDER BY ((lat - ?) * (lat - ?) + (lon - ?) * (lon - ?)) ASC
LIMIT 1;
`
	row := s.db.QueryRowContext(ctx, sqlNearest, lat, lat, lon, lon)
	var p Place
	var raion sql.NullString
	var nameRU sql.NullString
	if err := row.Scan(
		&p.ID,
		&p.NameUK,
		&nameRU,
		&p.Type,
		&p.Oblast,
		&raion,
		&p.Lat,
		&p.Lon,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.Name = p.NameUK
	if nameRU.Valid {
		p.NameRU = nameRU.String
	}
	if raion.Valid {
		v := raion.String
		p.Raion = &v
	}
	return &p, nil
}

// --- search cache ---

type searchCache struct {
	mu       sync.Mutex
	items    map[string]cacheEntry
	capacity int
	ttl      time.Duration
}

type cacheEntry struct {
	value     []Place
	expiresAt time.Time
}

func newSearchCache(capacity int, ttl time.Duration) *searchCache {
	return &searchCache{
		items:    make(map[string]cacheEntry),
		capacity: capacity,
		ttl:      ttl,
	}
}

func (c *searchCache) Get(key string) ([]Place, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.items == nil {
		return nil, false
	}

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.items, key)
		return nil, false
	}
	return entry.value, true
}

func (c *searchCache) Set(key string, value []Place) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.items == nil {
		c.items = make(map[string]cacheEntry)
	}

	if len(c.items) >= c.capacity {
		for k := range c.items {
			delete(c.items, k)
			break
		}
	}

	c.items[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Normalize нормализует строку для поиска:
// trim + lowercase, убирает пробелы/дефисы/апострофы и сглаживает укр/рус буквы.
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// унифицируем типы апострофов
	s = strings.ReplaceAll(s, "’", "'")

	replacer := strings.NewReplacer(
		" ", "",
		"-", "",
		"'", "",
		"ʼ", "",
		"`", "",
		"ё", "е",
	)
	s = replacer.Replace(s)

	// кросс-языковая унификация
	s = strings.ReplaceAll(s, "і", "и")
	s = strings.ReplaceAll(s, "ї", "и")
	s = strings.ReplaceAll(s, "є", "е")
	s = strings.ReplaceAll(s, "ґ", "г")
	s = strings.ReplaceAll(s, "й", "и")
	s = strings.ReplaceAll(s, "ъ", "")
	s = strings.ReplaceAll(s, "ь", "")

	return s
}

// TranslitLatin выполняет простую транслитерацию кириллицы в латиницу.
// Достаточно для человекочитаемых EN‑имен городов в подсказках.
func TranslitLatin(s string) string {
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"А", "A", "а", "a",
		"Б", "B", "б", "b",
		"В", "V", "в", "v",
		"Г", "H", "г", "h", // укр. г
		"Ґ", "G", "ґ", "g",
		"Д", "D", "д", "d",
		"Е", "E", "е", "e",
		"Ё", "Yo", "ё", "yo",
		"Є", "Ye", "є", "ie",
		"Ж", "Zh", "ж", "zh",
		"З", "Z", "з", "z",
		"И", "Y", "и", "y",
		"І", "I", "і", "i",
		"Ї", "Yi", "ї", "i",
		"Й", "Y", "й", "y",
		"К", "K", "к", "k",
		"Л", "L", "л", "l",
		"М", "M", "м", "m",
		"Н", "N", "н", "n",
		"О", "O", "о", "o",
		"П", "P", "п", "p",
		"Р", "R", "р", "r",
		"С", "S", "с", "s",
		"Т", "T", "т", "t",
		"У", "U", "у", "u",
		"Ф", "F", "ф", "f",
		"Х", "Kh", "х", "kh",
		"Ц", "Ts", "ц", "ts",
		"Ч", "Ch", "ч", "ch",
		"Ш", "Sh", "ш", "sh",
		"Щ", "Shch", "щ", "shch",
		"Ю", "Yu", "ю", "yu",
		"Я", "Ya", "я", "ya",
		"Ъ", "", "ъ", "",
		"Ь", "", "ь", "",
	)
	return replacer.Replace(s)
}

// ensureFTS настраивает FTS5-индекс и триггеры, если они доступны.
// Возвращает true, если FTS5 включён и готов к использованию.
func ensureFTS(db *sql.DB) bool {
	// проверить поддержку FTS5 и создать виртуальную таблицу
	_, err := db.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS places_fts USING fts5(
	name_uk,
	name_ru,
	oblast,
	raion,
	search_name,
	content='places',
	content_rowid='id',
	tokenize='unicode61 remove_diacritics 2'
);
`)
	if err != nil {
		if strings.Contains(err.Error(), "no such module: fts5") {
			log.Printf("[places] FTS5 module not available, falling back to LIKE search: %v", err)
			return false
		}
		log.Printf("[places] create FTS5 table failed, falling back to LIKE: %v", err)
		return false
	}

	// триггеры синхронизации
	if _, err := db.Exec(`
CREATE TRIGGER IF NOT EXISTS places_fts_ai AFTER INSERT ON places BEGIN
	INSERT INTO places_fts(rowid, name_uk, name_ru, oblast, raion, search_name)
	VALUES (new.id, new.name_uk, new.name_ru, new.oblast, new.raion, new.search_name);
END;
CREATE TRIGGER IF NOT EXISTS places_fts_ad AFTER DELETE ON places BEGIN
	DELETE FROM places_fts WHERE rowid = old.id;
END;
CREATE TRIGGER IF NOT EXISTS places_fts_au AFTER UPDATE ON places BEGIN
	UPDATE places_fts SET
		name_uk = new.name_uk,
		name_ru = new.name_ru,
		oblast = new.oblast,
		raion = new.raion,
		search_name = new.search_name
	WHERE rowid = new.id;
END;
`); err != nil {
		log.Printf("[places] create FTS5 triggers failed (will still try to use FTS): %v", err)
	}

	// если FTS-поисковый индекс пустой, а основная таблица нет — заполнить
	var baseCount, ftsCount int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM places`).Scan(&baseCount); err != nil {
		log.Printf("[places] count(places) failed: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM places_fts`).Scan(&ftsCount); err != nil {
		log.Printf("[places] count(places_fts) failed: %v", err)
	}
	if baseCount > 0 && ftsCount == 0 {
		log.Printf("[places] populating FTS index from %d rows", baseCount)
		if _, err := db.Exec(`
INSERT INTO places_fts(rowid, name_uk, name_ru, oblast, raion, search_name)
SELECT id, name_uk, name_ru, oblast, raion, search_name FROM places;
`); err != nil {
			log.Printf("[places] populate FTS index failed: %v", err)
		}
	}

	log.Printf("[places] FTS5 enabled (places=%d, places_fts=%d)", baseCount, ftsCount)
	return true
}

// searchFTS выполняет полнотекстовый поиск по places_fts с match-запросом.
func (s *Store) searchFTS(ctx context.Context, qNorm, qLatin string, limit int) ([]Place, error) {
	// строим FTS-строку запроса: токены + префиксные wildcard
	tokens := strings.Fields(qNorm)
	if len(tokens) == 0 {
		return []Place{}, nil
	}
	var cleaned []string
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		// убираем спецсимволы FTS
		t = strings.Map(func(r rune) rune {
			switch r {
			case '"', '\'', '*', ':':
				return -1
			default:
				return r
			}
		}, t)
		if t != "" {
			cleaned = append(cleaned, t+"*")
		}
	}
	if len(cleaned) == 0 {
		return []Place{}, nil
	}
	matchQuery := strings.Join(cleaned, " ")

	const sqlFTS = `
SELECT p.id, p.name_uk, p.name_ru, p.type, p.oblast, p.raion, p.lat, p.lon
FROM places_fts f
JOIN places p ON p.id = f.rowid
WHERE f MATCH ?
ORDER BY bm25(f) ASC
LIMIT ?;
`
	rows, err := s.db.QueryContext(ctx, sqlFTS, matchQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Place
	for rows.Next() {
		var p Place
		var raion sql.NullString
		var nameRU sql.NullString
		if err := rows.Scan(
			&p.ID,
			&p.NameUK,
			&nameRU,
			&p.Type,
			&p.Oblast,
			&raion,
			&p.Lat,
			&p.Lon,
		); err != nil {
			return nil, err
		}
		p.Name = p.NameUK
		if nameRU.Valid {
			p.NameRU = nameRU.String
		}
		if raion.Valid {
			v := raion.String
			p.Raion = &v
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// searchFallbackLike — подстрочный LIKE-поиск по названиям и search_name.
func (s *Store) searchFallbackLike(ctx context.Context, qNorm, qLatin string, limit int) ([]Place, error) {
	const sqlLike = `
SELECT id, name_uk, name_ru, type, oblast, raion, lat, lon
FROM places
WHERE
    lower(name_uk) LIKE '%' || ? || '%'
    OR lower(name_ru) LIKE '%' || ? || '%'
    OR search_name LIKE '%' || ? || '%'
ORDER BY LENGTH(name_uk), name_uk
LIMIT ?;
`
	rows, err := s.db.QueryContext(ctx, sqlLike, qNorm, qNorm, qLatin, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Place
	for rows.Next() {
		var p Place
		var raion sql.NullString
		var nameRU sql.NullString
		if err := rows.Scan(
			&p.ID,
			&p.NameUK,
			&nameRU,
			&p.Type,
			&p.Oblast,
			&raion,
			&p.Lat,
			&p.Lon,
		); err != nil {
			return nil, err
		}
		p.Name = p.NameUK
		if nameRU.Valid {
			p.NameRU = nameRU.String
		}
		if raion.Valid {
			v := raion.String
			p.Raion = &v
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// logDBDiagnostics выводит структуру таблиц и пример строки для быстрой отладки.
func logDBDiagnostics(db *sql.DB) {
	// список таблиц
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		log.Printf("[places] sqlite_master query failed: %v", err)
		return
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Printf("[places] scan table name failed: %v", err)
			continue
		}
		tables = append(tables, name)
	}
	rows.Close()
	log.Printf("[places] tables: %v", tables)

	// выбираем основную таблицу
	table := "places"
	found := false
	for _, t := range tables {
		if t == table {
			found = true
			break
		}
	}
	if !found && len(tables) > 0 {
		table = tables[0]
	}

	// schema через PRAGMA
	if tri, err := db.Query("PRAGMA table_info(" + table + ")"); err == nil {
		defer tri.Close()
		var cols []string
		for tri.Next() {
			var (
				cid    int
				name   string
				ctype  string
				notnull int
				dflt   sql.NullString
				pk     int
			)
			if err := tri.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				log.Printf("[places] table_info scan failed: %v", err)
				break
			}
			cols = append(cols, fmt.Sprintf("%s %s", name, ctype))
		}
		log.Printf("[places] %s columns: %v", table, cols)
	} else if err != nil {
		log.Printf("[places] PRAGMA table_info failed for %s: %v", table, err)
	}

	// COUNT(*)
	var cnt int64
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&cnt); err == nil {
		log.Printf("[places] %s count=%d", table, cnt)
	} else {
		log.Printf("[places] count query failed for %s: %v", table, err)
	}

	// пример строки
	if cnt > 0 {
		// достаём до 8 колонок как срез строк для простого лога
		row := db.QueryRow("SELECT * FROM " + table + " LIMIT 1")
		// узнаём количество колонок через pragma ещё раз
		tri, err := db.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			log.Printf("[places] table_info for sample failed: %v", err)
			return
		}
		var colNames []string
		for tri.Next() {
			var (
				cid    int
				name   string
				ctype  string
				notnull int
				dflt   sql.NullString
				pk     int
			)
			if err := tri.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				break
			}
			colNames = append(colNames, name)
		}
		tri.Close()
		if len(colNames) == 0 {
			return
		}
		dest := make([]interface{}, len(colNames))
		for i := range dest {
			var v interface{}
			dest[i] = &v
		}
		if err := row.Scan(dest...); err != nil {
			log.Printf("[places] sample row scan failed: %v", err)
			return
		}
		sample := make(map[string]interface{}, len(colNames))
		for i, name := range colNames {
			sample[name] = *(dest[i].(*interface{}))
		}
		log.Printf("[places] sample row from %s: %+v", table, sample)
	}
}

