package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	slog.Info("opened SQLite database", "path", dbPath)

	// Применяем миграции (аналогично первому проекту)
	if err := applyMigrations(db); err != nil {
		return nil, err
	}

	return &SQLiteStorage{db: db}, nil
}

// applyMigrations — самодельный механизм миграций (без внешней библиотеки)
func applyMigrations(db *sql.DB) error {
	// Создаём таблицу для версионирования
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			dirty   BOOLEAN NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	// Получаем текущую версию
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations WHERE dirty = 0").Scan(&currentVersion)
	if err != nil {
		return err
	}

	// Читаем все .up.sql файлы
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var applied int
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		var version int
		n, _ := fmt.Sscanf(name, "%d", &version)
		if n != 1 {
			continue
		}
		if version <= currentVersion {
			continue
		}
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", name, err)
		}
		_, err = tx.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (?, 0)", version)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		slog.Info("migration applied", "file", name)
		applied++
	}
	if applied == 0 {
		slog.Info("no new migrations to apply")
	} else {
		slog.Info("all migrations applied", "count", applied)
	}
	return nil
}

// CreateSite добавляет новый сайт в базу данных.
// Принимает указатель на Site, заполняет его поле ID (автогенерируемое значение)
// и возвращает ошибку, если что-то пошло не так.
func (s *SQLiteStorage) CreateSite(site *Site) error {
	// 1. Подготавливаем SQL-запрос с возвратом сгенерированного ID
	query := `
        INSERT INTO sites (url, interval_sec)
        VALUES (?, ?)
    `
	// 2. Выполняем запрос, передавая значения полей
	result, err := s.db.Exec(query, site.URL, site.IntervalSec)
	if err != nil {
		return fmt.Errorf("failed to insert site: %w", err)
	}

	// 3. Получаем ID последней вставленной записи
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// 4. Присваиваем ID переданной структуре
	site.ID = int(id)

	return nil
}

// GetSiteByID возвращает сайт по его ID.
// Если сайт не найден, возвращает (nil, nil) — без ошибки.
// В случае ошибки БД возвращает (nil, error).
func (s *SQLiteStorage) GetSiteByID(id int) (*Site, error) {
	// 1. Пишем SQL-запрос
	query := `
        SELECT id, url, interval_sec, 
               COALESCE(last_check, '') as last_check,
               COALESCE(last_status, 0) as last_status,
               COALESCE(response_time, 0) as response_time,
               created_at
        FROM sites
        WHERE id = ?
    `
	// 2. Выполняем запрос с параметром id
	row := s.db.QueryRow(query, id)

	// 3. Создаём пустую структуру для сканирования
	var site Site
	// Объявляем переменные для nullable полей (используем sql.NullTime и sql.NullInt64)
	var lastCheck sql.NullTime
	var lastStatus sql.NullInt64
	var responseTime sql.NullInt64

	// 4. Сканируем строку в переменные
	err := row.Scan(
		&site.ID,
		&site.URL,
		&site.IntervalSec,
		&lastCheck,
		&lastStatus,
		&responseTime,
		&site.CreatedAt,
	)
	if err != nil {
		// Если запись не найдена, sql.ErrNoRows — это не ошибка, а отсутствие данных
		if err == sql.ErrNoRows {
			return nil, nil // нет записи, но ошибки нет
		}
		return nil, fmt.Errorf("failed to scan site: %w", err)
	}

	// 5. Преобразуем nullable поля в указатели (для JSON-сериализации)
	if lastCheck.Valid {
		site.LastCheck = &lastCheck.Time
	}
	if lastStatus.Valid {
		status := int(lastStatus.Int64)
		site.LastStatus = &status
	}
	if responseTime.Valid {
		rt := int(responseTime.Int64)
		site.ResponseTime = &rt
	}

	return &site, nil
}
