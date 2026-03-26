package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// mysqlDriverName is the driver name for MySQL
const mysqlDriverName = "mysql"

// postgresDriverName is the driver name for PostgreSQL
const postgresDriverName = "pq"

// BuildMySQLConnectionString builds a MySQL connection string from config
func BuildMySQLConnectionString(config *DatabaseConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database))

	// Build params string
	params := make([]string, 0)

	// Always add parseTime for time parsing
	params = append(params, "parseTime=true")

	// Add SSL parameters if specified
	if config.SSLMode != "" {
		if config.SSLMode == "disable" {
			params = append(params, "tls=false")
		} else {
			params = append(params, "tls=true")
		}
	}

	// Add custom params if specified (comma-separated key=value)
	if config.Params != "" {
		customParams := strings.Split(config.Params, ",")
		for _, p := range customParams {
			p = strings.TrimSpace(p)
			if p != "" {
				params = append(params, p)
			}
		}
	}

	if len(params) > 0 {
		sb.WriteString("?" + strings.Join(params, "&"))
	}

	return sb.String()
}

// BuildPostgresConnectionString builds a PostgreSQL connection string from config
func BuildPostgresConnectionString(config *DatabaseConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		config.SSLMode))
	return sb.String()
}

// RowsToResult converts sql.Rows to QueryResult
func RowsToResult(rows *sql.Rows) (*QueryResult, error) {
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Get column types
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	// Create result
	result := &QueryResult{
		Columns: columns,
		Rows:    make([][]any, 0),
	}

	// Scan rows
	for rows.Next() {
		// Create slice of interface{} for scanning
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert to JSON-friendly format
		row := make([]any, len(columns))
		for i, v := range values {
			if v != nil {
				// Get the scan type for this column
				if scanType := columnTypes[i].ScanType(); scanType != nil {
					row[i] = convertValue(v, columnTypes[i].DatabaseTypeName())
				} else {
					row[i] = v
				}
			}
		}
		result.Rows = append(result.Rows, row)
	}

	result.RowCount = len(result.Rows)

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// convertValue converts database types to JSON-friendly types
func convertValue(value any, dbType string) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}

// ExecStatement executes a non-query statement and returns affected rows
func ExecStatement(db *sql.DB, stmt string, params ...any) (int64, error) {
	result, err := db.Exec(stmt, params...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute statement: %w", err)
	}
	return result.RowsAffected()
}

// Query executes a query and returns results
func Query(db *sql.DB, query string, params ...any) (*QueryResult, error) {
	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return RowsToResult(rows)
}

// GetColumnInfo retrieves column information for a table
func GetColumnInfo(ctx context.Context, db *sql.DB, tableName string, isMySQL bool) ([]ColumnInfo, error) {
	var query string
	if isMySQL {
		query = fmt.Sprintf("SHOW FULL COLUMNS FROM `%s`", tableName)
	} else {
		query = fmt.Sprintf(`
			SELECT
				c.column_name,
				c.data_type,
				c.is_nullable,
				c.column_default,
				c.column_key,
				c.extra,
				c.column_comment
			FROM information_schema.columns c
			WHERE c.table_name = $1 AND c.table_schema = current_database()
			ORDER BY c.ordinal_position
		`, tableName)
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		if isMySQL {
			err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &col.Default, &col.KeyType, &col.Extra, &col.Comment)
			if err != nil {
				return nil, fmt.Errorf("failed to scan column: %w", err)
			}
		} else {
			err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &col.Default, &col.KeyType, &col.Extra, &col.Comment)
			if err != nil {
				return nil, fmt.Errorf("failed to scan column: %w", err)
			}
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// GetTableList retrieves list of tables from the database
func GetTableList(ctx context.Context, db *sql.DB, isMySQL bool) ([]TableInfo, error) {
	var query string
	if isMySQL {
		query = `
			SELECT
				table_name,
				table_type,
				table_comment
			FROM information_schema.tables
			WHERE table_schema = DATABASE()
			ORDER BY table_name
		`
	} else {
		query = `
			SELECT
				t.table_name,
				t.table_type,
				obj_description(t.table_name::regclass) as table_comment
			FROM information_schema.tables t
			WHERE t.table_schema = current_database()
			AND t.table_type IN ('BASE TABLE', 'VIEW')
			ORDER BY t.table_name
		`
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		err := rows.Scan(&table.Name, &table.Type, &table.Comment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}
		tables = append(tables, table)
	}

	return tables, rows.Err()
}
