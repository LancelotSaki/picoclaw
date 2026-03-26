package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
)

// DatabaseTool provides dynamic database query capabilities
// It accepts connection parameters at runtime and executes queries
type DatabaseTool struct{}

// NewDatabaseTool creates a new database tool
func NewDatabaseTool() *DatabaseTool {
	return &DatabaseTool{}
}

func (t *DatabaseTool) Name() string {
	return "db_execute"
}

func (t *DatabaseTool) Description() string {
	return `Execute SQL queries on various databases (MySQL, PostgreSQL, Oracle).

Supported databases:
- MySQL: specify type="mysql", host, port, user, password, database
- PostgreSQL: specify type="postgresql", host, port, user, password, database
- Oracle: specify type="oracle", host, port, user, password, service_name

Examples:
- SELECT * FROM users WHERE id = 1
- INSERT INTO users (name, email) VALUES ('John', 'john@example.com')
- UPDATE users SET name = 'Jane' WHERE id = 1
- DELETE FROM users WHERE id = 1`
}

func (t *DatabaseTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"description": "Database type: mysql, postgresql, oracle",
				"enum":        []string{"mysql", "postgresql", "oracle"},
			},
			"host": map[string]any{
				"type":        "string",
				"description": "Database host (e.g., localhost, 192.168.1.100)",
			},
			"port": map[string]any{
				"type":        "integer",
				"description": "Database port (default: 3306 for mysql, 5432 for postgresql, 1521 for oracle)",
			},
			"user": map[string]any{
				"type":        "string",
				"description": "Database username",
			},
			"password": map[string]any{
				"type":        "string",
				"description": "Database password",
			},
			"database": map[string]any{
				"type":        "string",
				"description": "Database name (for MySQL/PostgreSQL)",
			},
			"service_name": map[string]any{
				"type":        "string",
				"description": "Oracle service name (for Oracle)",
			},
			"ssl_mode": map[string]any{
				"type":        "string",
				"description": "SSL mode for PostgreSQL (disable, require, etc.)",
			},
			"params": map[string]any{
				"type":        "string",
				"description": "Additional connection parameters (comma-separated key=value, e.g., 'useUnicode=true,characterEncoding=utf-8')",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "SQL query to execute (SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, etc.)",
			},
		},
		"required": []string{"type", "host", "user", "query"},
	}
}

// Execute runs a database query with the given parameters
func (t *DatabaseTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// Extract connection parameters
	dbType, _ := args["type"].(string)
	host, _ := args["host"].(string)
	port, _ := args["port"].(float64) // JSON numbers come as float64
	user, _ := args["user"].(string)
	password, _ := args["password"].(string)
	database, _ := args["database"].(string)
	serviceName, _ := args["service_name"].(string)
	sslMode, _ := args["ssl_mode"].(string)
	params, _ := args["params"].(string)
	query, _ := args["query"].(string)

	// Validate required parameters
	if dbType == "" {
		return ErrorResult("type is required (mysql, postgresql, oracle)")
	}
	if host == "" {
		return ErrorResult("host is required")
	}
	if user == "" {
		return ErrorResult("user is required")
	}
	if query == "" {
		return ErrorResult("query is required")
	}

	// Set default port based on type
	if port == 0 {
		switch dbType {
		case "mysql":
			port = 3306
		case "postgresql":
			port = 5432
		case "oracle":
			port = 1521
		}
	}

	// Build connection string
	connStr, err := buildConnectionString(dbType, host, int(port), user, password, database, serviceName, sslMode, params)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to build connection string: %v", err))
	}

	// Determine driver name
	driverName := dbType
	if dbType == "oracle" {
		driverName = "oracle"
	}

	// Open connection
	db, err := sql.Open(driverName, connStr)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to connect to database: %v", err))
	}
	defer db.Close()

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		return ErrorResult(fmt.Sprintf("failed to ping database: %v", err))
	}

	// Determine if it's a query or statement
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	isSelect := strings.HasPrefix(queryUpper, "SELECT") ||
		strings.HasPrefix(queryUpper, "SHOW") ||
		strings.HasPrefix(queryUpper, "DESCRIBE") ||
		strings.HasPrefix(queryUpper, "EXPLAIN")

	if isSelect {
		return executeQuery(ctx, db, query)
	} else {
		return executeStatement(ctx, db, query)
	}
}

// buildConnectionString builds the appropriate connection string for the database type
func buildConnectionString(dbType, host string, port int, user, password, database, serviceName, sslMode, params string) (string, error) {
	switch dbType {
	case "mysql":
		return buildMySQLConnectionString(host, port, user, password, database, params)
	case "postgresql":
		return buildPostgresConnectionString(host, port, user, password, database, sslMode, params)
	case "oracle":
		return buildOracleConnectionString(host, port, user, password, serviceName)
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// buildMySQLConnectionString builds MySQL connection string
func buildMySQLConnectionString(host string, port int, user, password, database, params string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		user, password, host, port, database))

	// Build params
	queryParams := []string{"parseTime=true"}

	// Add custom params
	if params != "" {
		customParams := strings.Split(params, ",")
		for _, p := range customParams {
			p = strings.TrimSpace(p)
			if p != "" {
				queryParams = append(queryParams, p)
			}
		}
	}

	sb.WriteString("?" + strings.Join(queryParams, "&"))
	return sb.String(), nil
}

// buildPostgresConnectionString builds PostgreSQL connection string
func buildPostgresConnectionString(host string, port int, user, password, database, sslMode, params string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, database, sslMode))

	// Add custom params
	if params != "" {
		customParams := strings.Split(params, ",")
		for _, p := range customParams {
			p = strings.TrimSpace(p)
			if p != "" {
				sb.WriteString(" ")
				sb.WriteString(p)
			}
		}
	}

	return sb.String(), nil
}

// buildOracleConnectionString builds Oracle connection string
func buildOracleConnectionString(host string, port int, user, password, serviceName string) (string, error) {
	return fmt.Sprintf("oracle://%s:%s@%s:%d/%s", user, password, host, port, serviceName), nil
}

// executeQuery executes a SELECT query and returns results
func executeQuery(ctx context.Context, db *sql.DB, query string) *ToolResult {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return ErrorResult(fmt.Sprintf("query failed: %v", err))
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get columns: %v", err))
	}

	// Create result
	result := make([][]any, 0)

	// Scan rows
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return ErrorResult(fmt.Sprintf("failed to scan row: %v", err))
		}

		row := make([]any, len(columns))
		for i, v := range values {
			if v != nil {
				row[i] = convertDBValue(v)
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return ErrorResult(fmt.Sprintf("error iterating rows: %v", err))
	}

	// Format as JSON
	jsonRes, err := json.MarshalIndent(map[string]any{
		"columns":   columns,
		"rows":      result,
		"row_count": len(result),
	}, "", "  ")
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to marshal result: %v", err))
	}

	return &ToolResult{
		ForLLM: string(jsonRes),
	}
}

// executeStatement executes INSERT, UPDATE, DELETE and returns affected rows
func executeStatement(ctx context.Context, db *sql.DB, stmt string) *ToolResult {
	result, err := db.ExecContext(ctx, stmt)
	if err != nil {
		return ErrorResult(fmt.Sprintf("statement failed: %v", err))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get affected rows: %v", err))
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("Statement executed successfully. Affected rows: %d", rowsAffected),
	}
}

// convertDBValue converts database types to JSON-friendly types
func convertDBValue(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}
