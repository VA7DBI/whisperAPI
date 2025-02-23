# Middleware Package

Package middleware provides HTTP middleware components for the Whisper API service.

## Components

### Authentication Middleware
- Token validation with Redis caching
- PostgreSQL token storage
- Fallback to static tokens
- Support for multiple authentication methods

## Authentication Flow

1. Check for Authorization header
2. Extract token from header
3. Validate token:
   - Check Redis cache
   - Query PostgreSQL if not in cache
   - Check static tokens if not in database
4. Cache valid tokens in Redis
5. Return 401 if token is invalid

## Usage Example

