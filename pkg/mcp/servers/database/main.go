// Package database provides MCP server implementations for various databases.
// This is the main entry point for running the database MCP servers.
//
// Usage:
//
//	# Run MySQL server
//	go run . --type mysql --host localhost --port 3306 --user root --password secret --database testdb
//
//	# Run PostgreSQL server
//	go run . --type postgresql --host localhost --port 5432 --user postgres --password secret --database testdb
//
//	# Run Oracle server
//	go run . --type oracle --host localhost --port 1521 --user system --password secret --service-name ORCL
package database

import (
	"flag"
	"fmt"
	"os"
)

// parseServerConfig parses command line flags and returns ServerConfig
func parseServerConfig() ServerConfig {
	dbType := flag.String("type", "mysql", "Database type: mysql, postgresql, oracle")
	host := flag.String("host", "localhost", "Database host")
	port := flag.Int("port", 0, "Database port (default varies by type)")
	user := flag.String("user", "root", "Database user")
	password := flag.String("password", "", "Database password")
	database := flag.String("database", "", "Database name")
	sslMode := flag.String("ssl-mode", "disable", "SSL mode (for PostgreSQL/MySQL)")
	serviceName := flag.String("service-name", "", "Service name (for Oracle)")
	// Additional connection parameters (can be specified multiple times)
	// Example: --params "useUnicode=true" --params "characterEncoding=utf-8"
	params := flag.String("params", "", "Additional connection parameters (comma-separated key=value pairs)")

	flag.Parse()

	// Set default port based on database type
	if *port == 0 {
		switch *dbType {
		case "mysql":
			*port = 3306
		case "postgresql":
			*port = 5432
		case "oracle":
			*port = 1521
		}
	}

	return ServerConfig{
		DatabaseType: DatabaseType(*dbType),
		Host:         *host,
		Port:         *port,
		User:         *user,
		Password:     *password,
		Database:     *database,
		SSLMode:      *sslMode,
		ServiceName:  *serviceName,
		Params:       *params,
	}
}

// RunMain is the main entry point for the database MCP server
func RunMain() error {
	config := parseServerConfig()

	switch config.DatabaseType {
	case MySQL:
		return RunMySQLServer()
	case PostgreSQL:
		return RunPostgreSQLServer()
	case Oracle:
		return RunOracleServer()
	default:
		return fmt.Errorf("unsupported database type: %s. Supported types: mysql, postgresql, oracle", config.DatabaseType)
	}
}

// ConnectionStringBuilder builds a connection string from ServerConfig
func (s *ServerConfig) ConnectionString() string {
	switch s.DatabaseType {
	case MySQL:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", s.User, s.Password, s.Host, s.Port, s.Database)
	case PostgreSQL:
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", s.Host, s.Port, s.User, s.Password, s.Database, s.SSLMode)
	case Oracle:
		// Build Oracle TNS format connection string
		return fmt.Sprintf("oracle://%s:%s@%s:%d/%s", s.User, s.Password, s.Host, s.Port, s.ServiceName)
	default:
		return ""
	}
}

// Main is the main function for running as standalone executable
func Main() {
	if err := RunMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
