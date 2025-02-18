package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Tag struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"uniqueIndex;size:50;not null"`
}

type Category struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"uniqueIndex;size:50;not null"`
}

type BlogPost struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Title        string    `gorm:"size:150;not null" json:"title"`
	Content      string    `gorm:"size:2000;not null" json:"content"`
	CategoryName string    `gorm:"size:50;set null" json:"category"`
	Tags         []Tag     `gorm:"many2many:blog_post_tags;"`
	CreatedAt    time.Time `gorm:"type:datetime;default:CURRENT_TIMESTAMP();not null" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"type:datetime;default:CURRENT_TIMESTAMP() on update CURRENT_TIMESTAMP();not null" json:"updatedAt"`
	Category     Category  `gorm:"foreignKey:CategoryName;references:Name;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type BlogPostTag struct {
	BlogPostID uint64   `gorm:"primaryKey"`
	TagID      uint64   `gorm:"primaryKey"`
	BlogPost   BlogPost `gorm:"foreignKey:BlogPostID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Tag        Tag      `gorm:"foreignKey:TagID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

var DB *gorm.DB

func ConnectDB() {
	dsn := buildDSN()
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	err = DB.AutoMigrate(&Tag{}, &Category{}, &BlogPost{}, &BlogPostTag{})
	if err != nil {
		panic(err)
	}
	log.Println("successfully connected to database")
}

func buildDSN() string {
	user := os.Getenv("mysql_user")
	pass := os.Getenv("mysql_pass")
	addr := os.Getenv("mysql_addr")
	database := os.Getenv("mysql_db")
	dsn := "%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&parseTime=true"

	return fmt.Sprintf(dsn, user, pass, addr, database)
}
