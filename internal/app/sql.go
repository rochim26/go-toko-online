package app

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenSQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(5)
	return db, nil
}
