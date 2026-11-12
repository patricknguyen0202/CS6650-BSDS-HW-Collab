package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Pool settings. Each query takes milliseconds, so 10 connections serve far more than
// 100 shopping sessions. Keeping the cap low also keeps ecs_count x maxOpenConns well
// under the small connection limit of a db.t3.micro.
const (
	maxOpenConns    = 10
	maxIdleConns    = 10              // same as open: no reconnect churn between bursts
	connMaxLifetime = 5 * time.Minute // recycle connections regularly
)

// db is the shared connection pool, set once in main() and used by every handler.
var db *sql.DB

// Two tables: one row per cart, one row per item in a cart.
// idx_customer_id serves the "all carts of a customer" access pattern.
const createCartsTable = `
CREATE TABLE IF NOT EXISTS shopping_carts (
  id          INT AUTO_INCREMENT PRIMARY KEY,
  customer_id INT NOT NULL,
  INDEX idx_customer_id (customer_id)
)`

// The (cart_id, product_id) key allows one row per product per cart, so adding the same
// product again is a single upsert. The foreign key keeps items from pointing to a missing cart.
const createCartItemsTable = `
CREATE TABLE IF NOT EXISTS cart_items (
  cart_id    INT NOT NULL,
  product_id INT NOT NULL,
  quantity   INT NOT NULL,
  PRIMARY KEY (cart_id, product_id),
  FOREIGN KEY (cart_id) REFERENCES shopping_carts(id) ON DELETE CASCADE
)`

// openDB builds the connection pool from DB_ADDR, DB_USER, DB_PASSWORD and DB_NAME.
// Any failure stops the process: ECS restarts the task, which also covers a database
// that is not ready yet.
func openDB() *sql.DB {
	env := map[string]string{}
	for _, name := range []string{"DB_ADDR", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
		value := os.Getenv(name)
		if value == "" {
			log.Fatalf("missing environment variable %s", name)
		}
		env[name] = value
	}

	// FormatDSN escapes special characters in the password
	cfg := mysql.NewConfig()
	cfg.User = env["DB_USER"]
	cfg.Passwd = env["DB_PASSWORD"]
	cfg.Net = "tcp"
	cfg.Addr = env["DB_ADDR"]
	cfg.DBName = env["DB_NAME"]

	pool, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	pool.SetMaxOpenConns(maxOpenConns)
	pool.SetMaxIdleConns(maxIdleConns)
	pool.SetConnMaxLifetime(connMaxLifetime)

	// sql.Open does not connect. Ping does, so a bad address or password fails here.
	if err := pool.Ping(); err != nil {
		log.Fatalf("ping database: %v", err)
	}
	return pool
}

// createSchema creates both tables if they do not exist yet. RDS sits in a private subnet,
// so the service creates its own schema at startup. The driver runs one statement per Exec.
func createSchema() {
	for _, stmt := range []string{createCartsTable, createCartItemsTable} {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("create schema: %v", err)
		}
	}
}
