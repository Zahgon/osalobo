# Gin Boilerplate

A production-ready boilerplate for building REST APIs with Go and Echo framework. This boilerplate includes essential features like database integration, API documentation, logging, error handling, and containerization support.

## 🚀 Features

- [x] [Echo Framework](https://github.com/labstack/echo) for routing and middleware
- [x] PostgreSQL integration with migration support
- [x] Swagger API documentation
- [x] API monitoring with APIToolkit
- [x] Custom error handling and logging
- [x] CORS configuration
- [x] Docker and Docker Compose support
- [x] OTP management system
- [x] Static file serving
- [x] Environment configuration
- [x] Hot reload during development
- [x] Code security scanning with gosec
- [x] Event streaming with Apache Kafka
- [x] Transactional message processing
- [x] Consumer group management
- [x] Event broadcasting system

## 📋 Prerequisites

- Go 1.x
- Docker and Docker Compose
- PostgreSQL (if running locally)
- Make

## 🛠️ Installation

1. Clone the repository
```bash
git clone https://github.com/CeoFred/gin-boilerplate.git
cd gin-boilerplate
```

2. Copy the example environment file
```bash
cp .env.example .env
```

3. Install dependencies
```bash
make requirements
```

## 🔧 Configuration

Update the `.env` file with your configuration:

```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=yourdatabase
API_TOOLKIT_KEY=your-api-toolkit-key
```

## 🚀 Running the Application

### Local Development

```bash
# Run the application with hot reload
make run-local
```

### Using Docker

```bash
# Build the Docker image
make build

# Start all services using Docker Compose
make service-start
```

## 📚 API Documentation

Swagger documentation is available at:
```
http://localhost:8080/swagger/index.html
```

To regenerate Swagger documentation:
```bash
make docs-generate
```

## 🛠️ Available Make Commands

- `make run-local` - Run the application locally with hot reload
- `make docs-generate` - Generate Swagger documentation
- `make requirements` - Install/update dependencies
- `make clean-packages` - Clean Go module cache
- `make build` - Build Docker image
- `make start-postgres` - Start PostgreSQL container
- `make stop-postgres` - Stop PostgreSQL container
- `make start` - Start application with Docker
- `make build-no-cache` - Build Docker image without cache
- `make service-stop` - Stop all Docker Compose services
- `make service-start` - Start all Docker Compose services

## 📁 Project Structure

```

.
├── constants/           # Application constants and configuration
├── database/           # Database connection and migrations
├── docs/              # Swagger documentation
├── internal/
│   ├── bootstrap/     # Application bootstrapping
│   ├── helpers/       # Helper functions
│   ├── otp/          # OTP management
│   ├── repository/    # Repository management
│   ├── routes/        # API routes
│   └── streaming/     # Kafka streaming implementation
│       ├── consumer.go  # Kafka consumer implementation
│       ├── producer.go  # Kafka producer implementation
│       └── events.go    # Event type definitions
├── static/            # Static files
├── templates/         # Template files
├── main.go           # Application entry point
├── Dockerfile        # Docker configuration
├── docker-compose.yml # Docker Compose configuration
└── Makefile          # Build and development commands
```

## 🔒 Security

The project includes security measures:
- Custom recovery middleware
- CORS configuration
- Request logging
- Security scanning with gosec

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

## 👤 Contact

Johnson Awah Alfred - [johnsonmessilo19@gmail.com](mailto:johnsonmessilo19@gmail.com)

## ⭐️ Show your support

Give a ⭐️ if this project helped you!
