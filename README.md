# JSON Database System

A hierarchical REST API-based database system using JSON files as tables within databases. Built with Golang and Gin.

## Data Structure

- **Databases**: Folders in `data/` (e.g., `data/demo/`)
- **Tables**: JSON files within database folders (e.g., `data/demo/users.json`)
- Each table is a JSON array of objects with auto-generated integer IDs.
- No cross-table queries or joins.

## Features

- JWT-based authentication for API access.
- CRUD operations (Create, Read, Update, Delete) for JSON records.
- Batch operations for efficient bulk data handling.
- Automatic ID generation (incremental integers).
- Pagination for listing records.
- Hard limits on number of records per table to prevent memory issues.
- Atomic writes with rotating backup system for data integrity.
- Transactional batch operations (all-or-nothing).
- Corruption detection and handling (renames corrupt files).
- Rate limiting (10 requests/second) to prevent abuse.
- Proper logging and error handling.
- Configurable settings via .env file.

## Authentication

- All CRUD and collection management APIs require JWT authentication.
- Login via `POST /login` with `{"username": "admin", "password": "admin"}` (credentials from .env).
- Include `Authorization: Bearer <token>` in headers for protected endpoints.

## API Endpoints

### Public Endpoints
- `GET /health` - Health check endpoint.
- `GET /ready` - Readiness check endpoint.
- `POST /login` - Login to get JWT token. Body: `{"username": "admin", "password": "admin"}`.

### Protected Endpoints (Require JWT)
#### Database Management
- `GET /dbs` - List all databases.
- `GET /:db/tables` - List all tables in a database.
- `POST /:db/tables` - Create a new table. Body: `{"name": "table_name", "records": [{"key": "value", ...}, ...]}`. At least one record required. IDs auto-assigned if not provided, must be unique integers.

#### Record Operations
- `POST /:db/:table` - Create a new record. Body: JSON object without "id". Limited by RECORD_LIMIT.
- `GET /:db/:table/:id` - Retrieve a record by ID.
- `PUT /:db/:table/:id` - Update a record by ID. Body: JSON object without "id".
- `DELETE /:db/:table/:id` - Delete a record by ID.
- `GET /:db/:table?limit=25&page=1` - List records with pagination. Defaults from .env: DEFAULT_LIMIT=25, MAX_LIMIT=50, page=1. Returns latest records first (sorted by ID desc).

#### Batch Operations (Transactional)
- `POST /:db/:table/batch` - Create multiple records. Body: Array of JSON objects. Returns array of IDs.
- `PUT /:db/:table/batch` - Update multiple records. Body: Array of `{"id": int, "data": object}`.
- `DELETE /:db/:table/batch` - Delete multiple records. Body: Array of IDs.

## Project Structure

- `handlers/` - Gin route handlers.
- `models/` - Data models (Record struct).
- `storage/` - File I/O operations for JSON collections.
- `data/` - Directory containing JSON collection files.

## Setup

1. Ensure Go is installed.
2. Run `go mod tidy` to install dependencies.
3. Create `.env` file with `PORT=5000` (or desired port).
4. Run `go run main.go` to start the server.

## Example Usage

### Login
```bash
curl -X POST http://localhost:5000/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

Use this token in `Authorization: Bearer <token>` for subsequent requests.

### List Databases
```bash
curl -X GET http://localhost:5000/dbs \
  -H "Authorization: Bearer <token>"
```

Response:
```json
{
  "databases": ["demo"]
}
```

### List Tables in Database
```bash
curl -X GET http://localhost:5000/demo/tables \
  -H "Authorization: Bearer <token>"
```

Response:
```json
{
  "tables": ["users", "logs"]
}
```

### Create Table
```bash
curl -X POST http://localhost:5000/demo/tables \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "products",
    "records": [
      {"name": "Laptop", "price": 999},
      {"name": "Book", "price": 19}
    ]
  }'
```

### Create Record
```bash
curl -X POST http://localhost:5000/demo/users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "David", "email": "david@example.com", "age": 28}'
```

Response:
```json
{
  "id": 3
}
```

### List Records
```bash
curl -X GET "http://localhost:5000/demo/users?limit=10" \
  -H "Authorization: Bearer <token>"
```

Response:
```json
{
  "records": [...],
  "total": 3,
  "page": 1,
  "limit": 10
}
```

### Batch Create Records
```bash
curl -X POST http://localhost:5000/demo/users/batch \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '[
    {"name": "Eve", "email": "eve@example.com"},
    {"name": "Frank", "email": "frank@example.com"}
  ]'
```

Response:
```json
{
  "ids": [4, 5]
}
```

### Batch Update Records
```bash
curl -X PUT http://localhost:5000/demo/users/batch \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '[
    {"id": 1, "data": {"name": "Alice Updated"}},
    {"id": 2, "data": {"name": "Bob Updated"}}
  ]'
```

### Batch Delete Records
```bash
curl -X DELETE http://localhost:5000/demo/users/batch \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '[3, 4]'
```

## Configuration (.env)

- `PORT=5000` - Server port.
- `DEFAULT_LIMIT=25` - Default pagination limit.
- `MAX_LIMIT=50` - Maximum pagination limit.
- `USERNAME=admin` - Login username.
- `PASSWORD=admin` - Login password.
- `RECORD_LIMIT=1000` - Maximum records per table.
- `BACKUP_COUNT=3` - Number of backup files to keep per table.

## Notes

- IDs are auto-generated as incremental integers.
- JSON files are kept indented for readability.
- Databases and tables must have valid names (alphanumeric, underscore, dash).
- Input validation includes names, ID types, and JSON structure.
- Thread-safe operations with per-table locking (readers-writer mutex).
- Tables are created via POST /:db/tables with initial records.
- Hard limit on records per table to prevent memory issues.
- Atomic writes: data is written to temp file, then renamed; rotating backups created before changes.
- Corruption handling: invalid JSON files are renamed to `corrupt_*` and reported.
- Batch operations are transactional: all succeed or all fail.
- Rate limiting applied to protected endpoints.
- No joins or complex queries; only single table operations.
- Authentication required for all data operations; credentials not modifiable via API.