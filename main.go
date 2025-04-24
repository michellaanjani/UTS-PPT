// --- main.go ---
package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Koneksi ke database
	db, err := InitDB()
	if err != nil {
		log.Fatalf("❌ Gagal terhubung ke database: %v", err)
		return
	}

	r := gin.Default()

	// // Setup Routes langsung dari fungsi yang sudah dibuat
	AuthRoutes(r, db)
	ProductRoutes(r, db)
	CategoryRoutes(r, db)
	ProductImageRoutes(r, db)
	RestockRequestRoutes(r, db)
	NotificationRoutes(r, db)
	ProductVariantRoutes(r, db)
	CartRoutes(r, db)
	StockReservationRoutes(r, db)
	CartItemRoutes(r, db)
	OrderRoutes(r, db)

	// // Menjalankan server
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ Gagal menjalankan server: %v", err)
	}
	log.Println("✅ Server running at http://localhost:8080")
}
