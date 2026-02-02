package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"json-db/storage"
)

func CreateRecord(c *gin.Context) {
	collection := c.Param("collection")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

	var data json.RawMessage
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	id, err := storage.AddRecord(collection, data)
	if err != nil {
		log.Printf("Error creating record in %s: %v", collection, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create record"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func GetRecord(c *gin.Context) {
	collection := c.Param("collection")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	record, err := storage.FindRecordByID(collection, id)
	if err != nil {
		log.Printf("Error getting record %d from %s: %v", id, collection, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	c.JSON(http.StatusOK, record)
}

func UpdateRecord(c *gin.Context) {
	collection := c.Param("collection")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

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
		log.Printf("Error updating record %d in %s: %v", id, collection, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record updated"})
}

func DeleteRecord(c *gin.Context) {
	collection := c.Param("collection")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := storage.DeleteRecord(collection, id); err != nil {
		log.Printf("Error deleting record %d from %s: %v", id, collection, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record deleted"})
}

func ListRecords(c *gin.Context) {
	collection := c.Param("collection")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

	limitStr := c.Query("limit")
	pageStr := c.Query("page")

	limit := 25 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 50 {
		limit = 50
	}

	page := 1 // default
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	records, total, pageNum, err := storage.GetRecordsPaginated(collection, limit, page)
	if err != nil {
		log.Printf("Error listing records from %s: %v", collection, err)
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
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

	var datas []json.RawMessage
	if err := c.ShouldBindJSON(&datas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON array"})
		return
	}

	ids, err := storage.BatchAdd(collection, datas)
	if err != nil {
		log.Printf("Error batch creating records in %s: %v", collection, err)
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
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

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
		log.Printf("Error batch updating records in %s: %v", collection, err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Records updated"})
}

func BatchDeleteRecord(c *gin.Context) {
	collection := c.Param("collection")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, collection); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid collection name"})
		return
	}

	var ids []int
	if err := c.ShouldBindJSON(&ids); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON array of IDs"})
		return
	}

	if err := storage.BatchDelete(collection, ids); err != nil {
		log.Printf("Error batch deleting records in %s: %v", collection, err)
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
		log.Printf("Error creating collection %s: %v", req.Name, err)
		if err.Error() == "collection already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Collection created"})
}