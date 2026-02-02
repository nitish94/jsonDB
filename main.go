package main

import (
	"log"
	"os"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"json-db/handlers"
	"json-db/storage"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Ensure data directory
	if err := storage.EnsureDataDir(); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	r := gin.Default()

	// Public endpoints
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if _, err := os.Stat("data"); os.IsNotExist(err) {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})
	r.POST("/login", handlers.Login)

	// Rate limiter: 10 requests per second
	limiter := tollbooth.NewLimiter(10, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})

	// Protected endpoints
	protected := r.Group("/")
	protected.Use(handlers.AuthMiddleware())
	protected.Use(tollbooth_gin.LimitHandler(limiter))
	{
		// Database and table management
		protected.GET("/dbs", handlers.ListDBs)
		protected.GET("/:db/tables", handlers.ListTables)
		protected.POST("/:db/tables", handlers.CreateTable)

		// Single record operations
		protected.POST("/:db/:table", handlers.CreateRecord)
		protected.GET("/:db/:table/:id", handlers.GetRecord)
		protected.PUT("/:db/:table/:id", handlers.UpdateRecord)
		protected.DELETE("/:db/:table/:id", handlers.DeleteRecord)
		protected.GET("/:db/:table", handlers.ListRecords)

		// Batch operations
		protected.POST("/:db/:table/batch", handlers.BatchCreateRecord)
		protected.PUT("/:db/:table/batch", handlers.BatchUpdateRecord)
		protected.DELETE("/:db/:table/batch", handlers.BatchDeleteRecord)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Starting server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}