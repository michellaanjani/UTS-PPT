package main

import "time"

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"password"` // hashed password
	Heart     int       `json:"heart"`
	CreatedAt time.Time `json:"created_at"`
}

type Admin struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"` // hashed password
	CreatedAt time.Time `json:"created_at"`
}

type Employee struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"` // hashed password
	Role      string    `json:"role"`     // cashier, stocker, manager
	CreatedAt time.Time `json:"created_at"`
}
