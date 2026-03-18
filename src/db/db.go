package db

import (
	"gorm.io/gorm/logger"
	"math"
	"os"
	"paperlink/db/entity"
	"paperlink/util"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sqliteTableColumn struct {
	Name string `gorm:"column:name"`
}

var (
	once     sync.Once
	instance *gorm.DB
)
var log = util.GroupLog("DATABASE")

func DB() *gorm.DB {
	once.Do(func() {
		err := os.MkdirAll("./data/log", 0755)
		if err != nil {
			logrus.Fatalf("Failed to create log directory: %v", err)
		}
		doesDBExist := true
		if _, err = os.Stat("./data/app.db"); os.IsNotExist(err) {
			doesDBExist = false
		}
		instance, err = gorm.Open(sqlite.Open("./data/app.db"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			log.Fatalf("Error connecting to the database: %v", err)
		}
		err = ApplySQLiteConfig(instance)
		if err != nil {
			log.Fatalf("Error connecting to the database: %v", err)
		}
		err = instance.AutoMigrate(
			&entity.Annotation{}, &entity.AnnotationAction{}, &entity.FileDocument{},
			&entity.Document{}, &entity.DocumentUser{}, &entity.Notification{},
			&entity.Tag{}, &entity.User{}, &entity.Directory{},
			&entity.RegistrationInvite{}, &entity.Digi4SchoolAccount{}, &entity.Digi4SchoolBook{}, &entity.Task{},
		)
		if err != nil {
			log.Fatalf("Error migrating database: %v", err)
		}
		err = ensureSQLiteColumns(instance)
		if err != nil {
			log.Fatalf("Error migrating database columns: %v", err)
		}
		log.Info("Database connection established.")
		if !doesDBExist {
			instance.Save(&entity.RegistrationInvite{
				Code:      "admin",
				ExpiresAt: math.MaxInt64,
				Uses:      1,
			})
			log.Info("Created admin token. This token is valid until it is taken")
		}
	})

	return instance
}

func ensureSQLiteColumns(instance *gorm.DB) error {
	requiredColumns := map[string]map[string]string{
		"annotations": {
			"page": "ALTER TABLE annotations ADD COLUMN page INTEGER DEFAULT 1",
		},
		"annotation_actions": {
			"action": "ALTER TABLE annotation_actions ADD COLUMN action TEXT DEFAULT 'UPDATE'",
		},
	}

	for tableName, tableColumns := range requiredColumns {
		existingColumns, err := getSQLiteColumns(instance, tableName)
		if err != nil {
			return err
		}

		for columnName, migration := range tableColumns {
			if _, ok := existingColumns[columnName]; ok {
				continue
			}
			if err := instance.Exec(migration).Error; err != nil {
				return err
			}
		}
	}

	if err := rebuildAnnotationActionsTableWithoutForeignKey(instance); err != nil {
		return err
	}

	return nil
}

func getSQLiteColumns(instance *gorm.DB, tableName string) (map[string]struct{}, error) {
	var columns []sqliteTableColumn
	if err := instance.Raw("PRAGMA table_info(" + tableName + ")").Scan(&columns).Error; err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		result[column.Name] = struct{}{}
	}

	return result, nil
}

func rebuildAnnotationActionsTableWithoutForeignKey(instance *gorm.DB) error {
	type foreignKeyRow struct {
		ID int `gorm:"column:id"`
	}

	var foreignKeys []foreignKeyRow
	if err := instance.Raw("PRAGMA foreign_key_list(annotation_actions)").Scan(&foreignKeys).Error; err != nil {
		return err
	}
	if len(foreignKeys) == 0 {
		return nil
	}

	return instance.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE annotation_actions RENAME TO annotation_actions_old").Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			CREATE TABLE annotation_actions (
				id integer PRIMARY KEY AUTOINCREMENT,
				action text,
				data text,
				created_at integer,
				annotation_id integer
			)
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO annotation_actions (id, action, data, created_at, annotation_id)
			SELECT id, action, data, created_at, annotation_id
			FROM annotation_actions_old
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec("DROP TABLE annotation_actions_old").Error; err != nil {
			return err
		}
		if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return err
		}
		return nil
	})
}
func ApplySQLiteConfig(instance *gorm.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = -10240;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA wal_autocheckpoint = 1000;",
	}

	for _, p := range pragmas {
		if err := instance.Exec(p).Error; err != nil {
			return err
		}
	}
	return nil
}
