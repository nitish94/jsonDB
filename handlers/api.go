package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"json-db/config"
	"json-db/storage"
)

// CollectionValidationMiddleware validates the collection name parameter
func CollectionValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		collection := c.Param("collection")
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func CreateRecord(c *gin.Context) {
	collection := c.Param("collection")

	var data json.RawMessage
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	id, err := storage.AddRecord(collection, data)
	if err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "error": err}).Error("Error creating record")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create record"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func GetRecord(c *gin.Context) {
	collection := c.Param("collection")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	record, err := storage.FindRecordByID(collection, id)
	if err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "id": id, "error": err}).Error("Error getting record")
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	c.JSON(http.StatusOK, record)
}

func UpdateRecord(c *gin.Context) {
	collection := c.Param("collection")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var data json.RawMessage
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := storage.UpdateRecord(collection, id, data); err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "id": id, "error": err}).Error("Error updating record")
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record updated"})
}

func DeleteRecord(c *gin.Context) {
	collection := c.Param("collection")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := storage.DeleteRecord(collection, id); err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "id": id, "error": err}).Error("Error deleting record")
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record deleted"})
}

func ListRecords(c *gin.Context) {
	collection := c.Param("collection")

	limitStr := c.Query("limit")
	pageStr := c.Query("page")

	limit := config.GlobalConfig.DefaultLimit // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > config.GlobalConfig.MaxLimit {
		limit = config.GlobalConfig.MaxLimit
	}

	page := 1 // default
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	records, total, pageNum, err := storage.GetRecordsPaginated(collection, limit, page)
	if err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "error": err}).Error("Error listing records")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list records"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   total,
		"page":    pageNum,
		"limit":   limit,
	})
}

func BatchCreateRecord(c *gin.Context) {
	collection := c.Param("collection")

	var datas []json.RawMessage
	if err := c.ShouldBindJSON(&datas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON array"})
		return
	}

	ids, err := storage.BatchAdd(collection, datas)
	if err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "error": err}).Error("Error batch creating records")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ids": ids})
}

type BatchUpdateRequest struct {
	ID   int             `json:"id"`
	Data json.RawMessage `json:"data"`
}

func BatchUpdateRecord(c *gin.Context) {
	collection := c.Param("collection")

	var updates []BatchUpdateRequest
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON array"})
		return
	}

	updateStructs := make([]struct {
		ID   int
		Data json.RawMessage
	}, len(updates))
	for i, u := range updates {
		updateStructs[i] = struct {
			ID   int
			Data json.RawMessage
		}{ID: u.ID, Data: u.Data}
	}

	if err := storage.BatchUpdate(collection, updateStructs); err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "error": err}).Error("Error batch updating records")
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Records updated"})
}

func BatchDeleteRecord(c *gin.Context) {
	collection := c.Param("collection")

	var ids []int
	if err := c.ShouldBindJSON(&ids); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON array of IDs"})
		return
	}

	if err := storage.BatchDelete(collection, ids); err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "error": err}).Error("Error batch deleting records")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete records"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Records deleted"})
}

type CreateCollectionRequest struct {
	Name    string                   `json:"name" binding:"required"`
	Records []map[string]interface{} `json:"records" binding:"required"`
}

func CreateCollection(c *gin.Context) {
	var req CreateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if len(req.Records) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one record required"})
		return
	}

	// Additional validation for records
	for _, rec := range req.Records {
		if idVal, hasID := rec["id"]; hasID {
			if _, ok := idVal.(float64); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "ID must be integer"})
				return
			}
		}
	}

	if err := storage.CreateCollection(req.Name, req.Records); err != nil {
		logrus.WithFields(logrus.Fields{"collection": req.Name, "error": err}).Error("Error creating collection")
		if err.Error() == "collection already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	logrus.WithField("collection", req.Name).Info("Collection created")
	c.JSON(http.StatusCreated, gin.H{"message": "Collection created"})
}

type RenameCollectionRequest struct {
	Name string `json:"name" binding:"required"`
}

func DeleteCollection(c *gin.Context) {
	collection := c.Param("collection")

	if err := storage.DeleteCollection(collection); err != nil {
		logrus.WithFields(logrus.Fields{"collection": collection, "error": err}).Error("Error deleting collection")
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Collection deleted"})
}

func RenameCollection(c *gin.Context) {
	collection := c.Param("collection")

	var req RenameCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := storage.RenameCollection(collection, req.Name); err != nil {
		logrus.WithFields(logrus.Fields{"old": collection, "new": req.Name, "error": err}).Error("Error renaming collection")
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Collection renamed"})
}