package main

import (
	//"database/sql"
	"time"
)

type CategoryModel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProductsModel struct {
	ID            int       `json:"id"`
	CategoryID    int       `json:"category_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	IsVarians     bool      `json:"is_varians"`
	IsDiscounted  *bool     `json:"is_discounted"` // NULLable, jadi pakai pointer
	DiscountPrice *int      `json:"discount_price"`
	Price         *int      `json:"price"`
	Stock         *int      `json:"stock"`
	IsService     bool      `json:"is_service"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	SearchVector  *string   `json:"search_vector"`
}

type ProductVariantModel struct {
	ID            int       `json:"id"`
	ProductID     int       `json:"product_id"`
	Name          string    `json:"name"`
	Color         *string   `json:"color"` // NULLable
	Price         int       `json:"price"`
	IsDiscounted  bool      `json:"is_discounted"`
	DiscountPrice *int      `json:"discount_price"` // NULLable
	Stock         int       `json:"stock"`
	IsService     bool      `json:"is_service"`
	SearchVector  *string   `json:"search_vector"` // NULLable
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProductImageModel struct {
	ID        int    `json:"id"`
	ProductID int    `json:"product_id"`
	ImageURL  string `json:"image_url"`
}

type CartModel struct {
	ID         int       `json:"id"` // Sama dengan user_id
	TotalPrice int       `json:"total_price"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type CartItemModel struct {
	ID               int       `json:"id"`
	CartID           int       `json:"cart_id"`
	ProductID        int       `json:"product_id"`
	ProductVariantID *int      `json:"product_variant_id"` // bisa NULL, jadi pakai pointer
	Quantity         int       `json:"quantity"`
	PricePerItem     int       `json:"price_per_item"`
	TotalPrice       int       `json:"total_price"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type OrderModel struct {
	ID              int       `json:"id"`
	UserID          int       `json:"user_id"`
	CartUserID      *int      `json:"cart_user_id"` // bisa NULL, pakai pointer
	Status          string    `json:"status"`
	TotalPrice      int       `json:"total_price"`
	TimerExpiration time.Time `json:"timer_expiration"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type OrderItemModel struct {
	ID               int  `json:"id"`
	OrderID          int  `json:"order_id"`
	ProductID        int  `json:"product_id"`
	ProductVariantID *int `json:"product_variant_id"` // bisa NULL, pakai pointer
	Quantity         int  `json:"quantity"`
	PriceAtPurchase  int  `json:"price_at_purchase"`
	TotalPrice       int  `json:"total_price"`
}

type TempStockReservationModel struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	OrderID    int       `json:"order_id"`
	ReservedAt time.Time `json:"reserved_at"`
	ExpiredAt  time.Time `json:"expired_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TempStockDetailModel struct {
	ID                int       `json:"id"`
	TempReservationID int       `json:"temp_reservation_id"`
	ProductID         int       `json:"product_id"`
	ProductVariantID  *int      `json:"product_variant_id"` // Bisa null jika tidak ada varian
	Quantity          int       `json:"quantity"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type RestockRequestModel struct {
	ID               int       `json:"id"`
	UserID           int       `json:"user_id"`            // Tak Bisa NULL
	ProductID        int       `json:"product_id"`         // Tak Bisa NULL
	ProductVariantID *int      `json:"product_variant_id"` // Bisa NULL
	Message          string    `json:"message"`            //Tak Bisa NULL
	Status           string    `json:"status"`             // ENUM: "pending", "seen", "responded"
	CreatedAt        time.Time `json:"created_at"`
}

type NotificationModel struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}
