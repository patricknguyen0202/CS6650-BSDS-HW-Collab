package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// cart is the response body of POST /shopping-carts and GET /shopping-carts/{id}.
type cart struct {
	ShoppingCartID int32      `json:"shopping_cart_id"`
	CustomerID     int32      `json:"customer_id"`
	Items          []cartItem `json:"items"`
}

// cartItem is one line of a cart, and also the request body of POST /items.
type cartItem struct {
	ProductID int32 `json:"product_id" binding:"required,min=1"`
	Quantity  int32 `json:"quantity" binding:"required,min=1"`
}

// createCartRequest is the request body of POST /shopping-carts.
type createCartRequest struct {
	CustomerID int32 `json:"customer_id" binding:"required,min=1"`
}

// errorResponse is the JSON body of every error (the Error schema in api.yaml).
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.IndentedJSON(status, errorResponse{Error: code, Message: message})
}

// respondInternalError logs the real error (it goes to CloudWatch Logs) and sends a
// generic 500, so SQL errors never reach the client.
func respondInternalError(c *gin.Context, op string, err error) {
	log.Printf("%s: %v", op, err)
	respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
}

// parseCartID reads the {id} path parameter. It must be a positive int32, so a value
// like "1 OR 1=1" is rejected here and never reaches SQL.
func parseCartID(c *gin.Context) (int32, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil || id < 1 {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "The shopping cart ID must be a positive integer")
		return 0, false
	}
	return int32(id), true
}

// createCart: POST /shopping-carts. One table, one statement, so no transaction.
func createCart(c *gin.Context) {
	var req createCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "The provided input is invalid")
		return
	}

	res, err := db.ExecContext(c.Request.Context(),
		"INSERT INTO shopping_carts (customer_id) VALUES (?)", req.CustomerID)
	if err != nil {
		respondInternalError(c, "create cart", err)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		respondInternalError(c, "create cart: last insert id", err)
		return
	}

	// A new cart has no items: send [] rather than null
	c.IndentedJSON(http.StatusCreated, cart{
		ShoppingCartID: int32(id),
		CustomerID:     req.CustomerID,
		Items:          []cartItem{},
	})
}

// getCartQuery reads a cart and all its items in one round trip. Both sides of the join
// use a primary key. LEFT JOIN returns one row with NULL item columns for an empty cart.
const getCartQuery = `
SELECT c.customer_id, i.product_id, i.quantity
FROM shopping_carts c
LEFT JOIN cart_items i ON i.cart_id = c.id
WHERE c.id = ?`

// getCart: GET /shopping-carts/{id}. No rows means the cart does not exist.
func getCart(c *gin.Context) {
	id, ok := parseCartID(c)
	if !ok {
		return
	}

	rows, err := db.QueryContext(c.Request.Context(), getCartQuery, id)
	if err != nil {
		respondInternalError(c, "get cart", err)
		return
	}
	// Without Close the connection never returns to the pool
	defer rows.Close()

	result := cart{ShoppingCartID: id, Items: []cartItem{}}
	found := false
	for rows.Next() {
		var productID, quantity sql.NullInt32
		if err := rows.Scan(&result.CustomerID, &productID, &quantity); err != nil {
			respondInternalError(c, "get cart: scan", err)
			return
		}
		found = true
		// NULL item columns: the cart exists but is empty
		if productID.Valid && quantity.Valid {
			result.Items = append(result.Items, cartItem{ProductID: productID.Int32, Quantity: quantity.Int32})
		}
	}
	if err := rows.Err(); err != nil {
		respondInternalError(c, "get cart: rows", err)
		return
	}
	if !found {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Shopping cart not found")
		return
	}

	c.IndentedJSON(http.StatusOK, result)
}

// addItemQuery adds the quantity to the existing row of this product, or inserts a new row.
// The row alias (AS new) needs MySQL 8.0.19 or later and replaces the deprecated VALUES().
const addItemQuery = `
INSERT INTO cart_items (cart_id, product_id, quantity) VALUES (?, ?, ?) AS new
ON DUPLICATE KEY UPDATE quantity = cart_items.quantity + new.quantity`

// addItem: POST /shopping-carts/{id}/items. The cart check and the item upsert touch
// two tables, so they run in one transaction. The upsert is atomic on its
// (cart_id, product_id) row, so no SELECT ... FOR UPDATE is needed.
func addItem(c *gin.Context) {
	id, ok := parseCartID(c)
	if !ok {
		return
	}
	var item cartItem
	if err := c.ShouldBindJSON(&item); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "The provided input is invalid")
		return
	}

	ctx := c.Request.Context()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		respondInternalError(c, "add item: begin", err)
		return
	}
	// No effect after a successful Commit
	defer tx.Rollback()

	var found int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM shopping_carts WHERE id = ?", id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Shopping cart not found")
		return
	}
	if err != nil {
		respondInternalError(c, "add item: find cart", err)
		return
	}

	if _, err := tx.ExecContext(ctx, addItemQuery, id, item.ProductID, item.Quantity); err != nil {
		respondInternalError(c, "add item: upsert", err)
		return
	}
	if err := tx.Commit(); err != nil {
		respondInternalError(c, "add item: commit", err)
		return
	}

	c.Status(http.StatusNoContent)
}
