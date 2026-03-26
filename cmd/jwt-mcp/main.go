package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sipeed/picoclaw/pkg/mcp/servers/jwt"
)

var (
	secretKey  = flag.String("secret", os.Getenv("JWT_SECRET_KEY"), "JWT secret key")
	issuer     = flag.String("issuer", jwt.DefaultIssuer, "JWT issuer")
	expireHour = flag.Int("expire", 24, "Token expiration in hours")
)

func main() {
	flag.Parse()

	// Create JWT provider
	provider := jwt.NewDefaultJWTProvider()

	// Use provided secret or default
	useSecret := *secretKey
	if useSecret == "" {
		useSecret = jwt.DefaultSecretKey
		fmt.Println("Using default secret key (same as comp-agent)")
	}

	err := provider.Connect(context.Background(), &jwt.JWTConfig{
		SecretKey:  useSecret,
		Issuer:     *issuer,
		ExpireTime: *expireHour,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect JWT provider: %v\n", err)
		os.Exit(1)
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jwt",
		Version: "1.0.0",
	}, nil)

	// Add generate_token tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_token",
		Description: "Generate a JWT token for authentication. Input: account (required), username (optional, defaults to admin). Output: JWT token string that can be used in Authorization header.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Account  string `json:"account"`
		Username string `json:"username"`
	}) (*mcp.CallToolResult, any, error) {

		username := args.Username
		if username == "" {
			username = "admin"
		}

		result, err := provider.Execute(ctx, &jwt.GenerateTokenRequest{
			Account:  args.Account,
			Username: username,
		})
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error generating token: %v", err),
					},
				},
			}, nil, nil
		}

		// Return as JSON for better readability
		resp := map[string]string{
			"token":      result.Token,
			"expires_at": result.ExpiresAt,
		}
		respBytes, _ := json.Marshal(resp)

		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(respBytes),
				},
			},
		}, nil, nil
	})

	// Run server on stdio transport
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Server error: %v\n", err)
		os.Exit(1)
	}
}
