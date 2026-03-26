// Package database provides MCP server implementations for various databases.
// This package implements the Model Context Protocol (MCP) to expose
// database operations as tools that can be used by AI agents.
//
// Supported databases:
//   - MySQL
//   - PostgreSQL
//   - Oracle
//   - And extensible support for other databases (Kingbase, Dameng, openGauss, etc.)
package database

import (
	"context"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	// MySQL represents MySQL database
	MySQL DatabaseType = "mysql"
	// PostgreSQL represents PostgreSQL database
	PostgreSQL DatabaseType = "postgresql"
	// Oracle represents Oracle database
	Oracle DatabaseType = "oracle"
	// Kingbase represents Kingbase (人大金仓) database
	Kingbase DatabaseType = "kingbase"
	// Dameng represents Dameng (达梦) database
	Dameng DatabaseType = "dameng"
	// OpenGauss represents openGauss/PostgreSQL compatible database (磐维)
	OpenGauss DatabaseType = "opengauss"
)

// DatabaseConfig holds the configuration for database connection
type DatabaseConfig struct {
	Type     DatabaseType `json:"type"`
	Host     string       `json:"host"`
	Port     int          `json:"port"`
	User     string       `json:"user"`
	Password string       `json:"password"`
	Database string       `json:"database"`
	SSLMode  string       `json:"ssl_mode,omitempty"`
	// For Oracle
	ServiceName string `json:"service_name,omitempty"`
	// Additional connection parameters (comma-separated key=value pairs)
	Params string `json:"params,omitempty"`
	// Connection string overrides (alternative to individual params)
	ConnectionString string `json:"connection_string,omitempty"`
}

// ColumnInfo represents information about a table column
type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default,omitempty"`
	KeyType    string `json:"key_type,omitempty"` // PRI, UNI, MUL
	Extra      string `json:"extra,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// TableInfo represents information about a database table
type TableInfo struct {
	Name    string      `json:"name"`
	Type    string      `json:"type,omitempty"` // table, view
	Comment string      `json:"comment,omitempty"`
	Columns []ColumnInfo `json:"columns,omitempty"`
}

// QueryResult represents the result of a query execution
type QueryResult struct {
	Columns []string       `json:"columns"`
	Rows    [][]any        `json:"rows"`
	RowCount int           `json:"row_count"`
	AffectedRows int64     `json:"affected_rows,omitempty"`
}

// QueryRequest represents a request to execute a query
type QueryRequest struct {
	Query string `json:"query"`
	// Optional: parameters for prepared statements
	Params []any `json:"params,omitempty"`
}

// StatementRequest represents a request to execute a non-query statement
type StatementRequest struct {
	Statement string `json:"statement"`
	// Optional: parameters for prepared statements
	Params []any `json:"params,omitempty"`
}

// DatabaseProvider defines the interface that all database providers must implement
type DatabaseProvider interface {
	// Connect establishes a connection to the database
	Connect(ctx context.Context, config *DatabaseConfig) error

	// Close closes the database connection
	Close() error

	// IsConnected returns true if the connection is active
	IsConnected() bool

	// ExecuteQuery executes a SELECT query and returns results
	ExecuteQuery(ctx context.Context, query string, params ...any) (*QueryResult, error)

	// ExecuteStatement executes INSERT, UPDATE, DELETE and returns affected rows
	ExecuteStatement(ctx context.Context, stmt string, params ...any) (int64, error)

	// GetTables returns list of all tables in the database
	GetTables(ctx context.Context) ([]TableInfo, error)

	// GetColumns returns column information for a specific table
	GetColumns(ctx context.Context, table string) ([]ColumnInfo, error)

	// Ping checks if the database connection is alive
	Ping(ctx context.Context) error
}

// ServerConfig holds the MCP server configuration
type ServerConfig struct {
	DatabaseType DatabaseType `json:"database_type"`
	Host         string       `json:"host"`
	Port         int          `json:"port"`
	User         string       `json:"user"`
	Password     string       `json:"password"`
	Database     string       `json:"database"`
	SSLMode      string       `json:"ssl_mode,omitempty"`
	ServiceName  string       `json:"service_name,omitempty"`
	Params       string       `json:"params,omitempty"` // Additional connection parameters (comma-separated key=value pairs)
}
