package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
	"github.com/sirupsen/logrus"
	"json-db/config"
	"json-db/models"
)

const DataDir = "data"

var collectionMutexes *lru.Cache[string, *sync.RWMutex]

func init() {
	var err error
	collectionMutexes, err = lru.New[string, *sync.RWMutex](1000) // LRU cache with capacity 1000
	if err != nil {
		logrus.Fatal("Failed to create mutex cache:", err)
	}
}

func getMutex(collection string) *sync.RWMutex {
	if mu, ok := collectionMutexes.Get(collection); ok {
		return mu
	}
	mu := &sync.RWMutex{}
	collectionMutexes.Add(collection, mu)
	return mu
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() error {
	return os.MkdirAll(DataDir, 0755)
}

// LoadCollection loads all records from a JSON file
func LoadCollection(name string) ([]models.Record, error) {
	filePath := filepath.Join(DataDir, name+".json")
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Record{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var records []models.Record
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		// Move to corrupt
		corruptPath := filepath.Join("corrupt", "corrupt_"+name+".json")
		file.Close()
		if moveErr := os.Rename(filePath, corruptPath); moveErr != nil {
			logrus.WithFields(logrus.Fields{"collection": name, "error": moveErr}).Error("Failed to move corrupted file")
			return nil, fmt.Errorf("file corrupted and failed to move")
		}
		logrus.WithFields(logrus.Fields{"collection": name, "corrupt_path": corruptPath}).Warn("Corrupted file moved")
		return nil, fmt.Errorf("file corrupted, moved to corrupt")
	}

	// Check for duplicate IDs
	idMap := make(map[int]bool)
	for _, r := range records {
		if idMap[r.ID] {
			// Move to corrupt
			file.Close()
			corruptPath := filepath.Join("corrupt", "corrupt_"+name+".json")
			if moveErr := os.Rename(filePath, corruptPath); moveErr != nil {
				logrus.WithFields(logrus.Fields{"collection": name, "error": moveErr}).Error("Failed to move file with duplicate IDs")
				return nil, fmt.Errorf("duplicate IDs and failed to move")
			}
			logrus.WithFields(logrus.Fields{"collection": name, "corrupt_path": corruptPath}).Warn("File with duplicate IDs moved to corrupt")
			return nil, fmt.Errorf("duplicate IDs, moved to corrupt")
		}
		idMap[r.ID] = true
	}

	return records, nil
}

// SaveCollection saves all records to a JSON file with indentation, atomically
func SaveCollection(name string, records []models.Record) error {
	filePath := filepath.Join(DataDir, name+".json")
	backupPath := filePath + ".bak"
	tempPath := filePath + ".tmp"

	// Create backup if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := copyFile(filePath, backupPath); err != nil {
			return err
		}
	}

	// Write to temp file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer tempFile.Close()

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(records); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath)
		return err
	}

	return nil
}

// copyFile copies src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

// FindRecordByID finds a record by ID in a collection
func FindRecordByID(collection string, id int) (*models.Record, error) {
	mu := getMutex(collection)
	mu.RLock()
	defer mu.RUnlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.ID == id {
			return &record, nil
		}
	}
	return nil, fmt.Errorf("record not found")
}

// AddRecord adds a new record with auto-generated ID
func AddRecord(collection string, data json.RawMessage) (int, error) {
	mu := getMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return 0, err
	}

	// Check limit
	if len(records) >= config.GlobalConfig.RecordLimit {
		return 0, fmt.Errorf("record limit exceeded: %d", config.GlobalConfig.RecordLimit)
	}

	// Find max ID
	maxID := 0
	for _, r := range records {
		if r.ID > maxID {
			maxID = r.ID
		}
	}
	newID := maxID + 1

	newRecord := models.Record{
		ID:   newID,
		Data: data,
	}
	records = append(records, newRecord)

	if err := SaveCollection(collection, records); err != nil {
		return 0, err
	}
	return newID, nil
}

// UpdateRecord updates a record by ID
func UpdateRecord(collection string, id int, data json.RawMessage) error {
	mu := getMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return err
	}

	for i, record := range records {
		if record.ID == id {
			records[i].Data = data
			return SaveCollection(collection, records)
		}
	}
	return fmt.Errorf("record not found")
}

// DeleteRecord deletes a record by ID
func DeleteRecord(collection string, id int) error {
	mu := getMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return err
	}

	for i, record := range records {
		if record.ID == id {
			records = append(records[:i], records[i+1:]...)
			return SaveCollection(collection, records)
		}
	}
	return fmt.Errorf("record not found")
}

// GetRecordsPaginated gets paginated records, sorted by ID desc (latest first)
func GetRecordsPaginated(collection string, limit int, page int) ([]models.Record, int, int, error) {
	mu := getMutex(collection)
	mu.RLock()
	defer mu.RUnlock()

	defaultLimit := config.GlobalConfig.DefaultLimit
	maxLimit := config.GlobalConfig.MaxLimit

	records, err := LoadCollection(collection)
	if err != nil {
		return nil, 0, 0, err
	}

	total := len(records)

	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	// Sort by ID desc
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID > records[j].ID
	})

	start := (page - 1) * limit
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > total {
		end = total
	}

	if page <= 0 {
		page = 1
	}

	return records[start:end], total, page, nil
}

// BatchAdd adds multiple records atomically
func BatchAdd(collection string, datas []json.RawMessage) ([]int, error) {
	mu := getMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return nil, err
	}

	if len(records)+len(datas) > config.GlobalConfig.RecordLimit {
		return nil, fmt.Errorf("record limit exceeded: %d", config.GlobalConfig.RecordLimit)
	}

	maxID := 0
	for _, r := range records {
		if r.ID > maxID {
			maxID = r.ID
		}
	}

	ids := make([]int, len(datas))
	for i, data := range datas {
		maxID++
		ids[i] = maxID
		records = append(records, models.Record{ID: maxID, Data: data})
	}

	if err := SaveCollection(collection, records); err != nil {
		return nil, err
	}
	return ids, nil
}

// BatchUpdate updates multiple records atomically
func BatchUpdate(collection string, updates []struct {
	ID   int
	Data json.RawMessage
}) error {
	mu := getMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return err
	}

	recordMap := make(map[int]int) // id to index
	for i, r := range records {
		recordMap[r.ID] = i
	}

	for _, update := range updates {
		if idx, exists := recordMap[update.ID]; exists {
			records[idx].Data = update.Data
		} else {
			return fmt.Errorf("record %d not found", update.ID)
		}
	}

	return SaveCollection(collection, records)
}

// BatchDelete deletes multiple records atomically
func BatchDelete(collection string, ids []int) error {
	mu := getMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(collection)
	if err != nil {
		return err
	}

	idSet := make(map[int]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	newRecords := make([]models.Record, 0)
	for _, r := range records {
		if !idSet[r.ID] {
			newRecords = append(newRecords, r)
		}
	}

	return SaveCollection(collection, newRecords)
}

// CreateCollection creates a new collection with initial records
func CreateCollection(name string, initialRecords []map[string]interface{}) error {
	// Validate name
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name); !matched {
		return fmt.Errorf("invalid collection name: must be alphanumeric, underscore, or dash")
	}

	mu := getMutex(name)
	mu.Lock()
	defer mu.Unlock()

	filePath := filepath.Join(DataDir, name+".json")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("collection already exists")
	}

	if len(initialRecords) == 0 {
		return fmt.Errorf("at least one record required")
	}

	// Collect IDs and validate
	idMap := make(map[int]bool)
	maxID := 0
	for _, rec := range initialRecords {
		if idVal, hasID := rec["id"]; hasID {
			idFloat, ok := idVal.(float64)
			if !ok {
				return fmt.Errorf("id must be an integer")
			}
			id := int(idFloat)
			if idMap[id] {
				return fmt.Errorf("duplicate id: %d", id)
			}
			idMap[id] = true
			if id > maxID {
				maxID = id
			}
		}
	}

	// Create records, assign IDs for those without
	nextID := maxID + 1
	records := make([]models.Record, 0, len(initialRecords))
	for _, rec := range initialRecords {
		var id int
		if idVal, hasID := rec["id"]; hasID {
			id = int(idVal.(float64))
		} else {
			id = nextID
			nextID++
		}
		delete(rec, "id")
		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		records = append(records, models.Record{ID: id, Data: data})
	}

	return SaveCollection(name, records)
}

// DeleteCollection moves the collection to bin with timestamp
func DeleteCollection(name string) error {
	mu := getMutex(name)
	mu.Lock()
	defer mu.Unlock()

	filePath := filepath.Join(DataDir, name+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("collection not found")
	}

	timestamp := time.Now().Format("20060102_150405")
	binPath := filepath.Join("bin", timestamp+"_"+name+".json")
	if err := os.Rename(filePath, binPath); err != nil {
		logrus.WithFields(logrus.Fields{"collection": name, "error": err}).Error("Failed to move collection to bin")
		return err
	}

	// Remove from mutex cache
	collectionMutexes.Remove(name)

	logrus.WithFields(logrus.Fields{"collection": name, "bin_path": binPath}).Info("Collection moved to bin")
	return nil
}

// RenameCollection renames the collection file
func RenameCollection(oldName, newName string) error {
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, newName); !matched {
		return fmt.Errorf("invalid collection name")
	}

	mu := getMutex(oldName)
	mu.Lock()
	defer mu.Unlock()

	oldPath := filepath.Join(DataDir, oldName+".json")
	newPath := filepath.Join(DataDir, newName+".json")
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("collection already exists")
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		logrus.WithFields(logrus.Fields{"old": oldName, "new": newName, "error": err}).Error("Failed to rename collection")
		return err
	}

	// Update mutex cache
	if mu, ok := collectionMutexes.Get(oldName); ok {
		collectionMutexes.Remove(oldName)
		collectionMutexes.Add(newName, mu)
	}

	logrus.WithFields(logrus.Fields{"old": oldName, "new": newName}).Info("Collection renamed")
	return nil
}