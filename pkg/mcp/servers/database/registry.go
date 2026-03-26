package database

import (
	"context"
	"fmt"
)

// DatabaseProviderFactory is a function that creates a new DatabaseProvider
type DatabaseProviderFactory func() DatabaseProvider

// DatabaseProviderRegistry is a global registry of database providers
var DatabaseProviderRegistry = make(map[DatabaseType]DatabaseProviderFactory)

// RegisterProvider registers a database provider factory
func RegisterProvider(dbType DatabaseType, factory DatabaseProviderFactory) {
	DatabaseProviderRegistry[dbType] = factory
}

// GetProvider returns a new provider for the given database type
func GetProvider(dbType DatabaseType) (DatabaseProvider, error) {
	factory, ok := DatabaseProviderRegistry[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	return factory(), nil
}

// IsSupported returns true if the given database type is supported
func IsSupported(dbType DatabaseType) bool {
	_, ok := DatabaseProviderRegistry[dbType]
	return ok
}

// GetSupportedTypes returns a list of all supported database types
func GetSupportedTypes() []DatabaseType {
	types := make([]DatabaseType, 0, len(DatabaseProviderRegistry))
	for dbType := range DatabaseProviderRegistry {
		types = append(types, dbType)
	}
	return types
}

func init() {
	// Register built-in providers
	RegisterProvider(MySQL, func() DatabaseProvider {
		return NewMySQLProvider()
	})
	RegisterProvider(PostgreSQL, func() DatabaseProvider {
		return NewPostgreSQLProvider()
	})
	RegisterProvider(Oracle, func() DatabaseProvider {
		return NewOracleProvider()
	})
	// Note: Kingbase, Dameng, and OpenGauss can be added later
	// They typically use PostgreSQL-compatible protocols
}

// CreateProvider creates a new provider instance for the given database type
func CreateProvider(dbType DatabaseType) (DatabaseProvider, error) {
	provider, err := GetProvider(dbType)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// ConnectProvider creates a provider and connects to the database
func ConnectProvider(ctx context.Context, dbType DatabaseType, config *DatabaseConfig) (DatabaseProvider, error) {
	provider, err := CreateProvider(dbType)
	if err != nil {
		return nil, err
	}

	if err := provider.Connect(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", dbType, err)
	}

	return provider, nil
}
