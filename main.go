package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"json-db/config"
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

	// Load configuration
	config.Init()

	// Set JWT secret in handlers
	handlers.SetJWTSecret(config.GlobalConfig.JWTSecret)

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
		// Table management
		protected.POST("/tables", handlers.CreateCollection)
		protected.DELETE("/tables/:collection", handlers.CollectionValidationMiddleware(), handlers.DeleteCollection)
		protected.PUT("/tables/:collection", handlers.CollectionValidationMiddleware(), handlers.RenameCollection)

		// Single record operations
		protected.POST("/:collection", handlers.CollectionValidationMiddleware(), handlers.CreateRecord)
		protected.GET("/:collection/:id", handlers.CollectionValidationMiddleware(), handlers.GetRecord)
		protected.PUT("/:collection/:id", handlers.CollectionValidationMiddleware(), handlers.UpdateRecord)
		protected.DELETE("/:collection/:id", handlers.CollectionValidationMiddleware(), handlers.DeleteRecord)
		protected.GET("/:collection", handlers.CollectionValidationMiddleware(), handlers.ListRecords)

		// Batch operations
		protected.POST("/:collection/batch", handlers.CollectionValidationMiddleware(), handlers.BatchCreateRecord)
		protected.PUT("/:collection/batch", handlers.CollectionValidationMiddleware(), handlers.BatchUpdateRecord)
		protected.DELETE("/:collection/batch", handlers.CollectionValidationMiddleware(), handlers.BatchDeleteRecord)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + config.GlobalConfig.Port,
		Handler: r,
	}

	go func() {
		logrus.WithField("port", config.GlobalConfig.Port).Info("Starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Fatal("Server forced to shutdown:", err)
	}

	logrus.Info("Server exited")
}