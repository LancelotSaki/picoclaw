package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MySQLProvider implements DatabaseProvider for MySQL
type MySQLProvider struct {
	db     *sql.DB
	config *DatabaseConfig
}

// NewMySQLProvider creates a new MySQL database provider
func NewMySQLProvider() *MySQLProvider {
	return &MySQLProvider{}
}

// Connect establishes a connection to MySQL database
func (p *MySQLProvider) Connect(ctx context.Context, config *DatabaseConfig) error {
	dsn := BuildMySQLConnectionString(config)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	p.db = db
	p.config = config
	return nil
}

// Close closes the MySQL database connection
func (p *MySQLProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// IsConnected returns true if the connection is active
func (p *MySQLProvider) IsConnected() bool {
	return p.db != nil && p.db.Ping() == nil
}

// ExecuteQuery executes a SELECT query and returns results
func (p *MySQLProvider) ExecuteQuery(ctx context.Context, query string, params ...any) (*QueryResult, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return Query(p.db, query, params...)
}

// ExecuteStatement executes INSERT, UPDATE, DELETE and returns affected rows
func (p *MySQLProvider) ExecuteStatement(ctx context.Context, stmt string, params ...any) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("not connected to database")
	}
	return ExecStatement(p.db, stmt, params...)
}

// GetTables returns list of all tables in the database
func (p *MySQLProvider) GetTables(ctx context.Context) ([]TableInfo, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return GetTableList(ctx, p.db, true)
}

// GetColumns returns column information for a specific table
func (p *MySQLProvider) GetColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return GetColumnInfo(ctx, p.db, table, true)
}

// Ping checks if the database connection is alive
func (p *MySQLProvider) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("not connected to database")
	}
	return p.db.PingContext(ctx)
}

// MySQLServer represents a MySQL MCP server
type MySQLServer struct {
	provider *MySQLProvider
}

// NewMySQLServer creates a new MySQL MCP server
func NewMySQLServer() *MySQLServer {
	return &MySQLServer{
		provider: NewMySQLProvider(),
	}
}

// RunMySQLServer runs the MySQL MCP server
func RunMySQLServer() error {
	config := parseServerConfig()

	server := NewMySQLServer()

	// Connect to database
	ctx := context.Background()
	if err := server.provider.Connect(ctx, &DatabaseConfig{
		Type:     MySQL,
		Host:     config.Host,
		Port:     config.Port,
		User:     config.User,
		Password: config.Password,
		Database: config.Database,
		SSLMode:  config.SSLMode,
		Params:   config.Params,
	}); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer server.provider.Close()

	// Create MCP server
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "picoclaw-database-mysql",
		Version: "1.0.0",
	}, nil)

	// Register tools using the generic AddTool function
	// Execute Query tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "execute_query",
		Description: "Execute a SELECT query and return results. Use this for SELECT, SHOW, DESCRIBE, and other read-only operations.",
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
		Description: "List all tables in the current database",
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
