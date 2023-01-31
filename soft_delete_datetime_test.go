package soft_delete

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Book struct {
	ID        uint
	Name      string
	Pages     uint
	DeletedAt DeletedDateTime `gorm:"default:'1970-01-01 00:00:01'"`
}

func TestSoftDeleteDateTime(t *testing.T) {
	DB, err := gorm.Open(sqlite.Open(filepath.Join(os.TempDir(), "gorm.db")), &gorm.Config{})
	DB = DB.Debug()
	if err != nil {
		t.Errorf("failed to connect database")
	}

	book := Book{Name: "jinzhu", Pages: 10}
	DB.Migrator().DropTable(&Book{})
	DB.AutoMigrate(&Book{})
	DB.Save(&book)

	var count int64
	if DB.Model(&Book{}).Where("name = ?", book.Name).Count(&count).Error != nil || count != 1 {
		t.Errorf("Count soft deleted record, expects: %v, got: %v", 1, count)
	}

	var pages uint
	if DB.Model(&Book{}).Select("pages").Where("name = ?", book.Name).Scan(&pages).Error != nil || pages != book.Pages {
		t.Errorf("Pages soft deleted record, expects: %v, got: %v", 0, pages)
	}

	if err := DB.Delete(&book).Error; err != nil {
		t.Fatalf("No error should happen when soft delete user, but got %v", err)
	}

	if book.DeletedAt.Time.Equal(DateTimeZero) {
		t.Errorf("book's deleted at should not be zero, DeletedAt: %v", book.DeletedAt)
	}

	sql := DB.Session(&gorm.Session{DryRun: true}).Delete(&book).Statement.SQL.String()
	if !regexp.MustCompile(`UPDATE .books. SET .deleted_at.=.* WHERE .books.\..id. = .* AND .books.\..deleted_at. = ?`).MatchString(sql) {
		t.Fatalf("invalid sql generated, got %v", sql)
	}

	if DB.First(&Book{}, "name = ?", book.Name).Error == nil {
		t.Errorf("Can't find a soft deleted record")
	}

	count = 0
	if DB.Model(&Book{}).Where("name = ?", book.Name).Count(&count).Error != nil || count != 0 {
		t.Errorf("Count soft deleted record, expects: %v, got: %v", 0, count)
	}

	pages = 0
	if err := DB.Model(&Book{}).Select("pages").Where("name = ?", book.Name).Scan(&pages).Error; err != nil || pages != 0 {
		t.Fatalf("Age soft deleted record, expects: %v, got: %v, err %v", 0, pages, err)
	}

	if err := DB.Unscoped().First(&Book{}, "name = ?", book.Name).Error; err != nil {
		t.Errorf("Should find soft deleted record with Unscoped, but got err %s", err)
	}

	count = 0
	if DB.Unscoped().Model(&Book{}).Where("name = ?", book.Name).Count(&count).Error != nil || count != 1 {
		t.Errorf("Count soft deleted record, expects: %v, count: %v", 1, count)
	}

	pages = 0
	if DB.Unscoped().Model(&Book{}).Select("pages").Where("name = ?", book.Name).Scan(&pages).Error != nil || pages != book.Pages {
		t.Errorf("Age soft deleted record, expects: %v, got: %v", 0, pages)
	}

	DB.Unscoped().Delete(&book)
	if err := DB.Unscoped().First(&Book{}, "name = ?", book.Name).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Can't find permanently deleted record")
	}

}
