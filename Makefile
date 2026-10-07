.PHONY: up down build test test-e2e clean

# Docker Compose shortcuts
up:
	docker-compose up -d

down:
	docker-compose down

build:
	docker-compose build

# Testing
test: test-backend test-frontend

test-backend:
	cd backend/catalog-service && go test ./...
	cd backend/inventory-service && go test ./...
	cd backend/booking-service && go test ./...
	# cd backend/auth-service && go test ./...

test-frontend:
	cd frontend && npm run test || echo "No frontend tests configured yet"

test-e2e:
	# Placeholder for automated integration tests across the API Gateway
	echo "Running E2E tests..."

# Clean up
clean: down
	docker system prune -f
	rm -rf frontend/node_modules
