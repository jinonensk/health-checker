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
