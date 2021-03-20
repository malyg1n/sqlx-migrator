package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/malyg1n/sql-migrator/internal/entity"
)

type dBContract interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Begin() (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Close() error
}

type MigrationsStore struct {
	db        dBContract
	tableName string
}

func NewStore(db dBContract, tableName string) *MigrationsStore {
	return &MigrationsStore{
		db:        db,
		tableName: tableName,
	}
}

func (s *MigrationsStore) CreateMigrationsTable(query string) error {
	_, err := s.db.Exec(query)
	return err
}

func (s *MigrationsStore) GetMigrations() ([]*entity.MigrationEntity, error) {
	var migrations []*entity.MigrationEntity
	query := fmt.Sprintf("SELECT * from %s ORDER BY created_at DESC", s.tableName)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		me := &entity.MigrationEntity{}
		if err := rows.Scan(&me.Id, &me.Migration, &me.Version, &me.CreatedAt); err != nil {
			return nil, err
		}
		migrations = append(migrations, me)
	}

	return migrations, nil
}

func (s *MigrationsStore) GetMigrationsByVersion(version uint) ([]*entity.MigrationEntity, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE version=%d ORDER BY created_at DESC", s.tableName, version)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	var migrations []*entity.MigrationEntity
	for rows.Next() {
		me := &entity.MigrationEntity{}
		if err := rows.Scan(&me.Id, &me.Migration, &me.Version, &me.CreatedAt); err != nil {
			return nil, err
		}
		migrations = append(migrations, me)
	}

	return migrations, nil
}

func (s *MigrationsStore) GetLatestVersionNumber() (uint, error) {
	var versionNumber uint
	me := &entity.MigrationEntity{}
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY version DESC limit 1", s.tableName)
	row := s.db.QueryRow(query)
	if row == nil {
		versionNumber = 0
	} else {
		err := row.Scan(&me.Id, &me.Migration, &me.Version, &me.CreatedAt)
		if me.Version == 0 || err != nil {
			return 0, nil
		}
		versionNumber = me.Version
	}
	return versionNumber, nil
}

func (s *MigrationsStore) ApplyMigrationsUp(migrations []*entity.MigrationEntity) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		_, err := s.db.Exec(m.Query)
		if err != nil {
			e := tx.Rollback()
			if e != nil {
				return e
			}
			return err
		}

		migrationQuery := fmt.Sprintf("INSERT INTO %s (migration, version) VALUES ($1, $2)", s.tableName)
		_, err = s.db.Exec(migrationQuery, m.Migration, m.Version)
		if err != nil {
			e := tx.Rollback()
			if e != nil {
				return e
			}
			return err
		}
	}

	return tx.Commit()
}

func (s *MigrationsStore) ApplyMigrationsDown(migrations []*entity.MigrationEntity) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		_, err := s.db.Exec(m.Query)
		if err != nil {
			e := tx.Rollback()
			if e != nil {
				return e
			}
			return err
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE migration='%s'", s.tableName, m.Migration)
		_, err = s.db.Exec(query)
		if err != nil {
			e := tx.Rollback()
			if e != nil {
				return e
			}
			return err
		}
	}

	return tx.Commit()
}
