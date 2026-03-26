package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	_ "github.com/sijms/go-ora/v2"
)

// OracleProvider implements DatabaseProvider for Oracle
type OracleProvider struct {
	db     *sql.DB
	config *DatabaseConfig
}

// NewOracleProvider creates a new Oracle database provider
func NewOracleProvider() *OracleProvider {
	return &OracleProvider{}
}

// BuildOracleConnectionString builds an Oracle connection string from config
func BuildOracleConnectionString(config *DatabaseConfig) string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}

	// Build TNS format connection string
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
	))

	if config.ServiceName != "" {
		sb.WriteString(config.ServiceName)
	} else if config.Database != "" {
		sb.WriteString(config.Database)
	}

	return sb.String()
}

// Connect establishes a connection to Oracle database
func (p *OracleProvider) Connect(ctx context.Context, config *DatabaseConfig) error {
	dsn := BuildOracleConnectionString(config)
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to Oracle: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping Oracle: %w", err)
	}

	p.db = db
	p.config = config
	return nil
}

// Close closes the Oracle database connection
func (p *OracleProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// IsConnected returns true if the connection is active
func (p *OracleProvider) IsConnected() bool {
	return p.db != nil && p.db.Ping() == nil
}

// ExecuteQuery executes a SELECT query and returns results
func (p *OracleProvider) ExecuteQuery(ctx context.Context, query string, params ...any) (*QueryResult, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return QueryOracle(p.db, query, params...)
}

// ExecuteStatement executes INSERT, UPDATE, DELETE and returns affected rows
func (p *OracleProvider) ExecuteStatement(ctx context.Context, stmt string, params ...any) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("not connected to database")
	}
	return ExecStatementOracle(p.db, stmt, params...)
}

// GetTables returns list of all tables in the database
func (p *OracleProvider) GetTables(ctx context.Context) ([]TableInfo, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return GetTableListOracle(ctx, p.db)
}

// GetColumns returns column information for a specific table
func (p *OracleProvider) GetColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return GetColumnInfoOracle(ctx, p.db, table)
}

// Ping checks if the database connection is alive
func (p *OracleProvider) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("not connected to database")
	}
	return p.db.PingContext(ctx)
}

// QueryOracle executes a query for Oracle database
func QueryOracle(db *sql.DB, query string, params ...any) (*QueryResult, error) {
	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
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
				row[i] = convertOracleValue(v)
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

// convertOracleValue converts Oracle database types to JSON-friendly types
func convertOracleValue(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}

// ExecStatementOracle executes a statement for Oracle database
func ExecStatementOracle(db *sql.DB, stmt string, params ...any) (int64, error) {
	result, err := db.Exec(stmt, params...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute statement: %w", err)
	}
	return result.RowsAffected()
}

// GetTableListOracle retrieves list of tables from Oracle database
func GetTableListOracle(ctx context.Context, db *sql.DB) ([]TableInfo, error) {
	query := `
		SELECT
			table_name,
			table_type
		FROM user_tables
		UNION ALL
		SELECT
			view_name,
			'VIEW'
		FROM user_views
		ORDER BY table_name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		err := rows.Scan(&table.Name, &table.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}
		tables = append(tables, table)
	}

	return tables, rows.Err()
}

// GetColumnInfoOracle retrieves column information from Oracle database
func GetColumnInfoOracle(ctx context.Context, db *sql.DB, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT
			column_name,
			data_type,
			data_length,
			data_precision,
			data_scale,
			nullable,
			data_default,
			column_id
		FROM user_tab_columns
		WHERE table_name = UPPER(:1)
		ORDER BY column_id
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var dataLength, dataPrecision, dataScale sql.NullInt64
		var dataDefault sql.NullString

		err := rows.Scan(&col.Name, &col.Type, &dataLength, &dataPrecision, &dataScale, &col.Nullable, &dataDefault)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		// Build full type string
		if dataPrecision.Valid && dataScale.Valid && dataPrecision.Int64 > 0 {
			col.Type = fmt.Sprintf("%s(%d,%d)", col.Type, dataPrecision.Int64, dataScale.Int64)
		} else if dataLength.Valid {
			col.Type = fmt.Sprintf("%s(%d)", col.Type, dataLength.Int64)
		}

		if dataDefault.Valid {
			col.Default = dataDefault.String
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// OracleServer represents an Oracle MCP server
type OracleServer struct {
	provider *OracleProvider
}

// NewOracleServer creates a new Oracle MCP server
func NewOracleServer() *OracleServer {
	return &OracleServer{
		provider: NewOracleProvider(),
	}
}

// RunOracleServer runs the Oracle MCP server
func RunOracleServer() error {
	config := parseServerConfig()

	server := NewOracleServer()

	// Connect to database
	ctx := context.Background()
	if err := server.provider.Connect(ctx, &DatabaseConfig{
		Type:        Oracle,
		Host:        config.Host,
		Port:        config.Port,
		User:        config.User,
		Password:    config.Password,
		Database:    config.Database,
		ServiceName: config.ServiceName,
		Params:      config.Params,
	}); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer server.provider.Close()

	// Create MCP server
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "picoclaw-database-oracle",
		Version: "1.0.0",
	}, nil)

	// Register tools using the generic AddTool function
	// Execute Query tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "execute_query",
		Description: "Execute a SELECT query and return results. Use this for SELECT and other read-only operations.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The SQL SELECT query to execute",
				},
			},
			"required": []string{"query"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Query string `json:"query"`
	}) (*mcp.CallToolResult, any, error) {
		res, err := server.provider.ExecuteQuery(ctx, args.Query)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
			}, nil, nil
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{&mcp.TextContent{Text: string(jsonRes)}},
		}, nil, nil
	})

	// Execute Statement tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "execute_statement",
		Description: "Execute an INSERT, UPDATE, DELETE, or other data modification statement. Returns the number of affected rows.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"statement": map[string]any{
					"type":        "string",
					"description": "The SQL statement to execute (INSERT, UPDATE, DELETE, CREATE, DROP, etc.)",
				},
			},
			"required": []string{"statement"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Statement string `json:"statement"`
	}) (*mcp.CallToolResult, any, error) {
		affected, err := server.provider.ExecuteStatement(ctx, args.Statement)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Affected rows: %d", affected)}},
		}, nil, nil
	})

	// List Tables tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_tables",
		Description: "List all tables and views in the current schema",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		tables, err := server.provider.GetTables(ctx)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
			}, nil, nil
		}
		jsonRes, _ := json.MarshalIndent(tables, "", "  ")
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{&mcp.TextContent{Text: string(jsonRes)}},
		}, nil, nil
	})

	// Describe Table tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "describe_table",
		Description: "Get detailed information about a table's columns",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table": map[string]any{
					"type":        "string",
					"description": "The name of the table to describe",
				},
			},
			"required": []string{"table"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Table string `json:"table"`
	}) (*mcp.CallToolResult, any, error) {
		columns, err := server.provider.GetColumns(ctx, args.Table)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
			}, nil, nil
		}
		jsonRes, _ := json.MarshalIndent(columns, "", "  ")
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{&mcp.TextContent{Text: string(jsonRes)}},
		}, nil, nil
	})

	// Ping tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "ping",
		Description: "Check if the database connection is alive",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		err := server.provider.Ping(ctx)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: false,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Connection error: %v", err)}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{&mcp.TextContent{Text: "Connection alive"}},
		}, nil, nil
	})

	// Run the server using stdio transport
	return mcpServer.Run(context.Background(), &mcp.StdioTransport{})
}
