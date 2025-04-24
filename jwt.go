package main

import (
	//"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

// Inisialisasi secret key dari .env
var jwtSecret []byte

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ File .env tidak ditemukan, lanjut pakai environment bawaan")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("❌ JWT_SECRET tidak ditemukan di environment")
	}
	jwtSecret = []byte(secret)
}

// Claims sesuai payload token
type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // user, admin, employee
	jwt.RegisteredClaims
}

// Fungsi untuk generate JWT token
func GenerateToken(userID int, email string, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Expired dalam 24 jam
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Middleware untuk validasi token dan set data user ke context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Token tidak ditemukan"})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			log.Printf("Token error: %v\n", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Token tidak valid atau expired"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Gagal parsing token"})
			c.Abort()
			return
		}

		// if err := claims.RegisteredClaims.Valid(); err != nil {
		// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "❌ Token kadaluarsa atau tidak valid"})
		// 	c.Abort()
		// 	return
		// }

		// Simpan ke context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// Middleware untuk cek role (admin, employee, user)
func RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "❌ Role tidak ditemukan"})
			c.Abort()
			return
		}

		for _, allowed := range allowedRoles {
			if role == allowed {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "❌ Akses ditolak (role tidak sesuai)"})
		c.Abort()
	}
}

// Helper untuk mengambil data dari context
func GetUserID(c *gin.Context) int {
	return c.GetInt("user_id")
}

func GetEmail(c *gin.Context) string {
	return c.GetString("email")
}

func GetRole(c *gin.Context) string {
	return c.GetString("role")
}
