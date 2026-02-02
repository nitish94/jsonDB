package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"json-db/models"
)

const DataDir = "data"

var tableMutexes = make(map[string]*sync.RWMutex) // key: db/table
var tableMutexesMu sync.Mutex

func getMutex(db, table string) *sync.RWMutex {
	key := db + "/" + table
	tableMutexesMu.Lock()
	defer tableMutexesMu.Unlock()
	if mu, ok := tableMutexes[key]; ok {
		return mu
	}
	mu := &sync.RWMutex{}
	tableMutexes[key] = mu
	return mu
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() error {
	return os.MkdirAll(DataDir, 0755)
}

// LoadCollection loads all records from a JSON file
func LoadCollection(db, table string) ([]models.Record, error) {
	filePath := filepath.Join(DataDir, db, table+".json")
	corruptPath := filepath.Join(DataDir, db, "corrupt_"+table+".json")

	// Check if corrupt file exists
	if _, err := os.Stat(corruptPath); err == nil {
		return nil, fmt.Errorf("database corrupted")
	}

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
		// Rename to corrupt
		os.Rename(filePath, corruptPath)
		return nil, fmt.Errorf("database corrupted")
	}
	return records, nil
}

// SaveCollection saves all records to a JSON file with indentation, atomically with backup rotation
func SaveCollection(db, table string, records []models.Record) error {
	filePath := filepath.Join(DataDir, db, table+".json")
	backupDir := filepath.Join(DataDir, db, "backups")
	os.MkdirAll(backupDir, 0755)

	backupCountStr := os.Getenv("BACKUP_COUNT")
	backupCount := 3
	if bc, err := strconv.Atoi(backupCountStr); err == nil {
		backupCount = bc
	}

	// Rotate backups
	for i := backupCount - 1; i >= 1; i-- {
		old := filepath.Join(backupDir, table+".bak"+strconv.Itoa(i))
		new := filepath.Join(backupDir, table+".bak"+strconv.Itoa(i+1))
		if i == backupCount-1 {
			os.Remove(new)
		}
		os.Rename(old, new)
	}

	// Create .bak1 if file exists
	if _, err := os.Stat(filePath); err == nil {
		bak1 := filepath.Join(backupDir, table+".bak1")
		if err := copyFile(filePath, bak1); err != nil {
			return err
		}
	}

	tempPath := filePath + ".tmp"

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

// FindRecordByID finds a record by ID in a table
func FindRecordByID(db, table string, id int) (*models.Record, error) {
	mu := getMutex(db, table)
	mu.RLock()
	defer mu.RUnlock()

	records, err := LoadCollection(db, table)
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
func AddRecord(db, table string, data json.RawMessage) (int, error) {
	mu := getMutex(db, table)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(db, table)
	if err != nil {
		return 0, err
	}

	// Check limit
	recordLimitStr := os.Getenv("RECORD_LIMIT")
	recordLimit := 1000
	if rl, err := strconv.Atoi(recordLimitStr); err == nil {
		recordLimit = rl
	}
	if len(records) >= recordLimit {
		return 0, fmt.Errorf("record limit exceeded: %d", recordLimit)
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

	if err := SaveCollection(db, table, records); err != nil {
		return 0, err
	}
	return newID, nil
}

// UpdateRecord updates a record by ID
func UpdateRecord(db, table string, id int, data json.RawMessage) error {
	mu := getMutex(db, table)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(db, table)
	if err != nil {
		return err
	}

	for i, record := range records {
		if record.ID == id {
			records[i].Data = data
			return SaveCollection(db, table, records)
		}
	}
	return fmt.Errorf("record not found")
}

// DeleteRecord deletes a record by ID
func DeleteRecord(db, table string, id int) error {
	mu := getMutex(db, table)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(db, table)
	if err != nil {
		return err
	}

	for i, record := range records {
		if record.ID == id {
			records = append(records[:i], records[i+1:]...)
			return SaveCollection(db, table, records)
		}
	}
	return fmt.Errorf("record not found")
}

// GetRecordsPaginated gets paginated records, sorted by ID desc (latest first)
func GetRecordsPaginated(db, table string, limit int, page int) ([]models.Record, int, int, error) {
	mu := getMutex(db, table)
	mu.RLock()
	defer mu.RUnlock()

	defaultLimitStr := os.Getenv("DEFAULT_LIMIT")
	maxLimitStr := os.Getenv("MAX_LIMIT")
	defaultLimit := 25
	maxLimit := 50
	if dl, err := strconv.Atoi(defaultLimitStr); err == nil {
		defaultLimit = dl
	}
	if ml, err := strconv.Atoi(maxLimitStr); err == nil {
		maxLimit = ml
	}

	records, err := LoadCollection(db, table)
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
func BatchAdd(db, table string, datas []json.RawMessage) ([]int, error) {
	mu := getMutex(db, table)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(db, table)
	if err != nil {
		return nil, err
	}

	recordLimitStr := os.Getenv("RECORD_LIMIT")
	recordLimit := 1000
	if rl, err := strconv.Atoi(recordLimitStr); err == nil {
		recordLimit = rl
	}
	if len(records)+len(datas) > recordLimit {
		return nil, fmt.Errorf("record limit exceeded: %d", recordLimit)
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

	if err := SaveCollection(db, table, records); err != nil {
		return nil, err
	}
	return ids, nil
}

// BatchUpdate updates multiple records atomically
func BatchUpdate(db, table string, updates []struct {
	ID   int
	Data json.RawMessage
}) error {
	mu := getMutex(db, table)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(db, table)
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

	return SaveCollection(db, table, records)
}

// BatchDelete deletes multiple records atomically
func BatchDelete(db, table string, ids []int) error {
	mu := getMutex(db, table)
	mu.Lock()
	defer mu.Unlock()

	records, err := LoadCollection(db, table)
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

	return SaveCollection(db, table, newRecords)
}

// CreateTable creates a new table with initial records
func CreateTable(db, name string, initialRecords []map[string]interface{}) error {
	// Validate name
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name); !matched {
		return fmt.Errorf("invalid table name: must be alphanumeric, underscore, or dash")
	}

	mu := getMutex(db, name)
	mu.Lock()
	defer mu.Unlock()

	// Ensure db dir exists
	dbDir := filepath.Join(DataDir, db)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dbDir, name+".json")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("table already exists")
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
		data, _ := json.Marshal(rec)
		records = append(records, models.Record{ID: id, Data: data})
	}

	return SaveCollection(db, name, records)
}

// ListDBs lists all databases (folders in data/)
func ListDBs() ([]string, error) {
	entries, err := os.ReadDir(DataDir)
	if err != nil {
		return nil, err
	}
	var dbs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dbs = append(dbs, entry.Name())
		}
	}
	return dbs, nil
}

// ListTables lists all tables (json files) in a db
func ListTables(db string) ([]string, error) {
	dbDir := filepath.Join(DataDir, db)
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("database not found")
		}
		return nil, err
	}
	var tables []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") && !strings.HasPrefix(entry.Name(), "corrupt_") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			tables = append(tables, name)
		}
	}
	return tables, nil
}