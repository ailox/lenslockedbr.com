package models

import (
	"errors"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var (
	// ErrNotFound is returned when a resource cannot be found
	// in database.
	ErrNotFound = errors.New("models: resource not found")

	// ErrInvalidID is returned when an invalid ID is
	// provided to a method like Delete.
	ErrInvalidID = errors.New("models: ID provided was invalid")
)

type User struct {
	gorm.Model
	Name string
	Email string `gorm:"not null.unique_index"`
}

type UserService struct {
	db *gorm.DB
}

func NewUserService(connectionInfo string) (*UserService, error) {
	db, err := gorm.Open("postgres", connectionInfo)
	if err != nil {
		return nil, err
	}

	db.LogMode(true)

	return &UserService { db: db, }, nil
}

func (u *UserService) Close() error {
	return u.db.Close()
}

// DestructiveReset drops the user table and rebuilds it
func (u *UserService)DestructiveReset() {
	u.db.DropTableIfExists(&User{})
	u.db.AutoMigrate(&User{})
}

// Create will create the provided user and backfill data like
// the ID, CreatedAt, and UpdatedAt fields.
func (u *UserService) Create(user *User) error {
	return u.db.Create(user).Error	
}

// ByID will look up a user with the provided ID.
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with more
// information about what went wrong.
// This may not be an error generated by the models package.
//
// As a general rule, any error but ErrNotFound should probably 
// result in a 500 error.
func (u *UserService) ByID(id uint) (*User, error) {
	var user User

	db := u.db.Where("id = ?", id)
	err := first(db, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ByEmail looks up a user with the given email address and returns 
// that user.
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with more 
// information about what went wrong. This may not be an error generated
// by the models package.
func (u *UserService) ByEmail(email string) (*User, error) {
	var user User
	db := u.db.Where("email = ?", email)
	err := first(db, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil

}

// Update will update the provided user with all of the data in
// the provided user object.
func (u *UserService) Update(user *User) error {
	return u.db.Save(user).Error
}

// Delete will delete the user with the provided ID
func (u *UserService) Delete(id uint) error {
	if id == 0 {
		return ErrInvalidID
	}

	user := User{Model: gorm.Model{ID: id}}
	return u.db.Delete(&user).Error
}

//
// Helper Functions
//

//
// first will query using the provided gorm.DB and it will get
// the first item returned and place it into dst. If nothing is
// found in the query, it will return ErrNotFound
//
func first(db *gorm.DB, dst interface{}) error {
	err := db.First(dst).Error
	if err == gorm.ErrRecordNotFound {
		return ErrNotFound
	}
	return err
}

