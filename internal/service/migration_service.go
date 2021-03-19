package service

import (
	"fmt"
	"github.com/malyg1n/sql-migrator/internal/entity"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type storeContract interface {
	GetDbDriver() string
	CreateMigrationsTable(query string) error
	GetMigrations() ([]*entity.MigrationEntity, error)
	GetMigrationsByVersion(version uint) ([]*entity.MigrationEntity, error)
	GetLatestVersionNumber() (uint, error)
	ApplyMigrationsUp(migrations []*entity.MigrationEntity) error
	ApplyMigrationsDown(migrations []*entity.MigrationEntity) error
}

type Service struct {
	repo           storeContract
	migrationsPath string
}

const (
	prepareScriptsPath = "../../prepare"
	timeFormat         = "20060102150405"
)

func NewService(repo storeContract, migrationsPath string) *Service {
	return &Service{
		repo:           repo,
		migrationsPath: migrationsPath,
	}
}

func (s *Service) Prepare() error {
	err := s.CreateFolder()
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(prepareScriptsPath + s.repo.GetDbDriver() + ".sql")
	if err != nil {
		return err
	}

	return s.repo.CreateMigrationsTable(string(data))
}

func (s *Service) CreateFolder() error {
	return os.Mkdir(s.migrationsPath, 0764)
}

func (s *Service) CreateMigrationFile(migrationName string) ([]string, error) {
	var messages []string
	upFileName := fmt.Sprintf("%s-%s-up.sql", time.Now().Format(timeFormat), strings.TrimSpace(migrationName))
	pathName := path.Join(s.migrationsPath, upFileName)
	fUp, err := os.Create(pathName)

	if err != nil {
		return nil, err
	}

	messages = append(messages, fmt.Sprintf("created migration %s", pathName))

	downFileName := fmt.Sprintf("%s-%s-down.sql", time.Now().Format(timeFormat), strings.TrimSpace(migrationName))
	pathName = path.Join(s.migrationsPath, downFileName)
	fDown, err := os.Create(pathName)

	if err != nil {
		return nil, err
	}

	messages = append(messages, fmt.Sprintf("created migration %s", pathName))

	defer func() {
		_ = fUp.Close()
		_ = fDown.Close()
	}()

	return messages, nil
}

func (s *Service) ApplyMigrationsUp() ([]string, error) {
	migrations, err := s.repo.GetMigrations()
	if err != nil {
		return nil, err
	}

	files, err := s.GetMigrationUpFiles(s.migrationsPath)
	if err != nil {
		return nil, err
	}

	newMigrationsFiles := s.FilterMigrations(migrations, files)
	if len(newMigrationsFiles) == 0 {
		return nil, nil
	}

	version, err := s.repo.GetLatestVersionNumber()
	if err != nil {
		return nil, err
	}

	// increase version number
	version++

	var migrated []string
	var newMigrations []*entity.MigrationEntity

	for _, file := range newMigrationsFiles {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}
		migrated = append(migrated, file)
		newMigrations = append(newMigrations, entity.NewMigrationEntity(file, string(data), version))
	}
	err = s.repo.ApplyMigrationsUp(newMigrations)
	if err != nil {
		return nil, err
	}

	return migrated, nil
}

func (s *Service) ApplyMigrationsDown() ([]string, error) {
	version, err := s.repo.GetLatestVersionNumber()
	if err != nil {
		return nil, err
	}

	migrations, err := s.repo.GetMigrationsByVersion(version)
	if err != nil {
		return nil, err
	}

	var rollback []string
	var backMigrations []*entity.MigrationEntity

	for _, m := range migrations {
		filePath := strings.Replace(m.Migration, "-up.sql", "-down.sql", 1)
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		rollback = append(rollback, filePath)
		backMigrations = append(backMigrations, entity.NewMigrationEntity(filePath, string(data), m.Version))
	}

	err = s.repo.ApplyMigrationsDown(backMigrations)
	if err != nil {
		return nil, err
	}

	return rollback, err
}

func (s *Service) ApplyAllMigrationsDown() ([]string, error) {
	migrations, err := s.repo.GetMigrations()
	if err != nil {
		return nil, err
	}

	var rollback []string

	var backMigrations []*entity.MigrationEntity

	for _, m := range migrations {
		filePath := strings.Replace(m.Migration, "-up.sql", "-down.sql", 1)
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		rollback = append(rollback, filePath)
		backMigrations = append(backMigrations, entity.NewMigrationEntity(filePath, string(data), m.Version))
	}

	if err := s.repo.ApplyMigrationsDown(backMigrations); err != nil {
		return nil, err
	}

	return rollback, err
}

func (s *Service) RefreshMigrations() ([]string, error) {
	var messages []string
	rolledBack, err := s.ApplyAllMigrationsDown()
	if err != nil {
		return nil, err
	}

	migrated, err := s.ApplyMigrationsUp()
	if err != nil {
		return nil, err
	}

	for _, rb := range rolledBack {
		messages = append(messages, fmt.Sprintf("rolled back: %s", rb))
	}

	for _, m := range migrated {
		messages = append(messages, fmt.Sprintf("migrated: %s", m))
	}

	return messages, err
}

func (s *Service) GetMigrationUpFiles(folder string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "-up.sql") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *Service) FilterMigrations(dbMigrations []*entity.MigrationEntity, files []string) []string {
	newFiles := make([]string, 0)
	for _, file := range files {
		found := false
		for _, m := range dbMigrations {
			if m.Migration == file {
				found = true
				break
			}
		}
		if found == false {
			newFiles = append(newFiles, file)
		}
	}
	return newFiles
}
