package main

import (
	"fmt"

	"lenslockedbr.com/models"
)

const (
	host     = "192.168.56.101"
	port     = 5432
	user     = "developer"
	password = "1234qwer"
	dbname   = "lenslockedbr_dev"
)


func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s " +  
                                "dbname=%s sslmode=disable",
		                 host, port, user, password, dbname)

	us, err := models.NewUserService(psqlInfo)
	if err != nil {
		panic(err)
	}

	defer us.Close()

	us.DestructiveReset()

	fmt.Println("Successfully connected!")

	var user models.User

	user = models.User{ Name: "Foobar", 
                     Email: "foobar@example.com",
	}
	err = us.Create(&user)
	if err != nil {
		panic(err)
	}

	fmt.Println("User created:", user)

	user = models.User{ Name: "Test", 
                     Email: "test@example.com",
	}
	err = us.Create(&user)
	if err != nil {
		panic(err)
	}

	fmt.Println("User created:", user)

	byId, err := us.ByID(2)
	if err != nil {
		panic(err)
	}

	fmt.Println("User find by ID:", byId)

	byEmail, err := us.ByEmail("foobar@example.com")
	if err != nil {
		panic(err)
	}

	fmt.Println("User find by Email:", byEmail)

	byId.Name = "Updated"
	err = us.Update(byId)
	if err != nil {
		panic(err)
	}

	fmt.Println("User updated:", byId)

	err = us.Delete(2)
	if err != nil {
		panic(err)
	}

	fetchById, err := us.ByID(2)
	if err != nil {
		panic(err)
	}
	fmt.Println("User find deleted by ID:", fetchById)
}
