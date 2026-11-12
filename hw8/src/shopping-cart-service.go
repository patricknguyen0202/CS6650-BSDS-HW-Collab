package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// Connect to MySQL and create the tables on first start.
	// Both stop the process on failure, and ECS restarts the task.
	db = openDB()
	createSchema()

	// Release mode drops Gin's debug route printout at startup
	gin.SetMode(gin.ReleaseMode)

	// Initialize a bare Gin router. Recovery turns any panic into a 500 response.
	router := gin.New()
	router.Use(gin.Recovery())

	// Associate the healthHandler function with the /health endpoint path.
	router.GET("/health", healthHandler)

	// Shopping cart endpoints from api.yaml (plus GET, which HW8 adds)
	router.POST("/shopping-carts", createCart)
	router.GET("/shopping-carts/:id", getCart)
	router.POST("/shopping-carts/:id/items", addItem)

	// Listen on port 8080 on all interfaces, so the container is reachable
	router.Run(":8080")
}

// healthHandler answers the ALB health check. It never touches the database,
// so a slow database never fails the health check.
func healthHandler(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, gin.H{"status": "ok"})
}
