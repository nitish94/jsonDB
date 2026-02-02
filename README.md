# JSON Database System

A simple REST API-based database system using JSON files as collections. Built with Golang and Gin.

## Features

- JWT-based authentication for API access.
- CRUD operations (Create, Read, Update, Delete) for JSON records.
- Batch operations for efficient bulk data handling.
- Each JSON file acts as a separate collection/database with record limits.
- Automatic ID generation (incremental integers).
- Pagination for listing records.
- Hard limits on number of records per collection to prevent memory issues.
- Atomic writes with backup system for data integrity.
- Transactional batch operations (all-or-nothing).
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
#### Collection Management
- `POST /collections` - Create a new collection. Body: `{"name": "collection_name", "records": [{"key": "value", ...}, ...]}`. At least one record required. IDs auto-assigned if not provided, must be unique integers.

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

### Create Collection
```
POST /collections
Authorization: Bearer <token>
{
  "name": "newcollection",
  "records": [
    {"name": "Item1", "value": 100},
    {"id": 5, "name": "Item2", "value": 200}
  ]
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

## Configuration (.env)

- `PORT=5000` - Server port.
- `DEFAULT_LIMIT=25` - Default pagination limit.
- `MAX_LIMIT=50` - Maximum pagination limit.
- `USERNAME=admin` - Login username.
- `PASSWORD=admin` - Login password.
- `RECORD_LIMIT=1000` - Maximum records per collection.

## Notes

- IDs are auto-generated as incremental integers.
- JSON files are kept indented for readability.
- Collections must have valid names (alphanumeric, underscore, dash).
- Input validation includes collection name, ID types, and JSON structure.
- Thread-safe operations with per-collection locking (readers-writer mutex).
- Collections are created via POST /collections with initial records.
- Hard limit on records per collection to prevent memory issues.
- Atomic writes: data is written to temp file, then renamed; backups created before changes.
- Batch operations are transactional: all succeed or all fail.
- Rate limiting applied to protected endpoints.
- No joins or complex queries; only single collection operations.
- Authentication required for all data operations; credentials not modifiable via API.