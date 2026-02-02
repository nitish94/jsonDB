package main

import (
	"log"
	"os"

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

	// Protected endpoints
	protected := r.Group("/")
	protected.Use(handlers.AuthMiddleware())
	{
		// Collection management
		protected.POST("/collections", handlers.CreateCollection)

		// Record operations
		protected.POST("/:collection", handlers.CreateRecord)
		protected.GET("/:collection/:id", handlers.GetRecord)
		protected.PUT("/:collection/:id", handlers.UpdateRecord)
		protected.DELETE("/:collection/:id", handlers.DeleteRecord)
		protected.GET("/:collection", handlers.ListRecords)
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