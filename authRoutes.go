package main

import (
	"database/sql"
	//"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Route setup
func AuthRoutes(r *gin.Engine, db *sql.DB) {
	r.POST("/api/v1/login", func(c *gin.Context) {
		handleLoginWithRole(c, db)
	})
	r.POST("/api/v1/register/user", func(c *gin.Context) {
		handleUserRegister(c, db)
	})
	r.POST("/api/v1/register/employee", func(c *gin.Context) {
		handleEmployeeRegister(c, db)
	})
}

// =================== LOGIN ===================

type RoleLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func handleLoginWithRole(c *gin.Context, db *sql.DB) {
	var input RoleLoginInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Email == "" || input.Password == "" || input.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Email, password, dan role wajib diisi"})
		return
	}

	email := strings.ToLower(input.Email)
	password := input.Password
	role := strings.ToLower(input.Role)
	validRoles := map[string]bool{"user": true, "admin": true, "employee": true}
	if !validRoles[role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Role tidak valid"})
		return
	}

	switch role {
	case "user":
		if user, found := findUserByEmail(db, email); found {
			if checkPassword(c, password, user.Password) {
				respondWithToken(c, user.ID, user.Email, "user", user.Username)
				return
			}
			return
		} else {
			//c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Email tidak ditemukan sebagai user"})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Email tidak ditemukan"})
		}
	case "admin":
		if admin, found := findAdminByEmail(db, email); found {
			if checkPassword(c, password, admin.Password) {
				respondWithToken(c, admin.ID, admin.Email, "admin", admin.Name)
				return
			}
			return
		} else {
			//c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Email tidak ditemukan sebagai admin"})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Email tidak ditemukan"})
		}
	case "employee":
		if emp, found := findEmployeeByEmail(db, email); found {
			if checkPassword(c, password, emp.Password) {
				respondWithToken(c, emp.ID, emp.Email, emp.Role, emp.Name)
				return
			}
			return
		} else {
			//c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Email tidak ditemukan sebagai employee"})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Email tidak ditemukan"})
		}
	}
}

// =================== REGISTER USER ===================

type UserRegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleUserRegister(c *gin.Context, db *sql.DB) {
	var input UserRegisterInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Email == "" || input.Password == "" || input.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Username, email, dan password wajib diisi"})
		return
	}

	// Untuk user
	// periksa apakah email sudah terdaftar
	// jika sudah terdaftar, kembalikan status 409 Conflict
	if _, found := findUserByEmail(db, strings.ToLower(input.Email)); found {
		c.JSON(http.StatusConflict, gin.H{"error": "❌ Email sudah terdaftar"})
		return
	}
	//periksa format email
	// jika tidak valid, kembalikan status 400 Bad Request
	if !strings.Contains(input.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format email tidak valid"})
		return
	}
	// periksa panjang password
	// jika kurang dari 6 karakter, kembalikan status 400 Bad Request
	if len(input.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Password minimal 6 karakter"})
		return
	}
	// periksa panjang username
	// jika kurang dari 3 karakter, kembalikan status 400 Bad Request
	if len(input.Username) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Username minimal 3 karakter"})
		return
	}
	// periksa apakah username sudah terdaftar
	// jika sudah terdaftar, kembalikan status 409 Conflict
	if _, found := findUserByUsername(db, input.Username); found {
		c.JSON(http.StatusConflict, gin.H{"error": "❌ Username sudah terdaftar"})
		return
	}

	//hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	hashedPwd, error := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengenkripsi password"})
		return
	}

	res, err := db.Exec("INSERT INTO users (username, email, password, heart, created_at) VALUES (?, ?, ?, ?, ?)",
		input.Username, strings.ToLower(input.Email), string(hashedPwd), 3, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mendaftarkan user"})
		return
	}
	id, _ := res.LastInsertId()
	// Langsung login (generate token)
	respondWithToken(c, int(id), strings.ToLower(input.Email), "user", input.Username)
}

// =================== REGISTER EMPLOYEE ===================

type EmployeeRegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // "employee"
}

func handleEmployeeRegister(c *gin.Context, db *sql.DB) {
	var input EmployeeRegisterInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Email == "" || input.Password == "" || input.Name == "" || input.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Name, email, password, dan role wajib diisi"})
		return
	}

	validRoles := map[string]bool{
		"cashier": true,
		"stocker": true,
		"manager": true,
	}

	if !validRoles[strings.ToLower(input.Role)] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Role harus salah satu dari 'cashier', 'stocker', atau 'manager'"})
		return
	}

	// Untuk employee
	// periksa apakah email sudah terdaftar
	// jika sudah terdaftar, kembalikan status 409 Conflict
	if _, found := findEmployeeByEmail(db, strings.ToLower(input.Email)); found {
		c.JSON(http.StatusConflict, gin.H{"error": "❌ Email sudah terdaftar"})
		return
	}
	//periksa format email
	// jika tidak valid, kembalikan status 400 Bad Request
	if !strings.Contains(input.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format email tidak valid"})
		return
	}
	// periksa panjang password
	// jika kurang dari 6 karakter, kembalikan status 400 Bad Request
	if len(input.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Password minimal 6 karakter"})
		return
	}

	//hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	hashedPwd, error := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengenkripsi password"})
		return
	}

	res, err := db.Exec("INSERT INTO employees (name, email, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
		input.Name, strings.ToLower(input.Email), string(hashedPwd), input.Role, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mendaftarkan employee"})
		return
	}
	id, _ := res.LastInsertId()
	// Langsung login (generate token)
	respondWithToken(c, int(id), strings.ToLower(input.Email), "employee", input.Name)
	//c.JSON(http.StatusCreated, gin.H{"message": "✅ Registrasi employee berhasil"})
}

// =================== DATABASE HELPER ===================

func findUserByUsername(db *sql.DB, username string) (User, bool) {
	var u User
	err := db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&u.ID)
	return u, err == nil
}

func findUserByEmail(db *sql.DB, email string) (User, bool) {
	var u User
	err := db.QueryRow("SELECT id, username, email, password, heart, created_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Username, &u.Email, &u.Password, &u.Heart, &u.CreatedAt)
	return u, err == nil
}

func findAdminByEmail(db *sql.DB, email string) (Admin, bool) {
	var a Admin
	err := db.QueryRow("SELECT id, name, email, password, created_at FROM admins WHERE email = ?", email).
		Scan(&a.ID, &a.Name, &a.Email, &a.Password, &a.CreatedAt)
	return a, err == nil
}

func findEmployeeByEmail(db *sql.DB, email string) (Employee, bool) {
	var e Employee
	err := db.QueryRow("SELECT id, name, email, password, role, created_at FROM employees WHERE email = ?", email).
		Scan(&e.ID, &e.Name, &e.Email, &e.Password, &e.Role, &e.CreatedAt)
	return e, err == nil
}

// =================== UTILITY ===================

func checkPassword(c *gin.Context, plainPwd, hashedPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Password salah"})
		return false
	}
	return true
}

func respondWithToken(c *gin.Context, id int, email, role, name string) {
	token, err := GenerateToken(id, email, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membuat token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Registrasi atau Login berhasil",
		"token":   token,
		"role":    role,
		"user": gin.H{
			"id":    id,
			"email": email,
			"name":  name,
		},
	})
}
