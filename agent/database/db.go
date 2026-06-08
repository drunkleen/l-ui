package database

import (
	"log"
	"os"
	"path"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var db *gorm.DB

func GetDB() *gorm.DB {
	return db
}

func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		db = nil
		return sqlDB.Close()
	}
	return nil
}

func InitDB(dbPath string) error {
	var gormLogger gormlogger.Interface
	gormLogger = gormlogger.Discard

	dir := path.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_txlock=immediate"
	var err error
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormLogger, DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=10000"); err != nil {
		return err
	}
	if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return err
	}

	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	models := []any{
		&NodeConfig{},
		&NodeSecret{},
		&MetricsSnapshot{},
	}
	for _, mdl := range models {
		if err := db.AutoMigrate(mdl); err != nil {
			log.Printf("Error auto migrating model %T: %v", mdl, err)
			return err
		}
	}

	return nil
}
