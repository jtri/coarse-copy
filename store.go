package main

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const postgresDSN = "host=localhost user=postgres password=password dbname=devdb port=5432"

func mustGetDBSession() *gorm.DB {
	db, err := gorm.Open(postgres.Open(postgresDSN), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}

type DBQuestion struct {
	gorm.Model
	ID   string
	Text string
}

func (DBQuestion) TableName() string {
	return "questions"
}

type Base struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func migrate(db *gorm.DB) error {
	type Questions struct {
		Base
		Text string
	}
	return db.AutoMigrate(
		&Questions{},
	)
}

type store struct {
	db *gorm.DB
}

func (s *store) getQuestion(id string) (*Question, error) {
	q := DBQuestion{}
	if err := s.db.Model(&DBQuestion{ID: id}).First(&q).Error; err != nil {
		return nil, err
	}
	return &Question{
		ID:   q.ID,
		Text: q.Text,
	}, nil
}

func (s *store) getQuestions() ([]*Question, error) {
	qs := []DBQuestion{}
	if err := s.db.Model(&DBQuestion{}).Find(&qs).Error; err != nil {
		return nil, err
	}

	questions := []*Question{}
	for _, q := range qs {
		questions = append(questions, &Question{ID: q.ID, Text: q.Text})
	}
	return questions, nil
}

func (s *store) createQuestion(id, text string) error {
	q := DBQuestion{ID: id, Text: text}
	return s.db.Model(&DBQuestion{}).Create(&q).Error
}

func (s *store) updateQuestion(id, text string) error {
	return s.db.Model(&DBQuestion{ID: id}).Updates(&DBQuestion{Text: text}).Error
}

func (s *store) deleteQuestion(id string) error {
	return s.db.Model(&DBQuestion{ID: id}).Unscoped().Delete(&DBQuestion{ID: id}).Error
}

func (s *store) upsertQuestion(questions []*Question) error {
	dbQuestions := []*Question{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i, q := range questions {
			dbQuestions[i] = &Question{
				ID:   q.ID,
				Text: q.Text,
			}
			now := time.Now()
			upsertErr := tx.Model(&dbQuestions[i]).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"text":       dbQuestions[i].Text,
					"updated_at": now,
					"created_at": now,
				})}).
				Create(&dbQuestions[i]).Error
			if upsertErr != nil {
				return upsertErr
			}
		}
		return nil
	})
	return err
}
