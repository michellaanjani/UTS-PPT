// Semuanya masih dalam package main
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// =======================
// 🧩 Helper Functions
// =======================
// GetIDParam is a helper function to get the ID parameter from the URL and convert it to an integer.
func GetIDParam(c *gin.Context) (int, string, bool) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ ID harus berupa angka"})
		return 0, "", false
	}
	return id, idStr, true
}

// ValidateRecordExistence is a helper function to check if a record exists in the database.
func ValidateRecordExistence(c *gin.Context, db *sql.DB, table string, id int) bool {
	valid, err := IsValidID(db, table, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("❌ Gagal memeriksa ID di tabel %s", table)})
		return false
	}
	if !valid {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("❌ Data %s tidak ditemukan", table)})
		return false
	}
	return true
}

// IsValidID is a helper function to check if a given ID exists in the specified table.
func IsValidID(db *sql.DB, tableName string, id int) (bool, error) {
	// List of allowed table names to prevent SQL injection
	allowedTables := map[string]bool{
		"categories":       true,
		"products":         true,
		"product_variants": true,
		"product_images":   true,
		"restock_requests": true,
		"users":            true,
		"employees":        true,
		"carts":            true,
		"notifications":    true,
		// Add more allowed tables here
	}

	// Check if the table name is allowed
	if !allowedTables[tableName] {
		return false, fmt.Errorf("invalid table name: %s", tableName)
	}

	// Build the query string safely using fmt.Sprintf after validation
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = ?)", tableName)

	var exists bool
	err := db.QueryRow(query, id).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// CheckIfVarians is a helper function to check if the product is a variant or not.
func CheckIfVarians(db *sql.DB, productID int) (bool, error) {
	var isVarians bool
	query := "SELECT is_varians FROM products WHERE id = ?"
	err := db.QueryRow(query, productID).Scan(&isVarians)
	if err != nil {
		return false, fmt.Errorf("❌ Gagal memeriksa varian produk: %v", err)
	}
	return isVarians, nil
}

// SetNullableFieldsForVariant is a helper function to set nullable fields to NULL if the product is a variant.
// SetNullableFieldsForVariant mengosongkan field-field jika is_varians = true
func SetNullableFieldsForVariant(isVarians bool, product *ProductsModel) {
	if isVarians {
		product.IsDiscounted = nil
		product.DiscountPrice = nil
		product.Price = nil
		product.Stock = nil
		product.IsService = false
	}
}

// SetRequiredFieldsForNonVariant memastikan field wajib jika is_varians = false
func SetRequiredFieldsForNonVariant(product *ProductsModel) error {
	if product.Price == nil || *product.Price < 0 {
		return fmt.Errorf("❌ Harga wajib diisi dan tidak boleh negatif")
	}
	if product.Stock == nil || *product.Stock < 0 {
		return fmt.Errorf("❌ Stok wajib diisi dan tidak boleh negatif")
	}
	if product.IsDiscounted != nil && *product.IsDiscounted {
		if product.DiscountPrice == nil {
			return fmt.Errorf("❌ Harga diskon wajib diisi jika produk sedang diskon")
		}
		if *product.DiscountPrice >= *product.Price {
			return fmt.Errorf("❌ Harga diskon harus lebih kecil dari harga normal")
		}
	}
	return nil
}

// helper function untuk products
func ValidateProductInput(product *ProductsModel, c *gin.Context, db *sql.DB) error {
	if strings.TrimSpace(product.Name) == "" {
		return fmt.Errorf("❌ Nama produk tidak boleh kosong")
	}
	if strings.TrimSpace(product.Description) == "" {
		return fmt.Errorf("❌ Deskripsi produk tidak boleh kosong")
	}
	// check if categiory_id is valid
	if !ValidateRecordExistence(c, db, "categories", product.CategoryID) {
		return fmt.Errorf("❌ Kategori tidak ditemukan")
	}
	if product.CategoryID == 0 {
		return fmt.Errorf("❌ ID kategori tidak boleh 0")
	}

	if product.IsVarians {
		SetNullableFieldsForVariant(true, product)
	} else {
		if err := SetRequiredFieldsForNonVariant(product); err != nil {
			return err
		}
	}
	return nil
}

// =========================
// 🛠️ Cart TotalPrice Helpers
// =========================

// Tambahkan nilai ke total_price cart
func AddToCartTotalPrice(db *sql.DB, cartID int, amount int) error {
	_, err := db.Exec(`
		UPDATE carts 
		SET total_price = total_price + ?, updated_at = NOW() 
		WHERE id = ?
	`, amount, cartID)
	return err
}

// Kurangi nilai dari total_price cart
func SubtractFromCartTotalPrice(db *sql.DB, cartID int, amount int) error {
	_, err := db.Exec(`
		UPDATE carts 
		SET total_price = GREATEST(0, total_price - ?), updated_at = NOW() 
		WHERE id = ?
	`, amount, cartID)
	return err
}

// =========================
// 🧩 Helper Functions END
// =========================

// =========================
// 🗂️ Category Management
// =========================
func CategoryRoutes(r *gin.Engine, db *sql.DB) {
	api := r.Group("/api/v1/categories")
	api.GET("", func(c *gin.Context) {
		GetAllCategories(c, db)
	})

	// 🔐 Khusus admin
	api.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		api.POST("", func(c *gin.Context) {
			CreateCategory(c, db)
		})
		api.PATCH("/:id", func(c *gin.Context) {
			UpdateCategory(c, db)
		})
		api.DELETE("/:id", func(c *gin.Context) {
			DeleteCategory(c, db)
		})
	}
}

// ++++++++++++++++++++++++
//
//	Categories READ
//
// +++++++++++++++++++++++++
func GetAllCategories(c *gin.Context, db *sql.DB) {
	rows, err := db.Query(`
		SELECT id, name FROM categories
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil data kategori"})
		return
	}
	defer rows.Close()

	var categories []CategoryModel
	for rows.Next() {
		var cat CategoryModel
		err := rows.Scan(&cat.ID, &cat.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membaca data kategori"})
			return
		}
		categories = append(categories, cat)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Semua kategori berhasil diambil",
		"data":    categories,
	})
}

// ++++++++++++++++++++++++
//
//	Categories CREATE
//
// +++++++++++++++++++++++++
func CreateCategory(c *gin.Context, db *sql.DB) {
	var input CategoryModel

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format JSON tidak valid"})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Nama kategori wajib diisi"})
		return
	}

	result, err := db.Exec(`INSERT INTO categories (name) VALUES (?)`, input.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menyimpan kategori"})
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, gin.H{
		"message": "✅ Kategori berhasil ditambahkan",
		"id":      id,
	})
}

// ++++++++++++++++++++++++
//  Categories UPDATE
// +++++++++++++++++++++++++

func UpdateCategory(c *gin.Context, db *sql.DB) {
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	if !ValidateRecordExistence(c, db, "categories", idInt) {
		return
	}

	var input CategoryModel

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Data tidak valid"})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Nama kategori wajib diisi"})
		return
	}

	result, err := db.Exec(`UPDATE categories SET name = ? WHERE id = ?`, input.Name, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengupdate kategori"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "⚠️ Kategori tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Kategori berhasil diupdate",
		"name":    input.Name,
		"id":      id,
	})
}

// ++++++++++++++++++++++++
//
//	Categories DELETE
//
// +++++++++++++++++++++++++
func DeleteCategory(c *gin.Context, db *sql.DB) {
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}

	// //cek apakah id valid
	if !ValidateRecordExistence(c, db, "categories", idInt) {
		return
	}

	_, err := db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus kategori"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Kategori berhasil dihapus",
	})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// =========================
// 📦 Product Management
// =========================
func ProductRoutes(r *gin.Engine, db *sql.DB) {
	api := r.Group("/api/v1/products")

	// 🟢 Public untuk melihat produk
	api.GET("", func(c *gin.Context) {
		GetAllProducts(c, db) // Fungsi untuk mengambil semua produk, implementasi terpisah
	})

	// 🔐 Khusus admin
	api.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		api.POST("", func(c *gin.Context) {
			CreateProduct(c, db)
		})
		api.PATCH("/:id", func(c *gin.Context) {
			UpdateProduct(c, db)
		})
		api.DELETE("/:id", func(c *gin.Context) {
			DeleteProduct(c, db)
		})
	}
}

// ++++++++++++++++++++++++
//
//	Product READ
//
// +++++++++++++++++++++++++
func GetAllProducts(c *gin.Context, db *sql.DB) {
	rows, err := db.Query(`
		SELECT 
			id, category_id, name, description, is_varians, is_discounted, discount_price, price, stock, 
			is_service, created_at, updated_at, search_vector
		FROM products
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil data produk"})
		return
	}
	defer rows.Close()

	var products []ProductsModel

	for rows.Next() {
		var p ProductsModel
		err := rows.Scan(
			&p.ID,
			&p.CategoryID,
			&p.Name,
			&p.Description,
			&p.IsVarians,
			&p.IsDiscounted,
			&p.DiscountPrice,
			&p.Price,
			&p.Stock,
			&p.IsService,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.SearchVector,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("❌ Scan error: %v", err)})
			return
		}

		products = append(products, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Semua produk berhasil diambil",
		"data":    products,
	})
}

// ++++++++++++++++++++++++
//
//	Product CREATE
//
// ++++++++++++++++++++++++
func CreateProduct(c *gin.Context, db *sql.DB) {
	var product ProductsModel
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Data produk tidak valid"})
		return
	}

	// Validasi logika produk berdasarkan is_varians
	if err := ValidateProductInput(&product, c, db); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Masukkan ke database
	query := `
		INSERT INTO products 
		(category_id, name, description, is_varians, is_discounted, discount_price, price, stock, is_service, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	res, err := db.Exec(query,
		product.CategoryID,
		product.Name,
		product.Description,
		product.IsVarians,
		product.IsDiscounted,
		product.DiscountPrice,
		product.Price,
		product.Stock,
		product.IsService,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menyimpan produk ke database"})
		return
	}

	// Ambil ID yang baru dibuat
	lastID, _ := res.LastInsertId()
	product.ID = int(lastID)

	// Kirim response lengkap
	c.JSON(http.StatusCreated, gin.H{
		"message": "✅ Produk berhasil dibuat",
		"data": gin.H{
			"id":             product.ID,
			"category_id":    product.CategoryID,
			"name":           product.Name,
			"description":    product.Description,
			"is_varians":     product.IsVarians,
			"is_discounted":  product.IsDiscounted,
			"discount_price": product.DiscountPrice,
			"price":          product.Price,
			"stock":          product.Stock,
			"is_service":     product.IsService,
		},
	})
}

// ++++++++++++++++++++++++
//
//	Product UPDATE
//
// ++++++++++++++++++++++++
func UpdateProduct(c *gin.Context, db *sql.DB) {
	// Ambil ID dari parameter
	idInt, _, ok := GetIDParam(c)
	if !ok {
		return
	}

	// Ambil produk dari database
	var product struct {
		CategoryID    int
		IsVarians     bool
		IsDiscounted  sql.NullBool
		DiscountPrice sql.NullInt64
		Price         sql.NullInt64
		Stock         sql.NullInt64
		IsService     sql.NullBool
	}
	err := db.QueryRow(`SELECT category_id, is_varians, is_discounted, discount_price, price, stock, is_service FROM products WHERE id = ?`, idInt).
		Scan(&product.CategoryID, &product.IsVarians, &product.IsDiscounted, &product.DiscountPrice, &product.Price, &product.Stock, &product.IsService)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "❌ Produk tidak ditemukan"})
		return
	}

	// Bind input dari request
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Data tidak valid"})
		return
	}
	if len(input) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Tidak ada data untuk diupdate"})
		return
	}

	// Validasi category_id jika ada
	if categoryIDRaw, ok := input["category_id"]; ok {
		categoryID, ok := categoryIDRaw.(float64)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ category_id harus berupa angka"})
			return
		}
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM categories WHERE id = ?", int(categoryID)).Scan(&count)
		if err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ category_id tidak valid"})
			return
		}
	}

	// Ambil is_varians dari input jika ada, jika tidak gunakan dari DB
	isVarians := product.IsVarians
	if raw, ok := input["is_varians"]; ok {
		val, ok := raw.(bool)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ is_varians harus berupa boolean"})
			return
		}
		isVarians = val
	}

	if isVarians {
		// Kosongkan field yang tidak relevan
		input["price"] = nil
		input["stock"] = nil
		input["is_service"] = false
		input["discount_price"] = nil
		input["is_discounted"] = nil
	} else {
		// Validasi is_discounted
		isDiscounted := false
		if val, ok := input["is_discounted"].(bool); ok {
			isDiscounted = val
		} else if product.IsDiscounted.Valid {
			isDiscounted = product.IsDiscounted.Bool
		}

		// Jika produk diskon, validasi harga
		if isDiscounted {
			var price float64
			if val, ok := input["price"].(float64); ok {
				price = val
			} else if product.Price.Valid {
				price = float64(product.Price.Int64)
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "❌ price diperlukan untuk produk diskon"})
				return
			}

			var discountPrice float64
			if val, ok := input["discount_price"].(float64); ok {
				discountPrice = val
			} else if product.DiscountPrice.Valid {
				discountPrice = float64(product.DiscountPrice.Int64)
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "❌ discount_price diperlukan untuk produk diskon"})
				return
			}

			if discountPrice >= price {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("⚠️ discount_price harus lebih kecil dari price: %.2f >= %.2f", discountPrice, price)})
				return
			}
		}

		// Validasi stock
		if stockVal, ok := input["stock"]; ok {
			stock, ok := stockVal.(float64)
			if !ok || stock < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "❌ stock harus angka >= 0"})
				return
			}
		} else if !product.Stock.Valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ stock harus tersedia untuk produk tanpa variasi"})
			return
		}

		// Validasi is_service
		if _, ok := input["is_service"]; !ok && !product.IsService.Valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ is_service harus tersedia untuk produk tanpa variasi"})
			return
		}
	}

	// Build query update secara dinamis
	var fields []string
	var args []interface{}
	for key, val := range input {
		fields = append(fields, fmt.Sprintf("%s = ?", key))
		args = append(args, val)
	}
	fields = append(fields, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, idInt)

	// Eksekusi query
	query := fmt.Sprintf("UPDATE products SET %s WHERE id = ?", strings.Join(fields, ", "))
	_, err = db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengupdate produk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "✅ Produk berhasil diupdate"})
}

// ++++++++++++++++++++++++
//
//	Product DELETE
//
// ++++++++++++++++++++++++
func DeleteProduct(c *gin.Context, db *sql.DB) {
	//id string to int
	idInt, _, ok := GetIDParam(c)
	if !ok {
		return
	}
	//cek apakah id valid
	if !ValidateRecordExistence(c, db, "products", idInt) {
		return
	}

	// Eksekusi delete
	_, err := db.Exec("DELETE FROM products WHERE id = ?", idInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus produk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "✅ Produk berhasil dihapus"})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// =========================
// 📦 Product Variant Management
// =========================
func ProductVariantRoutes(r *gin.Engine, db *sql.DB) {
	api := r.Group("/api/v1/productvariants")

	// 🟢 Public untuk melihat semua varian produk
	api.GET("", func(c *gin.Context) {
		GetAllProductVariants(c, db) // Fungsi untuk mengambil semua varian produk
	})

	// 🔐 Khusus admin
	api.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		// Menambahkan varian produk baru
		api.POST("", func(c *gin.Context) {
			CreateProductVariant(c, db)
		})

		// Memperbarui varian produk berdasarkan ID
		api.PATCH("/:id", func(c *gin.Context) {
			UpdateProductVariant(c, db)
		})

		// Menghapus varian produk berdasarkan ID
		api.DELETE("/:id", func(c *gin.Context) {
			DeleteProductVariant(c, db)
		})
	}
}

// ++++++++++++++++++++++++
// Product Variant READ
// +++++++++++++++++++++++++
func GetAllProductVariants(c *gin.Context, db *sql.DB) {
	rows, err := db.Query(`
		SELECT 
			id, product_id, name, color, price, is_discounted, discount_price, stock, 
			is_service, created_at, updated_at, search_vector
		FROM product_variants
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil data varian produk"})
		return
	}
	defer rows.Close()

	var productVariants []ProductVariantModel

	for rows.Next() {
		var pv ProductVariantModel
		err := rows.Scan(
			&pv.ID,
			&pv.ProductID,
			&pv.Name,
			&pv.Color,
			&pv.Price,
			&pv.IsDiscounted,
			&pv.DiscountPrice,
			&pv.Stock,
			&pv.IsService,
			&pv.CreatedAt,
			&pv.UpdatedAt,
			&pv.SearchVector,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("❌ Scan error: %v", err)})
			return
		}

		productVariants = append(productVariants, pv)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Semua varian produk berhasil diambil",
		"data":    productVariants,
	})
}

// ++++++++++++++++++++++++
//
//	Product Variant CREATE
//
// ++++++++++++++++++++++++

// ++++++++++++++++++++++++
//
//	Product Variant CREATE Helper
//
// ValidateProductVariantInput adalah fungsi untuk memvalidasi input varian produk.
func ValidateProductVariantInput(productVariant *ProductVariantModel) error {
	// Validasi nama varian produk
	if strings.TrimSpace(productVariant.Name) == "" {
		return fmt.Errorf("❌ Nama varian produk tidak boleh kosong")
	}

	// Validasi ID produk, ID produk harus ada dan lebih dari 0
	if productVariant.ProductID == 0 {
		return fmt.Errorf("❌ ID produk tidak boleh kosong")
	}

	// Validasi harga varian produk, harga tidak boleh negatif
	if productVariant.Price < 0 {
		return fmt.Errorf("❌ Harga varian produk tidak boleh negatif")
	}

	// Validasi stok varian produk, stok tidak boleh negatif
	if productVariant.Stock < 1 {
		return fmt.Errorf("❌ Stok varian produk harus diisi dan tidak boleh negatif")
	}

	// Validasi diskon jika ada, jika varian produk diskon, pastikan harga diskon lebih kecil dari harga normal
	if productVariant.IsDiscounted && (productVariant.DiscountPrice == nil || *productVariant.DiscountPrice >= productVariant.Price) {
		return fmt.Errorf("❌ Harga diskon harus lebih kecil dari harga normal")
	}

	// Validasi warna jika ada (nullable), jika ada, pastikan tidak kosong hanya jika diberikan
	if productVariant.Color != nil && strings.TrimSpace(*productVariant.Color) == "" {
		return fmt.Errorf("❌ Warna varian produk tidak boleh kosong jika diberikan")
	}

	// Validasi search_vector (nullable), bisa kosong atau diisi string
	if productVariant.SearchVector != nil && strings.TrimSpace(*productVariant.SearchVector) == "" {
		return fmt.Errorf("❌ Search vector tidak boleh kosong jika diberikan")
	}

	return nil
}

// ++++++++++++++++++++++++
func CreateProductVariant(c *gin.Context, db *sql.DB) {
	var productVariant ProductVariantModel

	// Bind input JSON ke struct
	if err := c.ShouldBindJSON(&productVariant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Data varian produk tidak valid, pastikan is_service diisi "})
		return
	}

	// Set is_discounted berdasarkan keberadaan discount_price
	if productVariant.DiscountPrice != nil {
		productVariant.IsDiscounted = true
	} else {
		productVariant.IsDiscounted = false
	}

	// Validasi input
	if err := ValidateProductVariantInput(&productVariant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Validasi apakah produk dengan ID tersebut punya is_varians = true
	isVarians, err := CheckIfVarians(db, int(productVariant.ProductID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal memeriksa varian produk"})
		return
	}
	if !isVarians {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Produk tidak memiliki varian"})
		return
	}

	// Default IsService jika tidak dikirim (Go default bool = false, jadi aman)

	// Set waktu
	productVariant.CreatedAt = time.Now()
	productVariant.UpdatedAt = time.Now()

	// SQL untuk insert
	query := `
		INSERT INTO product_variants 
		(product_id, name, color, price, is_discounted, discount_price, stock, is_service, search_vector, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := db.Exec(
		query,
		productVariant.ProductID,
		productVariant.Name,
		productVariant.Color,
		productVariant.Price,
		productVariant.IsDiscounted,
		productVariant.DiscountPrice,
		productVariant.Stock,
		productVariant.IsService,
		productVariant.SearchVector,
		productVariant.CreatedAt,
		productVariant.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menyimpan varian produk ke database"})
		return
	}

	lastID, _ := res.LastInsertId()
	productVariant.ID = int(lastID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "✅ Varian produk berhasil dibuat",
		"data": gin.H{
			"id":             productVariant.ID,
			"product_id":     productVariant.ProductID,
			"name":           productVariant.Name,
			"color":          productVariant.Color,
			"price":          productVariant.Price,
			"is_discounted":  productVariant.IsDiscounted,
			"discount_price": productVariant.DiscountPrice,
			"stock":          productVariant.Stock,
			"is_service":     productVariant.IsService,
			"search_vector":  productVariant.SearchVector,
			"created_at":     productVariant.CreatedAt,
			"updated_at":     productVariant.UpdatedAt,
		},
	})
}

// ++++++++++++++++++++++++
// Product Variant UPDATE
// +++++++++++++++++++++++++
func UpdateProductVariant(c *gin.Context, db *sql.DB) {
	// Ambil ID dari parameter
	idInt, idStr, ok := GetIDParam(c)
	if !ok {
		return
	}

	// Validasi apakah varian produk dengan ID ada
	if !ValidateRecordExistence(c, db, "product_variants", idInt) {
		return
	}

	// Bind JSON ke map
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Data tidak valid"})
		return
	}

	if len(input) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Tidak ada data untuk diupdate"})
		return
	}

	// Handle logic diskon
	if dpRaw, exists := input["discount_price"]; exists {
		if dpRaw == nil || dpRaw == "" {
			input["discount_price"] = nil
			input["is_discounted"] = false
		} else {
			discountPrice, ok := dpRaw.(float64)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "❌ discount_price harus berupa angka"})
				return
			}

			var currentPrice float64
			if priceRaw, ok := input["price"]; ok {
				price, ok := priceRaw.(float64)
				if !ok {
					c.JSON(http.StatusBadRequest, gin.H{"error": "❌ price harus berupa angka"})
					return
				}
				currentPrice = price
			} else {
				err := db.QueryRow("SELECT price FROM product_variants WHERE id = ?", idStr).Scan(&currentPrice)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil harga varian"})
					return
				}
			}

			if discountPrice >= currentPrice {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("⚠️ discount_price harus lebih murah dari price: %.2f >= %.2f", discountPrice, currentPrice),
				})
				return
			}

			input["is_discounted"] = true
		}
	}

	// Validasi jika ingin ubah product_id
	if pidRaw, exists := input["product_id"]; exists {
		pidFloat, ok := pidRaw.(float64)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ product_id harus berupa angka"})
			return
		}
		if !ValidateRecordExistence(c, db, "products", int(pidFloat)) {
			return
		}
		// Validasi apakah produk dengan ID tersebut punya is_varians = true
		isVarians, err := CheckIfVarians(db, int(input["product_id"].(float64)))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal memeriksa varian produk"})
			return
		}
		if !isVarians {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Produk tidak memiliki varian"})
			return
		}
	}

	// Update waktu
	input["updated_at"] = time.Now()

	// Siapkan query dinamis
	var fields []string
	var values []interface{}

	for key, value := range input {
		fields = append(fields, fmt.Sprintf("%s = ?", key))
		values = append(values, value)
	}

	values = append(values, idStr)

	query := fmt.Sprintf("UPDATE product_variants SET %s WHERE id = ?", strings.Join(fields, ", "))

	result, err := db.Exec(query, values...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengupdate varian produk"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "⚠️ Varian produk tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Varian produk berhasil diupdate",
		"updated": input,
	})
}

// ++++++++++++++++++++++++
// Product Variant DELETE
// +++++++++++++++++++++++++

func DeleteProductVariant(c *gin.Context, db *sql.DB) {
	// Ambil ID dari parameter
	idInt, _, ok := GetIDParam(c)
	if !ok {
		return
	}

	// Validasi apakah varian produk dengan ID tersebut ada
	if !ValidateRecordExistence(c, db, "product_variants", idInt) {
		return
	}

	// Eksekusi DELETE
	_, err := db.Exec("DELETE FROM product_variants WHERE id = ?", idInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus varian produk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "✅ Varian produk berhasil dihapus"})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// ===========================
// 🖼️ Product Image Management
// ===========================
func ProductImageRoutes(r *gin.Engine, db *sql.DB) {
	api := r.Group("/api/v1/product-images")

	// 🟢 Public untuk semua tanpa login
	api.GET("", func(c *gin.Context) {
		GetAllProductImages(c, db)
	})

	// 🔐 Khusus admin
	api.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		api.POST("", func(c *gin.Context) {
			CreateProductImage(c, db)
		})
		api.PATCH("/:id", func(c *gin.Context) {
			UpdateProductImage(c, db)
		})
		api.DELETE("/:id", func(c *gin.Context) {
			DeleteProductImage(c, db)
		})
	}
}

// ++++++++++++++++++++++++
//
//	Images READ
//
// ++++++++++++++++++++++++
func GetAllProductImages(c *gin.Context, db *sql.DB) {
	productID := c.Query("product_id") // bisa kosong (ambil semua)

	var rows *sql.Rows
	var err error

	if productID != "" {
		rows, err = db.Query(`SELECT id, product_id, image_url FROM product_images WHERE product_id = ?`, productID)
	} else {
		rows, err = db.Query(`SELECT id, product_id, image_url FROM product_images`)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil data gambar produk"})
		return
	}
	defer rows.Close()

	var images []ProductImageModel
	for rows.Next() {
		var img ProductImageModel
		if err := rows.Scan(&img.ID, &img.ProductID, &img.ImageURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membaca data gambar"})
			return
		}
		images = append(images, img)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Semua gambar berhasil diambil",
		"data":    images,
	})
}

// ++++++++++++++++++++++++
//  Images CREATE
// ++++++++++++++++++++++++

func CreateProductImage(c *gin.Context, db *sql.DB) {
	var input ProductImageModel

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format JSON tidak valid"})
		return
	}

	if input.ProductID == 0 || input.ImageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ 'product_id' dan 'image_url' wajib diisi"})
		return
	}

	// Cek apakah image_url valid
	if !strings.HasPrefix(input.ImageURL, "http://") && !strings.HasPrefix(input.ImageURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "⚠️ URL gambar tidak valid"})
		return
	}

	// Cek apakah product_id valid
	if !ValidateRecordExistence(c, db, "products", int(input.ProductID)) {
		return
	}

	result, err := db.Exec(`INSERT INTO product_images (product_id, image_url) VALUES (?, ?)`, input.ProductID, input.ImageURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menambahkan gambar produk"})
		return
	}
	resultID, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, gin.H{
		"message": "✅ Gambar produk berhasil ditambahkan",
		"id":      resultID,
	})
}

// ++++++++++++++++++++++++
//
//	Images UPDATE
//
// ++++++++++++++++++++++++
func UpdateProductImage(c *gin.Context, db *sql.DB) {
	//idStr := c.Param("id")
	//id string to int
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	//Cek apakah id valid
	if !ValidateRecordExistence(c, db, "product_images", idInt) {
		return
	}

	var input ProductImageModel
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format data tidak valid"})
		return
	}

	if input.ImageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ 'image_url' wajib diisi"})
		return
	}
	// Cek apakah image_url valid
	if !strings.HasPrefix(input.ImageURL, "http://") && !strings.HasPrefix(input.ImageURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "⚠️ URL gambar tidak valid"})
		return
	}

	// Siapkan query dan args
	var query string
	var args []interface{}

	if input.ProductID != 0 {
		// Jika ProductID baru diinput, validasi dulu
		if !ValidateRecordExistence(c, db, "products", int(input.ProductID)) {
			return
		}

		query = `UPDATE product_images SET product_id = ?, image_url = ? WHERE id = ?`
		args = []interface{}{input.ProductID, input.ImageURL, id}
	} else {
		query = `UPDATE product_images SET image_url = ? WHERE id = ?`
		args = []interface{}{input.ImageURL, id}
	}

	// Eksekusi update
	result, err := db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengupdate gambar produk"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "⚠️ Gambar tidak ditemukan"})
		return
	}

	response := gin.H{
		"message":   "✅ Gambar produk berhasil diupdate",
		"id":        id,
		"image_url": input.ImageURL,
	}
	if input.ProductID != 0 {
		response["product_id"] = input.ProductID
	}

	c.JSON(http.StatusOK, response)
}

// ++++++++++++++++++++++++
//
//	Images DELETE
//
// ++++++++++++++++++++++++
func DeleteProductImage(c *gin.Context, db *sql.DB) {
	//id string to int
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	//Cek apakah id valid
	if !ValidateRecordExistence(c, db, "product_images", idInt) {
		return
	}

	res, err := db.Exec(`DELETE FROM product_images WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus gambar produk"})
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "⚠️ Gambar tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Gambar produk berhasil dihapus",
	})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// =========================
// 🛒 Cart Management
// =========================
func CartRoutes(r *gin.Engine, db *sql.DB) {
	// 🔐 Khusus customer
	customerCart := r.Group("/api/v1/carts")
	customerCart.Use(AuthMiddleware(), RoleMiddleware("user"))
	{
		customerCart.POST("", func(c *gin.Context) {
			CreateCart(c, db)
		})
		customerCart.GET("/:id", func(c *gin.Context) {
			GetCartByID(c, db)
		})
		// customerCart.PUT("/addprice/:id", func(c *gin.Context) {
		// 	AddCartTotalPrice(c, db)
		// })
		// customerCart.PUT("/substractprice/:id", func(c *gin.Context) {
		// 	SubtractCartTotalPrice(c, db)
		// })
	}

	// 🔐 Khusus admin untuk hapus cart
	adminCart := r.Group("/api/v1/carts")
	adminCart.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		adminCart.DELETE("/:id", func(c *gin.Context) {
			DeleteCart(c, db)
		})
	}
}

// ++++++++++++++++++++++++
//
//	Cart READ
//
// ++++++++++++++++++++++++
func GetCartByID(c *gin.Context, db *sql.DB) {
	id, _, ok := GetIDParam(c)
	if !ok {
		return
	}

	if !ValidateRecordExistence(c, db, "carts", id) {
		return
	}

	var cart CartModel
	err := db.QueryRow("SELECT id, total_price, created_at, updated_at FROM carts WHERE id = ?", id).
		Scan(&cart.ID, &cart.TotalPrice, &cart.CreatedAt, &cart.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cart})
}

// ++++++++++++++++++++++++
//
//	Cart CREATE
//
// ++++++++++++++++++++++++
func CreateCart(c *gin.Context, db *sql.DB) {
	var cart CartModel

	if err := c.ShouldBindJSON(&cart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Data tidak valid, masukkan id user"})
		return
	}

	// Cek apakah user valid
	if !ValidateRecordExistence(c, db, "users", cart.ID) {
		return
	}

	cart.TotalPrice = 0 // Set total_price ke 0 saat membuat cart baru

	query := "INSERT INTO carts (id, total_price, created_at, updated_at) VALUES (?, ?, NOW(), NOW())"
	_, err := db.Exec(query, cart.ID, cart.TotalPrice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membuat cart"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "✅ Cart berhasil dibuat", "data": cart})
}

// ++++++++++++++++++++++++
//
//	Cart UPDATE
//
// ++++++++++++++++++++++++
// func EditCart(c *gin.Context) {
// 	c.JSON(http.StatusCreated, gin.H{"message": "✅ Produk ditambahkan ke keranjang (dummy)"})
// }

// ++++++++++++++++++++++++
//
//	Cart DELETE
//
// ++++++++++++++++++++++++
func DeleteCart(c *gin.Context, db *sql.DB) {
	id, _, ok := GetIDParam(c)
	if !ok {
		return
	}

	if !ValidateRecordExistence(c, db, "carts", id) {
		return
	}

	_, err := db.Exec("DELETE FROM carts WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "✅ Cart berhasil dihapus"})
}

// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
// =========================
// 🛒 Cart Item Management
// =========================
func CartItemRoutes(r *gin.Engine, db *sql.DB) {
	// 🔐 Khusus customer (user)
	customerCartItem := r.Group("/api/v1/cart-items")
	customerCartItem.Use(AuthMiddleware(), RoleMiddleware("user"))
	{
		// Create cart item
		customerCartItem.POST("", func(c *gin.Context) {
			CreateCartItem(c, db)
		})

		// Get cart item by my ID
		customerCartItem.GET("/my", func(c *gin.Context) {
			MyCartItems(c, db)
		})

		// Update quantity (jika dibutuhkan nanti)
		customerCartItem.PATCH("/:id", func(c *gin.Context) {
			UpdateCartItemQuantity(c, db)
		})

		// Delete cart item
		customerCartItem.DELETE("/:id", func(c *gin.Context) {
			DeleteCartItem(c, db)
		})
	}
}

// +++++++++++++++++++++++++++++++++
// Cart Item CREATE MY CART
// +++++++++++++++++++++++++++++++++
func CreateCartItem(c *gin.Context, db *sql.DB) {
	userID := GetUserID(c) // cart_id juga

	var input struct {
		ProductID        int  `json:"product_id"`
		ProductVariantID *int `json:"product_variant_id"` // opsional
		Quantity         int  `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Input tidak valid"})
		return
	}
	// Cek apakah input ada product_id dan quantity
	if input.ProductID == 0 || input.Quantity == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ product_id dan quantity harus diisi"})
		return
	}
	// Cek apakah product_id valid
	if !ValidateRecordExistence(c, db, "products", input.ProductID) {
		return
	}

	// Cek apakah cart ada
	if !ValidateRecordExistence(c, db, "carts", userID) {
		return
	}

	// Ambil data product: is_varians, price, stock
	var isVarians bool
	var is_discounted *bool
	var productPrice, productStock, discount_price *int
	err := db.QueryRow(`
		SELECT is_varians, price, stock, is_discounted, discount_price FROM products WHERE id = ?
	`, input.ProductID).Scan(&isVarians, &productPrice, &productStock, &is_discounted, &discount_price)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Produk tidak ditemukan"})
		return
	}

	var stockAvailable int
	var pricePerItem int

	// Kalau pakai variant
	if isVarians {
		if input.ProductVariantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Product variant harus diisi karena produk punya variasi"})
			return
		}
		var isDiscount bool
		var productVarPrice int
		var productVarDisprice *int
		// Ambil stok dari variant
		err := db.QueryRow(`
			SELECT stock, price, is_discounted, discount_price FROM product_variants WHERE id = ? AND product_id = ?
		`, int(*input.ProductVariantID), input.ProductID).Scan(&stockAvailable, &productVarPrice, &isDiscount, &productVarDisprice)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Variant tidak ditemukan untuk produk ini"})
			return
		}
		// Cek apakah variant ada diskon
		if isDiscount {
			pricePerItem = *productVarDisprice
		} else {
			pricePerItem = productVarPrice
		}
	} else {
		if input.ProductVariantID != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Produk ini tidak memiliki variasi, hapus product_variant_id"})
			return
		}
		stockAvailable = *productStock
		if *is_discounted {
			pricePerItem = *discount_price
		} else {
			pricePerItem = *productPrice
		}
	}

	if input.Quantity <= 0 || input.Quantity > stockAvailable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Quantity melebihi stok atau tidak valid"})
		return
	}

	totalPrice := input.Quantity * pricePerItem

	// Tambahkan ke total cart dulu
	if err := AddToCartTotalPrice(db, userID, totalPrice); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal update total harga cart"})
		return
	}

	// Insert cart item
	_, err = db.Exec(`
		INSERT INTO cart_items 
		(cart_id, product_id, product_variant_id, quantity, price_per_item, total_price, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, userID, input.ProductID, input.ProductVariantID, input.Quantity, pricePerItem, totalPrice)
	if err != nil {
		// Rollback kalau gagal insert
		_ = SubtractFromCartTotalPrice(db, userID, totalPrice)

		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menambahkan item ke cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "✅ Item berhasil ditambahkan ke cart"})
}

// +++++++++++++++++++++++++++++++++
// Cart Item READ MY CART
// +++++++++++++++++++++++++++++++++
func MyCartItems(c *gin.Context, db *sql.DB) {
	userID := GetUserID(c) // Ini cart_id juga

	// Cek apakah cart ada
	if !ValidateRecordExistence(c, db, "carts", userID) {
		return
	}

	rows, err := db.Query(`
		SELECT id, cart_id, product_id, product_variant_id, quantity, price_per_item, total_price, created_at, updated_at
		FROM cart_items
		WHERE cart_id = ?
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil data cart item"})
		return
	}
	defer rows.Close()

	var items []CartItemModel
	for rows.Next() {
		var item CartItemModel
		if err := rows.Scan(
			&item.ID,
			&item.CartID,
			&item.ProductID,
			&item.ProductVariantID,
			&item.Quantity,
			&item.PricePerItem,
			&item.TotalPrice,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal memproses data cart item"})
			return
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Kesalahan saat membaca data cart item"})
		return
	}

	if len(items) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "⚠️ Keranjang kosong",
			"data":    []CartItemModel{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Berhasil mengambil item dalam cart kamu",
		"data":    items,
	})
}

// +++++++++++++++++++++++++++++++++
// Cart Item UPDATE MY CART
// +++++++++++++++++++++++++++++++++
func UpdateCartItemQuantity(c *gin.Context, db *sql.DB) {
	userID := GetUserID(c)
	itemID := c.Param("id")

	var input struct {
		Quantity int `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Quantity tidak valid atau tidak diisi"})
		return
	}

	// Ambil data cart item dulu (harus milik user yang login)
	var cartID, productID, oldQuantity, pricePerItem int
	var productVariantID *int

	err := db.QueryRow(`
		SELECT cart_id, product_id, product_variant_id, quantity, price_per_item
		FROM cart_items WHERE id = ? AND cart_id = ?
	`, itemID, userID).Scan(&cartID, &productID, &productVariantID, &oldQuantity, &pricePerItem)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "❌ Item tidak ditemukan atau bukan milik kamu"})
		return
	}

	// Cek apakah produk menggunakan variant
	var isVarians bool
	if productVariantID == nil {
		isVarians = false
	} else {
		isVarians = true
	}

	var stockAvailable int
	if isVarians {
		if productVariantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Variant wajib karena produk punya variasi"})
			return
		}
		err = db.QueryRow(`SELECT stock FROM product_variants WHERE id = ?`, *productVariantID).Scan(&stockAvailable)
	} else {
		err = db.QueryRow(`SELECT stock FROM products WHERE id = ?`, productID).Scan(&stockAvailable)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Gagal ambil data stok"})
		return
	}

	if input.Quantity > stockAvailable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Quantity melebihi stok"})
		return
	}

	oldTotal := oldQuantity * pricePerItem
	newTotal := input.Quantity * pricePerItem
	diff := newTotal - oldTotal

	// Update total harga di cart
	if diff > 0 {
		if err := AddToCartTotalPrice(db, userID, diff); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal update total harga cart"})
			return
		}
	} else if diff < 0 {
		if err := SubtractFromCartTotalPrice(db, userID, -diff); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal update total harga cart"})
			return
		}
	}

	// Update cart item
	_, err = db.Exec(`
		UPDATE cart_items
		SET quantity = ?, total_price = ?, updated_at = NOW()
		WHERE id = ?
	`, input.Quantity, newTotal, itemID)

	if err != nil {
		// Balikin total price cart kalau gagal update item
		if diff > 0 {
			_ = SubtractFromCartTotalPrice(db, userID, diff)
		} else if diff < 0 {
			_ = AddToCartTotalPrice(db, userID, -diff)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal update item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "✅ Quantity berhasil diupdate"})
}

// +++++++++++++++++++++++++++++++++
// Cart Item DELETE MY CART
// +++++++++++++++++++++++++++++++++
func DeleteCartItem(c *gin.Context, db *sql.DB) {
	userID := GetUserID(c)
	itemID := c.Param("id")

	// Ambil total_price dari cart item & pastikan item milik user
	var cartID, itemTotal int
	err := db.QueryRow(`
		SELECT cart_id, total_price
		FROM cart_items
		WHERE id = ? AND cart_id = ?
	`, itemID, userID).Scan(&cartID, &itemTotal)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "❌ Item tidak ditemukan atau bukan milik kamu"})
		return
	}

	// Kurangi total harga di cart
	if err := SubtractFromCartTotalPrice(db, cartID, itemTotal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal update total harga cart"})
		return
	}

	// Hapus item
	_, err = db.Exec(`DELETE FROM cart_items WHERE id = ?`, itemID)
	if err != nil {
		// Balikin harga kalau gagal hapus item
		_ = AddToCartTotalPrice(db, cartID, itemTotal)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "🗑️ Item berhasil dihapus dari cart"})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

func CreateStockReservation(db *sql.DB, orderID, userID int, items []OrderItemModel) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	now := time.Now()
	var expiredAt time.Time
	var hearts int
	err = tx.QueryRow("SELECT hearts FROM users WHERE id = ?", userID).Scan(&hearts)
	if err != nil {
		return err
	}

	switch hearts {
	case 3:
		expiredAt = now.Add(24 * time.Hour)
	case 2:
		expiredAt = now.Add(12 * time.Hour)
	case 1:
		expiredAt = now.Add(6 * time.Hour)
	default:
		return errors.New("invalid heart count")
	}

	res, err := tx.Exec(`INSERT INTO temp_stock_reservations (user_id, order_id, reserved_at, expired_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, userID, orderID, now, expiredAt, now, now)
	if err != nil {
		return err
	}

	reservationID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for _, item := range items {
		// Kurangi stok produk/varian
		if item.ProductVariantID != nil {
			_, err = tx.Exec(`UPDATE product_variants SET stock = stock - ? WHERE id = ? AND stock >= ?`, item.Quantity, *item.ProductVariantID, item.Quantity)
			if err != nil {
				return err
			}
		} else {
			_, err = tx.Exec(`UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?`, item.Quantity, item.ProductID, item.Quantity)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`INSERT INTO temp_stock_details (temp_reservation_id, product_id, product_variant_id, quantity, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, reservationID, item.ProductID, item.ProductVariantID, item.Quantity, now, now)
		if err != nil {
			return err
		}
	}

	return nil
}

// Hapus reservasi dan kembalikan stok
func DeleteStockReservation(db *sql.DB, orderID int) error {
	rows, err := db.Query(`SELECT d.product_id, d.product_variant_id, d.quantity
		FROM temp_stock_reservations r
		JOIN temp_stock_details d ON r.id = d.temp_reservation_id
		WHERE r.order_id = ?`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []OrderItemModel
	for rows.Next() {
		var item OrderItemModel
		err := rows.Scan(&item.ProductID, &item.ProductVariantID, &item.Quantity)
		if err != nil {
			return err
		}
		items = append(items, item)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Kembalikan stok
	for _, item := range items {
		if item.ProductVariantID != nil {
			_, err = tx.Exec(`UPDATE product_variants SET stock = stock + ? WHERE id = ?`, item.Quantity, *item.ProductVariantID)
			if err != nil {
				return err
			}
		} else {
			_, err = tx.Exec(`UPDATE products SET stock = stock + ? WHERE id = ?`, item.Quantity, item.ProductID)
			if err != nil {
				return err
			}
		}
	}

	// Hapus detail dan reservasi
	_, err = tx.Exec(`DELETE FROM temp_stock_details WHERE temp_reservation_id IN (SELECT id FROM temp_stock_reservations WHERE order_id = ?)`, orderID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM temp_stock_reservations WHERE order_id = ?`, orderID)
	return err
}

// Untuk rollback stok ke inventory
func ReturnStockToInventory(db *sql.DB, items []OrderItemModel) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	for _, item := range items {
		if item.ProductVariantID != nil {
			_, err = tx.Exec("UPDATE product_variants SET stock = stock + ? WHERE id = ?", item.Quantity, *item.ProductVariantID)
		} else {
			_, err = tx.Exec("UPDATE products SET stock = stock + ? WHERE id = ?", item.Quantity, item.ProductID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// =========================
// 📦 Order Management
// =========================
func OrderRoutes(r *gin.Engine, db *sql.DB) {
	// 👤 USER: Customer routes (create order, lihat order sendiri, cancel order)
	customerOrder := r.Group("/api/v1/orders")
	customerOrder.Use(AuthMiddleware(), RoleMiddleware("user"))
	{
		// Buat order dari cart
		customerOrder.POST("", func(c *gin.Context) {
			CreateOrder(c, db)
		})

		// Lihat semua order milik user saat ini
		customerOrder.GET("/my", func(c *gin.Context) {
			GetMyOrders(c, db)
		})

		// Cancel order milik sendiri (ubah status jadi canceled, bukan hard delete)
		customerOrder.PUT("/:id/cancel", func(c *gin.Context) {
			CancelOrder(c, db)
		})
	}

	// 🛠️ CRONJOB / Scheduled Task: Cek dan tandai order yang expired
	r.GET("/api/v1/orders/check-expired", func(c *gin.Context) {
		CheckAndExpireOrders(c, db)
	})
}

// ++++++++++++++++++++++++
//
//	Order READ
//
// ++++++++++++++++++++++++
func GetMyOrders(c *gin.Context, db *sql.DB) {
	userID := c.GetInt("user_id")

	// Ambil semua order user
	orderRows, err := db.Query(`
		SELECT id, user_id, cart_user_id, status, total_price, timer_expiration, created_at, updated_at
		FROM orders
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data order"})
		return
	}
	defer orderRows.Close()

	// Struct gabungan order dan items
	type OrderWithItems struct {
		Order OrderModel       `json:"order"`
		Items []OrderItemModel `json:"items"`
	}

	var allOrders []OrderWithItems

	for orderRows.Next() {
		var order OrderModel
		err := orderRows.Scan(
			&order.ID, &order.UserID, &order.CartUserID, &order.Status,
			&order.TotalPrice, &order.TimerExpiration, &order.CreatedAt, &order.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca data order"})
			return
		}

		// Ambil order items
		itemRows, err := db.Query(`
			SELECT id, order_id, product_id, product_variant_id, quantity, price_at_purchase, total_price
			FROM order_items
			WHERE order_id = ?
		`, order.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil item order"})
			return
		}

		var items []OrderItemModel
		for itemRows.Next() {
			var item OrderItemModel
			err := itemRows.Scan(
				&item.ID, &item.OrderID, &item.ProductID, &item.ProductVariantID,
				&item.Quantity, &item.PriceAtPurchase, &item.TotalPrice,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca item order"})
				itemRows.Close()
				return
			}
			items = append(items, item)
		}
		itemRows.Close()

		allOrders = append(allOrders, OrderWithItems{
			Order: order,
			Items: items,
		})
	}

	if len(allOrders) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Belum ada order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": allOrders,
	})
}

// ++++++++++++++++++++++++
//
//	Order CREATE
//
// ++++++++++++++++++++++++
func CreateStockReservationTx(tx *sql.Tx, orderID, userID int, items []OrderItemModel) error {
	now := time.Now()

	// 1. Insert ke temp_stock_reservations
	res, err := tx.Exec(`
		INSERT INTO temp_stock_reservations (user_id, order_id, reserved_at, expired_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, orderID, now, now, now, now)
	if err != nil {
		return fmt.Errorf("gagal insert stock reservation: %v", err)
	}
	tempResID, _ := res.LastInsertId()

	// 2. Untuk setiap item, simpan detail dan kurangi stok
	for _, item := range items {
		// Simpan detail
		_, err := tx.Exec(`
			INSERT INTO temp_stock_details (temp_reservation_id, product_id, product_variant_id, quantity, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			tempResID, item.ProductID, item.ProductVariantID, item.Quantity, now, now)
		if err != nil {
			return fmt.Errorf("gagal insert detail stock: %v", err)
		}

		// Kurangi stok
		if item.ProductVariantID != nil {
			_, err = tx.Exec(`
				UPDATE product_variants SET stock = stock - ? WHERE id = ? AND stock >= ?`,
				item.Quantity, *item.ProductVariantID, item.Quantity)
		} else {
			_, err = tx.Exec(`
				UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?`,
				item.Quantity, item.ProductID, item.Quantity)
		}
		if err != nil {
			return fmt.Errorf("gagal mengurangi stok: %v", err)
		}
	}

	return nil
}

func CreateOrder(c *gin.Context, db *sql.DB) {
	userID := GetUserID(c)

	// Cek cart & item-nya
	var cart CartModel
	err := db.QueryRow(`SELECT id, total_price FROM carts WHERE id = ?`, userID).
		Scan(&cart.ID, &cart.TotalPrice)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Cart tidak ditemukan"})
		return
	}

	rows, err := db.Query(`
		SELECT id, product_id, product_variant_id, quantity, price_per_item, total_price
		FROM cart_items WHERE cart_id = ?
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil cart items"})
		return
	}
	defer rows.Close()

	var items []OrderItemModel
	for rows.Next() {
		var item OrderItemModel
		var variantID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ProductID, &variantID, &item.Quantity, &item.PriceAtPurchase, &item.TotalPrice); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membaca cart item"})
			return
		}
		if variantID.Valid {
			id := int(variantID.Int64)
			item.ProductVariantID = &id
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "⚠️ Cart kosong, tidak bisa membuat order"})
		return
	}

	// Hitung durasi timer berdasarkan hati user
	var heartCount int
	if err := db.QueryRow(`SELECT heart FROM users WHERE id = ?`, userID).Scan(&heartCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil jumlah hati"})
		return
	}
	var duration time.Duration
	switch heartCount {
	case 3:
		duration = 24 * time.Hour
	case 2:
		duration = 12 * time.Hour
	case 1:
		duration = 6 * time.Hour
	default:
		duration = 6 * time.Hour
	}
	now := time.Now()
	expiration := now.Add(duration)

	// Transaksi pembuatan order
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal memulai transaksi"})
		return
	}

	// Insert ke orders
	res, err := tx.Exec(`
		INSERT INTO orders (user_id, cart_user_id, status, total_price, timer_expiration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, cart.ID, "waitToBuy", cart.TotalPrice, expiration, now, now)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membuat order"})
		return
	}
	orderID, _ := res.LastInsertId()

	// Simpan order_items
	for _, item := range items {
		_, err := tx.Exec(`
			INSERT INTO order_items (order_id, product_id, product_variant_id, quantity, price_at_purchase, total_price)
			VALUES (?, ?, ?, ?, ?, ?)`,
			orderID, item.ProductID, item.ProductVariantID, item.Quantity, item.PriceAtPurchase, item.TotalPrice)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menyimpan order item"})
			return
		}
	}

	// Buat stock reservation
	if err := CreateStockReservationTx(tx, int(orderID), userID, items); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal melakukan reservasi stok"})
		return
	}

	// Hapus cart items dan reset total cart
	_, _ = tx.Exec(`DELETE FROM cart_items WHERE cart_id = ?`, userID)
	_, _ = tx.Exec(`UPDATE carts SET total_price = 0 WHERE id = ?`, userID)

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menyelesaikan order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "✅ Order berhasil dibuat",
		"order_id":   orderID,
		"expired_at": expiration,
	})
}

// ++++++++++++++++++++++++
//
//	Order UPDATE
//
// ++++++++++++++++++++++++
// start helper
func DeleteStockReservationTx(tx *sql.Tx, orderID int) error {
	// Hapus detail reservasi stok
	_, err := tx.Exec(`
		DELETE FROM temp_stock_details
		WHERE temp_reservation_id IN (SELECT id FROM temp_stock_reservations WHERE order_id = ?)
	`, orderID)
	if err != nil {
		return fmt.Errorf("gagal hapus temp_stock_details: %v", err)
	}

	// Hapus reservasi stok
	_, err = tx.Exec(`
		DELETE FROM temp_stock_reservations
		WHERE order_id = ?
	`, orderID)
	if err != nil {
		return fmt.Errorf("gagal hapus temp_stock_reservations: %v", err)
	}

	return nil
}
func ReturnStockToInventoryTx(tx *sql.Tx, items []OrderItemModel) error {
	for _, item := range items {
		// Update stok produk atau produk varian
		if item.ProductVariantID != nil {
			// Update stok untuk produk varian
			_, err := tx.Exec(`
				UPDATE product_variants
				SET stock = stock + ?
				WHERE id = ?
			`, item.Quantity, *item.ProductVariantID)
			if err != nil {
				return fmt.Errorf("gagal mengembalikan stok ke produk varian: %v", err)
			}
		} else {
			// Update stok untuk produk
			_, err := tx.Exec(`
				UPDATE products
				SET stock = stock + ?
				WHERE id = ?
			`, item.Quantity, item.ProductID)
			if err != nil {
				return fmt.Errorf("gagal mengembalikan stok ke produk: %v", err)
			}
		}
	}

	return nil
}

func GetOrderItems(db *sql.DB, orderID int) ([]OrderItemModel, error) {
	// Query untuk mengambil item berdasarkan order_id
	rows, err := db.Query(`
		SELECT oi.id, oi.order_id, oi.product_id, oi.product_variant_id, oi.quantity, oi.price_at_purchase, oi.total_price
		FROM order_items oi
		WHERE oi.order_id = ?
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data order items: %v", err)
	}
	defer rows.Close()

	var orderItems []OrderItemModel
	for rows.Next() {
		var item OrderItemModel
		// Scan hasil query ke dalam struct OrderItemModel
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.ProductVariantID, &item.Quantity, &item.PriceAtPurchase, &item.TotalPrice); err != nil {
			return nil, fmt.Errorf("gagal memindahkan data ke struct: %v", err)
		}
		// Tambahkan item ke dalam slice orderItems
		orderItems = append(orderItems, item)
	}

	// Pastikan tidak ada error setelah iterasi
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error dalam iterasi rows: %v", err)
	}

	return orderItems, nil
}

//end helper

func CancelOrder(c *gin.Context, db *sql.DB) {
	userID := c.GetInt("user_id")
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID tidak valid"})
		return
	}

	// Cek order-nya
	var order OrderModel
	err = db.QueryRow(`
		SELECT id, user_id, status FROM orders
		WHERE id = ?
	`, orderID).Scan(&order.ID, &order.UserID, &order.Status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order tidak ditemukan"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal cek order"})
		return
	}

	if order.UserID != userID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Tidak bisa membatalkan order orang lain"})
		return
	}

	if order.Status != "waitToBuy" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order tidak bisa dibatalkan"})
		return
	}

	// Ambil semua itemnya buat kembalikan stok
	items, err := GetOrderItems(db, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil item order"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}

	// Ubah status jadi canceled
	_, err = tx.Exec(`UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`, "canceled", time.Now(), orderID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengubah status order"})
		return
	}

	// Hapus reservasi stok dan kembalikan ke produk
	err = DeleteStockReservationTx(tx, orderID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus stock reservation"})
		return
	}

	err = ReturnStockToInventoryTx(tx, items)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengembalikan stok"})
		return
	}

	err = tx.Commit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pembatalan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order berhasil dibatalkan"})
}

// ++++++++++++++++++++++++
//
//	Order DELETE
//
// ++++++++++++++++++++++++
func DeleteStockReservationAndReturn(orderID int, db *sql.DB) {
	// Memulai transaksi
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return
	}

	// Pastikan kita melakukan rollback jika terjadi error
	defer tx.Rollback()

	// Ambil semua item yang terkait dengan order dari temp_stock_details
	rows, err := tx.Query(`
		SELECT tsd.product_id, tsd.product_variant_id, tsd.quantity
		FROM temp_stock_details tsd
		LEFT JOIN temp_stock_reservations tsr ON tsr.id = tsd.temp_reservation_id
		WHERE tsr.order_id = ?
	`, orderID)
	if err != nil {
		log.Printf("Failed to get stock details for order %d: %v", orderID, err)
		return
	}
	defer rows.Close()

	var stockDetails []struct {
		ProductID        int
		ProductVariantID *int
		Quantity         int
	}

	// Ambil detail produk dan varian untuk dikembalikan ke inventory
	for rows.Next() {
		var detail struct {
			ProductID        int
			ProductVariantID *int
			Quantity         int
		}
		if err := rows.Scan(&detail.ProductID, &detail.ProductVariantID, &detail.Quantity); err != nil {
			log.Printf("Failed to scan stock details for order %d: %v", orderID, err)
			return
		}
		stockDetails = append(stockDetails, detail)
	}

	// Hapus temp_stock_reservations dan temp_stock_details
	_, err = tx.Exec(`
		DELETE tsd FROM temp_stock_details tsd
		JOIN temp_stock_reservations tsr ON tsr.id = tsd.temp_reservation_id
		WHERE tsr.order_id = ?
	`, orderID)
	if err != nil {
		log.Printf("Failed to delete temp stock details for order %d: %v", orderID, err)
		return
	}

	_, err = tx.Exec(`
		DELETE FROM temp_stock_reservations WHERE order_id = ?
	`, orderID)
	if err != nil {
		log.Printf("Failed to delete temp stock reservations for order %d: %v", orderID, err)
		return
	}

	// Kembalikan stok ke inventory
	for _, detail := range stockDetails {
		var execErr error
		if detail.ProductVariantID != nil {
			// Update stok untuk product variant
			_, execErr = tx.Exec(`
				UPDATE product_variants
				SET stock = stock + ?
				WHERE id = ?
			`, detail.Quantity, *detail.ProductVariantID)
		} else {
			// Update stok untuk product utama
			_, execErr = tx.Exec(`
				UPDATE products
				SET stock = stock + ?
				WHERE id = ?
			`, detail.Quantity, detail.ProductID)
		}
		if execErr != nil {
			log.Printf("Failed to update stock for product %d or variant %v: %v", detail.ProductID, detail.ProductVariantID, execErr)
			return
		}
	}

	// Commit transaksi jika semuanya berhasil
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
	}
}

func CheckAndExpireOrders(c *gin.Context, db *sql.DB) {
	// Query untuk mengambil semua order dengan status 'waitToBuy' dan timer_expiration yang lewat
	rows, err := db.Query(`
		SELECT o.id, o.user_id, o.timer_expiration 
		FROM orders o 
		WHERE o.status = 'waitToBuy' AND o.timer_expiration < NOW()
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check expired orders"})
		return
	}
	defer rows.Close()

	var expiredOrders []struct {
		OrderID int `json:"order_id"`
		UserID  int `json:"user_id"`
	}

	// Ambil semua order yang sudah kadaluarsa
	for rows.Next() {
		var order struct {
			OrderID int `json:"order_id"`
			UserID  int `json:"user_id"`
		}
		if err := rows.Scan(&order.OrderID, &order.UserID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan order data"})
			return
		}
		expiredOrders = append(expiredOrders, order)
	}

	// Pastikan tidak ada error setelah iterasi rows
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating rows"})
		return
	}

	// Proses setiap order yang kadaluarsa
	for _, order := range expiredOrders {
		// Update status order menjadi 'expired'
		_, err := db.Exec(`
			UPDATE orders
			SET status = 'expired'
			WHERE id = ?
		`, order.OrderID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		// Kurangi jumlah hati dari user yang terkait
		_, err = db.Exec(`
			UPDATE users
			SET hearts = hearts - 1
			WHERE id = ?
		`, order.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrease hearts"})
			return
		}

		// Hapus temp_stock_reservations terkait order ini dan kembalikan stok ke inventory
		DeleteStockReservationAndReturn(order.OrderID, db)
	}

	// Response sukses
	c.JSON(http.StatusOK, gin.H{"message": "Expired orders have been processed"})
}

// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
func StockReservationRoutes(r *gin.Engine, db *sql.DB) {
	r.DELETE("/reservations/expired/clean", func(c *gin.Context) {
		CleanExpiredReservations(c, db)
	})

	stock := r.Group("/api/v1/stock-reservations")
	stock.Use(AuthMiddleware(), RoleMiddleware("user"))
	{
		stock.GET("", func(c *gin.Context) {
			GetMyStockReservations(c, db)
		})
		stock.GET("/:id/details", func(c *gin.Context) {
			GetStockReservationDetail(c, db)
		})
	}
}

func GetMyStockReservations(c *gin.Context, db *sql.DB) {
	userID := c.GetInt("user_id")

	// Ambil semua reservasi milik user ini
	rows, err := db.Query(`
		SELECT id, user_id, order_id, reserved_at, expired_at, created_at, updated_at
		FROM temp_stock_reservations
		WHERE user_id = ?
		ORDER BY reserved_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data reservasi stok"})
		return
	}
	defer rows.Close()

	var reservations []TempStockReservationModel
	for rows.Next() {
		var r TempStockReservationModel
		err := rows.Scan(
			&r.ID, &r.UserID, &r.OrderID, &r.ReservedAt, &r.ExpiredAt, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal parsing reservasi stok"})
			return
		}
		reservations = append(reservations, r)
	}

	if len(reservations) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Tidak ada reservasi stok aktif"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reservations": reservations})
}
func GetStockReservationDetail(c *gin.Context, db *sql.DB) {
	reservationID := c.Param("id")

	rows, err := db.Query(`
		SELECT id, temp_reservation_id, product_id, product_variant_id, quantity, created_at, updated_at
		FROM temp_stock_details
		WHERE temp_reservation_id = ?
	`, reservationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil detail reservasi stok"})
		return
	}
	defer rows.Close()

	var details []TempStockDetailModel
	for rows.Next() {
		var d TempStockDetailModel
		err := rows.Scan(
			&d.ID, &d.TempReservationID, &d.ProductID, &d.ProductVariantID,
			&d.Quantity, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal parsing detail reservasi stok"})
			return
		}
		details = append(details, d)
	}

	if len(details) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Tidak ada detail untuk reservasi ini"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"details": details})
}

type ExpiredReservationInfo struct {
	ID               int
	OrderID          int
	ProductID        int
	ProductVariantID sql.NullInt64
	Quantity         int
}

func CleanExpiredReservations(c *gin.Context, db *sql.DB) {
	// Ambil semua detail reservasi yang sudah expired
	rows, err := db.Query(`
		SELECT 
			d.temp_reservation_id, r.order_id, d.product_id, d.product_variant_id, d.quantity
		FROM temp_stock_details d
		JOIN temp_stock_reservations r ON d.temp_reservation_id = r.id
		WHERE r.expired_at <= NOW()
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal fetch expired reservations"})
		return
	}
	defer rows.Close()

	var details []ExpiredReservationInfo
	reservationIDs := make(map[int]struct{}) // untuk memastikan unik
	for rows.Next() {
		var info ExpiredReservationInfo
		if err := rows.Scan(&info.ID, &info.OrderID, &info.ProductID, &info.ProductVariantID, &info.Quantity); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal parsing data", "details": err.Error()})
			return
		}
		details = append(details, info)
		reservationIDs[info.ID] = struct{}{}
	}

	// Jika tidak ada data expired, kirimkan pesan informasi
	if len(details) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Tidak ada reservasi expired"})
		return
	}

	// Mulai transaksi untuk rollback stok dan update status order
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mulai transaksi", "details": err.Error()})
		return
	}

	// Rollback stok untuk setiap produk atau varian
	for _, d := range details {
		var query string
		var args []interface{}

		if d.ProductVariantID.Valid {
			// Update stok untuk product_variant
			query = `UPDATE product_variants SET stock = stock + ? WHERE id = ?`
			args = append(args, d.Quantity, d.ProductVariantID.Int64)
		} else {
			// Update stok untuk produk
			query = `UPDATE products SET stock = stock + ? WHERE id = ?`
			args = append(args, d.Quantity, d.ProductID)
		}

		_, err := tx.Exec(query, args...)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal rollback stok", "details": err.Error()})
			return
		}
	}

	// Siapkan slice ID untuk update dan delete
	var ids []interface{}
	for id := range reservationIDs {
		ids = append(ids, id)
	}

	// Membuat placeholder ? untuk query IN
	placeholder := strings.Join(strings.Split(strings.Repeat("?,", len(ids)), ","), ",")
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Tidak ada ID reservasi untuk diproses"})
		return
	}

	// Update status order menjadi expired berdasarkan reservasi expired saja
	_, err = tx.Exec(
		fmt.Sprintf(`UPDATE orders SET status = 'expired' 
		WHERE id IN (SELECT order_id FROM temp_stock_reservations WHERE expired_at <= NOW() AND id IN (%s))`, placeholder),
		ids...,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal update status order"})
		return
	}

	// Hapus reservasi yang sudah expired saja
	_, err = tx.Exec(
		fmt.Sprintf(`DELETE FROM temp_stock_reservations WHERE expired_at <= NOW() AND id IN (%s)`, placeholder),
		ids...,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal hapus reservasi", "details": err.Error()})
		return
	}

	// Commit transaksi
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal commit perubahan", "details": err.Error()})
		return
	}

	// Kirimkan respon sukses
	c.JSON(http.StatusOK, gin.H{"message": "✅ Expired reservations dibersihkan & stok dikembalikan"})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// =========================
// ➕ RestockRequest Management
// =========================
func RestockRequestRoutes(r *gin.Engine, db *sql.DB) {
	// 🔐 Khusus user
	userRestock := r.Group("/api/v1/restock-requests")
	userRestock.Use(AuthMiddleware(), RoleMiddleware("user"))
	{
		userRestock.POST("", func(c *gin.Context) {
			CreateRestockRequest(c, db)
		})
	}

	// 🔐 Khusus employee dan admin
	employeeAdminRestock := r.Group("/api/v1/restock-requests")
	employeeAdminRestock.Use(AuthMiddleware(), RoleMiddleware("employee", "admin"))
	{
		employeeAdminRestock.GET("", func(c *gin.Context) {
			GetAllRestockRequests(c, db)
		})
	}

	// 🔐 Khusus admin
	adminRestock := r.Group("/api/v1/restock-requests")
	adminRestock.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		adminRestock.PATCH("/:id", func(c *gin.Context) {
			UpdateRestockRequestStatus(c, db)
		})
		adminRestock.DELETE("/:id", func(c *gin.Context) {
			DeleteRestockRequest(c, db)
		})
	}
}

// ++++++++++++++++++++++++
//
//	RestockRequest READ
//
// ++++++++++++++++++++++++
func GetAllRestockRequests(c *gin.Context, db *sql.DB) {
	status := c.Query("status")
	productID := c.Query("product_id")

	// Menyusun query dasar untuk mengambil permintaan restock
	query := `SELECT id, user_id, product_id, product_variant_id, message, status, created_at FROM restock_requests WHERE 1=1`
	args := []interface{}{}

	// Menambahkan filter jika status diberikan
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	// Menambahkan filter jika product_id diberikan
	if productID != "" {
		query += ` AND product_id = ?`
		args = append(args, productID)
	}

	// Menjalankan query
	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil data permintaan restock"})
		return
	}
	defer rows.Close()

	// Mengambil data dari query
	var requests []RestockRequestModel
	for rows.Next() {
		var r RestockRequestModel
		if err := rows.Scan(&r.ID, &r.UserID, &r.ProductID, &r.ProductVariantID, &r.Message, &r.Status, &r.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal membaca data"})
			return
		}
		requests = append(requests, r)
	}

	// Menyusun response
	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Semua permintaan restock berhasil diambil",
		"data":    requests,
	})
}

// ++++++++++++++++++++++++
//  RestockRequest CREATE
// ++++++++++++++++++++++++

func CreateRestockRequest(c *gin.Context, db *sql.DB) {
	var input RestockRequestModel

	// Memasukkan data dari body request ke dalam model
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format JSON tidak valid"})
		return
	}

	// Validasi field wajib
	if input.UserID == 0 || input.ProductID == 0 || input.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Semua field wajib diisi (user_id, product_id, message)"})
		return
	}

	// Cek apakah user_id valid
	if !ValidateRecordExistence(c, db, "users", int(input.UserID)) {
		return
	}

	// Cek apakah product_id valid
	if !ValidateRecordExistence(c, db, "products", int(input.ProductID)) {
		return
	}

	// Cek apakah produk adalah varian
	isVarians, err := CheckIfVarians(db, int(input.ProductID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Jika produk adalah varian, pastikan product_variant_id diisi
	if isVarians && input.ProductVariantID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Produk ini adalah varian, product_variant_id harus diisi"})
		return
	}

	// Insert permintaan restock ke database
	res, err := db.Exec(`INSERT INTO restock_requests (user_id, product_id, product_variant_id, message, status, created_at) 
		VALUES (?, ?, ?, ?, 'pending', NOW())`,
		input.UserID, input.ProductID, input.ProductVariantID, input.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengirim permintaan restock"})
		return
	}

	lastID, _ := res.LastInsertId()

	// Menyusun response sukses
	c.JSON(http.StatusCreated, gin.H{
		"message": "✅ Permintaan restock berhasil dibuat",
		"data": gin.H{
			"id":                 lastID,
			"user_id":            input.UserID,
			"product_id":         input.ProductID,
			"product_variant_id": input.ProductVariantID,
			"message":            input.Message,
			"status":             "pending",
			"created_at":         input.CreatedAt,
		},
	})
}

// ++++++++++++++++++++++++
//  RestockRequest UPDATE
// ++++++++++++++++++++++++

func UpdateRestockRequestStatus(c *gin.Context, db *sql.DB) {
	// Mengambil parameter ID dari URL
	idInt, _, ok := GetIDParam(c)
	if !ok {
		return
	}

	var input struct {
		Status string `json:"status"`
	}

	// Validasi format input JSON
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format data tidak valid"})
		return
	}

	// Cek apakah status valid
	validStatuses := map[string]bool{"pending": true, "seen": true, "responded": true}
	if !validStatuses[input.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Status tidak valid permitted (pending, seen, responded)"})
		return
	}

	// Cek apakah id valid dalam tabel restock_requests
	if !ValidateRecordExistence(c, db, "restock_requests", idInt) {
		return
	}

	// Update status permintaan restock
	result, err := db.Exec(`UPDATE restock_requests SET status = ? WHERE id = ?`, input.Status, idInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengupdate status"})
		return
	}

	// Mengecek apakah ada baris yang terpengaruh (updated)
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "⚠️ Permintaan tidak ditemukan"})
		return
	}

	// Response sukses
	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Status permintaan restock diperbarui",
		"data": gin.H{
			"id":     idInt,
			"status": input.Status,
		},
	})
}

// ++++++++++++++++++++++++
//  RestockRequest DELETE
// ++++++++++++++++++++++++

func DeleteRestockRequest(c *gin.Context, db *sql.DB) {
	//id string to int
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	//Cek apakah id valid
	if !ValidateRecordExistence(c, db, "restock_requests", idInt) {
		return
	}

	_, error := db.Exec(`DELETE FROM restock_requests WHERE id = ?`, idInt)
	if error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus permintaan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Permintaan restock berhasil dihapus",
		"id":      id,
	})
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// =========================
// 💬 Notification Management
// =========================

func NotificationRoutes(r *gin.Engine, db *sql.DB) {
	// 🔓 Public (tanpa middleware) - GET notification by ID
	publicNotif := r.Group("/api/v1/notifications")
	{
		publicNotif.GET("/:id", func(c *gin.Context) {
			GetNotificationByID(c, db)
		})
	}

	// 🔐 Admin only - semua route selain GET by ID
	adminNotif := r.Group("/api/v1/notifications")
	adminNotif.Use(AuthMiddleware(), RoleMiddleware("admin"))
	{
		adminNotif.GET("/", func(c *gin.Context) {
			GetAllNotifications(c, db)
		})
		adminNotif.POST("/", func(c *gin.Context) {
			CreateNotification(c, db)
		})
		adminNotif.PATCH("/:id/read", func(c *gin.Context) {
			MarkNotificationRead(c, db)
		})
		adminNotif.DELETE("/:id", func(c *gin.Context) {
			DeleteNotification(c, db)
		})
	}
}

// ++++++++++++++++++++++++
//
//	Notification READ
//
// ++++++++++++++++++++++++
// Get all notifications (optional: filter by user_id)
func GetAllNotifications(c *gin.Context, db *sql.DB) {
	userID := c.Query("user_id")

	var rows *sql.Rows
	var err error

	if userID != "" {
		rows, err = db.Query("SELECT * FROM notifications WHERE user_id = ? ORDER BY created_at DESC", userID)
	} else {
		rows, err = db.Query("SELECT * FROM notifications ORDER BY created_at DESC")
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengambil notifikasi"})
		return
	}
	defer rows.Close()

	var notifications []NotificationModel
	for rows.Next() {
		var n NotificationModel
		if err := rows.Scan(&n.ID, &n.UserID, &n.Message, &n.IsRead, &n.CreatedAt); err == nil {
			notifications = append(notifications, n)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "✅ notifikasi berhasil diambil",
		"data":    notifications,
	})

}

// ++++++++++++++++++++++++
//
//	Notification CREATE
//
// ++++++++++++++++++++++++
// Create notification
func CreateNotification(c *gin.Context, db *sql.DB) {
	var input NotificationModel
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Format data tidak valid"})
		return
	}
	if input.UserID == 0 || input.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ user_id dan message wajib diisi"})
		return
	}

	// Cek apakah user_id valid
	if !ValidateRecordExistence(c, db, "users", int(input.UserID)) {
		return
	}

	res, err := db.Exec(`INSERT INTO notifications (user_id, message, is_read, created_at) VALUES (?, ?, false, NOW())`, input.UserID, input.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menyimpan notifikasi"})
		return
	}
	lastID, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"message": "✅ Notifikasi berhasil dibuat",
		"data": gin.H{
			"id":      lastID,
			"user_id": input.UserID,
			"message": input.Message,
			"is_read": false,
		},
	})
}

// ++++++++++++++++++++++++
//
//	Notification UPDATE
//
// ++++++++++++++++++++++++
// Mark notification as read
func MarkNotificationRead(c *gin.Context, db *sql.DB) {
	// id string to int
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	// Cek apakah id valid
	if !ValidateRecordExistence(c, db, "notifications", idInt) {
		return
	}

	var isRead bool
	error := db.QueryRow("SELECT is_read FROM notifications WHERE id = ?", id).Scan(&isRead)
	if error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal memeriksa status notifikasi"})
		return
	}
	if isRead {
		c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Notifikasi sudah dibaca sebelumnya"})
		return
	}

	_, err := db.Exec("UPDATE notifications SET is_read = true WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal mengupdate status notifikasi"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("✅ Notifikasi dengan id %s ditandai sebagai sudah dibaca", id),
		"id":      id,
		"is_read": true,
	})
}

// ++++++++++++++++++++++++
//
//	Notification DELETE
//
// ++++++++++++++++++++++++
// Delete notification
func DeleteNotification(c *gin.Context, db *sql.DB) {
	// id string to int
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	// Cek apakah id valid
	if !ValidateRecordExistence(c, db, "notifications", idInt) {
		return
	}

	// Hapus notifikasi dari database
	_, err := db.Exec("DELETE FROM notifications WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "❌ Gagal menghapus notifikasi"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "✅ Notifikasi berhasil dihapus",
		"id":      id,
		"status":  "deleted",
	})
}

// ++++++++++++++++++++++++
//
//	Notification FIND
//
// ++++++++++++++++++++++++
// Get notification by ID
func GetNotificationByID(c *gin.Context, db *sql.DB) {
	// id string to int
	idInt, id, ok := GetIDParam(c)
	if !ok {
		return
	}
	// Cek apakah id valid
	if !ValidateRecordExistence(c, db, "notifications", idInt) {
		return
	}
	//id := c.Param("id")
	var n NotificationModel
	err := db.QueryRow("SELECT * FROM notifications WHERE id = ?", id).
		Scan(&n.ID, &n.UserID, &n.Message, &n.IsRead, &n.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "❌ Notifikasi tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, n)
}

//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
