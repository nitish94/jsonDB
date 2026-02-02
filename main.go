package main

import (
	"os"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"json-db/handlers"
	"json-db/storage"
)

func main() {
	// Setup logging with rotation
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(&lumberjack.Logger{
		Filename:   "logs/app.log",
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	})
	logrus.SetLevel(logrus.InfoLevel)

	// Load .env
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found")
	}

	// Ensure data directory
	if err := storage.EnsureDataDir(); err != nil {
		logrus.Fatal("Failed to create data directory:", err)
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
		// Collection management
		protected.POST("/collections", handlers.CreateCollection)
		protected.DELETE("/collections/:collection", handlers.DeleteCollection)
		protected.PUT("/collections/:collection", handlers.RenameCollection)

		// Single record operations
		protected.POST("/:collection", handlers.CreateRecord)
		protected.GET("/:collection/:id", handlers.GetRecord)
		protected.PUT("/:collection/:id", handlers.UpdateRecord)
		protected.DELETE("/:collection/:id", handlers.DeleteRecord)
		protected.GET("/:collection", handlers.ListRecords)

		// Batch operations
		protected.POST("/:collection/batch", handlers.BatchCreateRecord)
		protected.PUT("/:collection/batch", handlers.BatchUpdateRecord)
		protected.DELETE("/:collection/batch", handlers.BatchDeleteRecord)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	logrus.WithField("port", port).Info("Starting server")
	if err := r.Run(":" + port); err != nil {
		logrus.Fatal("Failed to start server:", err)
	}
}