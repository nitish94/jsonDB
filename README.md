# JSON Database System

A simple REST API-based database system using JSON files as collections. Built with Golang and Gin.

## Features

- JWT-based authentication for API access.
- CRUD operations (Create, Read, Update, Delete) for JSON records.
- Batch operations for efficient bulk data handling.
- Table management: create, delete (archive to bin/), rename.
- Each JSON file acts as a separate table with record limits.
- Automatic ID generation (incremental integers).
- Pagination for listing records.
- Hard limits on number of records per table to prevent memory issues.
- Atomic writes with backup system for data integrity.
- Corruption detection: invalid JSON or duplicate IDs move files to `corrupt/` folder.
- Transactional batch operations (all-or-nothing).
- Rate limiting (10 requests/second) to prevent abuse.
- Structured logging with file rotation.
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
#### Table Management
- `POST /tables` - Create a new table (JSON file). Body: `{"name": "table_name", "records": [{"key": "value", ...}, ...]}`. At least one record required. IDs auto-assigned if not provided, must be unique integers.
- `DELETE /tables/:table` - Delete (archive) a table. Moves to `bin/` with timestamp.
- `PUT /tables/:table` - Rename a table. Body: `{"name": "new_name"}`.

#### Record Operations
- `POST /:collection` - Create a new record. Body: JSON object without "id". Limited by RECORD_LIMIT.
- `GET /:collection/:id` - Retrieve a record by ID.
- `PUT /:collection/:id` - Update a record by ID. Body: JSON object without "id".
- `DELETE /:collection/:id` - Delete a record by ID.
- `GET /:collection?limit=25&page=1` - List records with pagination. Defaults from .env: DEFAULT_LIMIT=25, MAX_LIMIT=50, page=1. Returns latest records first (sorted by ID desc).

#### Batch Operations (Transactional)
- `POST /:collection/batch` - Create multiple records. Body: Array of JSON objects. Returns array of IDs.
- `PUT /:collection/batch` - Update multiple records. Body: Array of `{"id": int, "data": object}`.
- `DELETE /:collection/batch` - Delete multiple records. Body: Array of IDs.

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
```
POST /login
{
  "username": "admin",
  "password": "admin"
}
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

Use this token in `Authorization: Bearer <token>` for subsequent requests.

### Create Table
```
POST /tables
Authorization: Bearer <token>
{
  "name": "newtable",
  "records": [
    {"name": "Item1", "value": 100},
    {"id": 5, "name": "Item2", "value": 200}
  ]
}
```

### Delete Table
```
DELETE /tables/users
Authorization: Bearer <token>
```

### Rename Table
```
PUT /tables/oldname
Authorization: Bearer <token>
{
  "name": "newname"
}
```

### Create Record
```
POST /users
Authorization: Bearer <token>
{
  "name": "David",
  "email": "david@example.com",
  "age": 28
}
```

Response:
```json
{
  "id": 4
}
```

### List Records
```
GET /users
Authorization: Bearer <token>
```

Response:
```json
{
  "records": [...],
  "total": 4,
  "page": 1,
  "limit": 25
}
```

### Batch Create Records
```
POST /users/batch
Authorization: Bearer <token>
[
  {"name": "Eve", "email": "eve@example.com"},
  {"name": "Frank", "email": "frank@example.com"}
]
```

Response:
```json
{
  "ids": [5, 6]
}
```

### Batch Update Records
```
PUT /users/batch
Authorization: Bearer <token>
[
  {"id": 1, "data": {"name": "Alice Updated"}},
  {"id": 2, "data": {"name": "Bob Updated"}}
]
```

### Batch Delete Records
```
DELETE /users/batch
Authorization: Bearer <token>
[3, 4]
```

### Additional Examples

# Get a specific record
curl -H "Authorization: Bearer <token>" http://localhost:5000/users/1

# Update a record
curl -X PUT -H "Authorization: Bearer <token>" -d '{"name": "Updated Name"}' http://localhost:5000/users/1

# List records with pagination
curl -H "Authorization: Bearer <token>" "http://localhost:5000/users?limit=10&page=1"

# Health check
curl http://localhost:5000/health

## Configuration (.env)

- `PORT=5000` - Server port.
- `DEFAULT_LIMIT=25` - Default pagination limit.
- `MAX_LIMIT=50` - Maximum pagination limit.
- `USERNAME=admin` - Login username.
- `PASSWORD=admin` - Login password.
- `RECORD_LIMIT=1000` - Maximum records per table.
- `JWT_SECRET=your-secret-key` - Secret key for JWT signing.

## Working Brief

This API provides a simple JSON-based database system where each table is a JSON file containing an array of records. Operations are thread-safe with per-table locking, atomic writes for data integrity, and corruption detection (files with invalid JSON or duplicate IDs are moved to `corrupt/` folder). Authentication uses JWT tokens, and rate limiting prevents abuse. Logging captures all operations for monitoring.

## Notes

- IDs are auto-generated as incremental integers.
- JSON files are kept indented for readability.
- Tables must have valid names (alphanumeric, underscore, dash).
- Input validation includes table name, ID types, and JSON structure.
- Thread-safe operations with per-table locking (readers-writer mutex, LRU cached for memory efficiency).
- Tables are created via POST /tables with initial records.
- Delete tables: moved to `bin/` with timestamp (e.g., `20231001_120000_users.json`).
- Rename tables: file renamed, mutex updated.
- Corrupted tables: moved to `corrupt/` with prefix (e.g., `corrupt_users.json`).
- Hard limit on records per table to prevent memory issues.
- Atomic writes: data is written to temp file, then renamed; backups created before changes.
- Batch operations are transactional: all succeed or all fail.
- Rate limiting applied to protected endpoints.
- Logging: structured JSON logs with rotation (max 10MB, 3 backups, 28 days).
- Mutex cache: LRU with capacity 1000 to prevent unbounded memory growth.
- No joins or complex queries; only single table operations.
- Authentication required for all data operations; credentials not modifiable via API.