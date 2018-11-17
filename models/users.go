package models

import (
	"errors"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"golang.org/x/crypto/bcrypt"

	"lenslockedbr.com/rand"
	"lenslockedbr.com/hash"
)

const hmacSecretKey = "secret-hmac-key"

var (
	// ErrNotFound is returned when a resource cannot be found
	// in database.
	ErrNotFound = errors.New("models: resource not found")

	// ErrInvalidID is returned when an invalid ID is
	// provided to a method like Delete.
	ErrInvalidID = errors.New("models: ID provided was invalid")

	// ErrInvalidPassword is returned when an invalid password
	// is used when attempting to authenticate a user.
	ErrInvalidPassword = errors.New("models: incorrect password provided")

	// Default user pepper for password
	userPwPepper = "foobar"

	_ UserDB = &userGorm{}
	_ UserService = &userService{}
)

type User struct {
	gorm.Model
	Name         string
	Age          int
	Email        string `gorm:"not null;unique_index"`
	Password     string `gorm:"-"`
	PasswordHash string `gorm:"not null"`
	Remember     string `gorm:"-"`
	RememberHash string `gorm:"not nill;unique_index"`
}

// UserDB is used to interact with the users database.
//
// For pretty much all single user queries:
//
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with more
// information about what went wrong. This may not be an error
// generated by the models package.
//
// For single user queries, any error but ErrNotFound should probably
// result in a 500 error until we make "public" facing errors.
type UserDB interface {

	// Methods for querying for single users
	ByID(id uint) (*User, error)
	ByEmail(email string) (*User, error)
	ByRemember(token string) (*User, error)
	ByAge(age int) (*User, error) 

	// Methods for querying multiples users
	InAgeRange(min, max int) ([]User, error)

	// Methods for altering users
	Create(user *User) error
	Update(user *User) error
	Delete(id uint) error

	// Used to close a DB connection
	Close() error

	// Migration helpers
	AutoMigrate() error
	DestructiveReset() error	
}

// UserService interface is a set of methods used to manipulate and
// userGorm represents our database interaction layer and implements
// the UserDB interface fully.
type userGorm struct {
	db *gorm.DB
}

// work with the user model
type UserService interface {

	UserDB

	// Authenticate will verify the provided email address
	// and password are correct. If they are correct, the
	// user corresponding to that email will be returned.
	// Otherwise you will receive either:
	// ErrNotFound, ErrInvalidPassword, or another error if
	// something goes wrong.
	Authenticate(email, password string) (*User, error)
}

type userService struct {
	UserDB
}

// userValidator is our validation layer that validates and normalizes
// data before passing it on to the next UserDB in our interface chain.
type userValidator struct {
	UserDB
	hmac hash.HMAC
}

type userValFn func(*User) error

func runUserValFns(user *User, fns ...userValFn) error {
	for _, fn := range fns {
		if err := fn(user); err != nil {
			return err
		}
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
//
// METHODS
//
/////////////////////////////////////////////////////////////////////

// THIS NO LONGER RETURNS A POINTER! Interfaces can be nil, so we don't
// need to return a pointer here. Don't forget to update this first 
// line - we removed the * character at the end where we write
// (UserService, error)
func NewUserService(connectionInfo string) (UserService, error) {

	u, err := newUserGorm(connectionInfo)
	if err != nil {
		return nil, err
	}

	// this old line was in newUserGorm
	hmac := hash.NewHMAC(hmacSecretKey)
	uv := &userValidator {
		UserDB: u,
		hmac: hmac,
	}

	// We also need to update how we construct the user service.
	// We no longer have a UserService type to construct, and 
	// instead need to use the userService type.
	// This IS still a pointer, as our functions implementing the
	// UserService are done with pointer receivers. eg:
	//   func (us *userService) <- this uses a pointer
	return &userService{
		UserDB: uv,
	}, nil
}

func newUserGorm(connectionInfo string) (*userGorm, error) {
	db, err := gorm.Open("postgres", connectionInfo)
	if err != nil {
		return nil, err
	}

	db.LogMode(true)

	return &userGorm{
		db: db,
	}, nil
}

func (u *userGorm) Close() error {
	return u.db.Close()
}

// DestructiveReset drops the user table and rebuilds it
func (u *userGorm) DestructiveReset() error {
	err := u.db.DropTableIfExists(&User{}).Error
	if err != nil {
		return err
	}
	return u.AutoMigrate()
}

// AutoMigrate will attempt to automatically migrate the users table
func (u *userGorm) AutoMigrate() error {
	if err := u.db.AutoMigrate(&User{}).Error; err != nil {
		return err
	}
	return nil
}

// Create will create the provided user and backfill data like ID,
// CreatedAt, and UpdatedAt fields.
func (u *userValidator) Create(user *User) error {

	if err := runUserValFns(user, u.bcryptPassword,
				      u.setRememberIfUnset,
                                      u.hmacRemember); err != nil {
		return err
	}

	return u.UserDB.Create(user)
}

// Create will create the provided user and backfill data like
// the ID, CreatedAt, and UpdatedAt fields.
func (u *userGorm) Create(user *User) error {
	return u.db.Create(user).Error
}

// Update will hash a remember token if it is provided
func (u *userValidator) Update(user *User) error {

	if err := runUserValFns(user, u.bcryptPassword,
                                      u.hmacRemember); err != nil {
		return err
	}

	return u.UserDB.Update(user)
}

// Update will update the provided user with all of the data in
// the provided user object.
func (u *userGorm) Update(user *User) error {
	return u.db.Save(user).Error
}

// Delete will delete the user with the provided ID
func (u *userValidator) Delete(id uint) error {
	if id == 0 {
		return ErrInvalidID
	}

	return u.UserDB.Delete(id)
}

// Delete will delete the user with the provided ID
func (u *userGorm) Delete(id uint) error {
	user := User{Model: gorm.Model{ID: id}}
	return u.db.Delete(&user).Error
}

// Authenticate can be used to authenticate a user with the provided
// email address and password.
// If the email address provided is invalid, this will return
// nil, ErroNotFound
// If the password provided is invalid, this will return 
// nil. ErrInvalidPassword
// If the email and password are both valid, this will return
// user, nil
// Otherwise if another error is encountered this will return nil, error
func (u *userService) Authenticate(email, password string) (*User, error) {
	foundUser, err := u.ByEmail(email)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword(
			[]byte(foundUser.PasswordHash),
			[]byte(password+userPwPepper))

	switch err {
	case nil:
		return foundUser, nil
	case bcrypt.ErrMismatchedHashAndPassword:
		return nil, ErrInvalidPassword
	default:
		return nil, err
	}
}

// bcryptPassword will hash a user's password with an app-wide pepper
// and bcrypt, which salts for us.
func (u *userValidator) bcryptPassword(user *User) error {

	if user.Password == "" {
		// We DO NOT need to run this if the password
		// hasn't been changed.
		return nil
	}
	
	pwBytes := []byte(user.Password + userPwPepper)
	hashedBytes, err := bcrypt.GenerateFromPassword(pwBytes, 
						bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedBytes)
	user.Password = ""

	return nil
}

func (u *userValidator) hmacRemember(user *User) error {
	if user.Remember == "" {
		return nil
	}
	user.RememberHash = u.hmac.Hash(user.Remember)

	return nil
}

func (u *userValidator) setRememberIfUnset(user *User) error {
	if user.Remember != "" {
		return nil
	}

	token, err := rand.RememberToken()
	if err != nil {
		return err
	}

	user.Remember = token

	return nil
}

/////////////////////////////////////////////////////////////////////
//
// Query Methods
//
/////////////////////////////////////////////////////////////////////

// ByID will look up a user with the provided ID.
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with more
// information about what went wrong.
// This may not be an error generated by the models package.
//
// As a general rule, any error but ErrNotFound should probably
// result in a 500 error.
func (u *userGorm) ByID(id uint) (*User, error) {
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
func (u *userGorm) ByEmail(email string) (*User, error) {
	var user User
	db := u.db.Where("email = ?", email)
	err := first(db, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil

}

// ByAge will look up a user with the provided age.
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with more
// information about what went wrong.
// This may not be an error generated by the models package.
//
// As a general rule, any error but ErrNotFound should probably
// result in a 500 error.
func (u *userGorm) ByAge(age int) (*User, error) {
	var user User
	db := u.db.Where("age = ?", age)
	err := first(db, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// AgeInRange will find all the users where its age are between
// a specific range of ages
func (u *userGorm) InAgeRange(min, max int) ([]User, error) {

	users := make([]User, 0)

	db := u.db.Where("age BETWEEN ? AND ?", min, max)
	err := all(db, &users)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// ByRemember looks up a user with the given remember token and returns
// that user. This method expects the remember token already hashed
func (u *userGorm) ByRemember(rememberHashed string) (*User, error) {
	var user User
	err := first(u.db.Where("remember_hash = ?", rememberHashed), 
                     &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ByRemember will hash the remember token and then call ByRemember on
// the subsequent UserDB layer.
func (u *userValidator) ByRemember(token string) (*User, error) {

	user := User {
		Remember: token,
	}

	if err := runUserValFns(&user, u.hmacRemember); err != nil {
		return nil, err
	}

	return u.UserDB.ByRemember(user.RememberHash)
}

/////////////////////////////////////////////////////////////////////
//
// Helper Functions
//
/////////////////////////////////////////////////////////////////////

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

func all(db *gorm.DB, dst interface{}) error {
	err := db.Find(dst).Error
	if err == gorm.ErrRecordNotFound {
		return ErrNotFound
	}
	return err
}
