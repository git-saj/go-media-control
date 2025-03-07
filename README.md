# go-media-control

`go-media-control` is a Go-based web application built with the `go-app` framework (v10) and styled with Bulma CSS. It fetches media streams from either an `.m3u` playlist file or the Xtreams API, displays them as interactive cards, and allows users to filter them via a search bar. When a media card is clicked, the app sends the media details to a configurable Discord webhook. The project emphasizes modularity, asynchronous operations, and observability-ready design, making it a solid foundation for further development.

## Features
- **Dual Media Sources**: Supports fetching media from `.m3u` files or the Xtreams API, configurable via environment variables.
- **Interactive UI**: Displays all media as cards with names and logos, filterable with a top search bar.
- **Discord Integration**: Sends selected media details to a Discord webhook with retry logic.
- **Responsive Design**: Built with Bulma CSS for a clean, mobile-friendly interface.
- **Best Practices**: Includes structured logging (`zap`), unit tests, asynchronous fetching, HTTP timeouts, and graceful shutdown.

## Project Structure
```
go-media-control/
├── cmd/
│   └── go-media-control/
│       └── main.go      # Entry point and server setup
├── internal/
│   ├── app/            # UI logic with go-app
│   │   └── app.go
│   ├── config/         # Configuration management
│   │   └── config.go
│   ├── discord/        # Discord webhook logic
│   │   └── discord.go
│   └── media/          # Media fetching logic
│       ├── media.go
│       └── media_test.go # Unit tests
├── static/             # Static assets (CSS, images)
│   └── styles.css      # Stylesheet for the application
├── Dockerfile          # Docker configuration
├── Makefile            # Build automation
├── go.mod              # Go module file
├── go.sum              # Dependency checksums
└── README.md           # This file
```

## Prerequisites
- **Go**: Version 1.24.1 or later.
- **Git**: For cloning the repository.
- **Environment Variables**: Required for configuration (see below).

## Installation

### Option 1: Using Docker
1. **Pull the Docker image**:
    ```bash
    docker pull ghcr.io/git-saj/go-media-control:latest
    ```

2. **Run the container**:
    ```bash
    docker run -p 8080:8080 \
      -e APP_MEDIA_SOURCE=m3u \
      -e APP_M3U_URL=http://example.com/playlist.m3u \
      -e APP_DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/your-id/your-token \
      ghcr.io/git-saj/go-media-control:latest
    ```

### Option 2: Building from Source
1. **Clone the Repository**:
    ```bash
    git clone https://github.com/git-saj/go-media-control.git
    cd go-media-control
    ```

2. **Install Dependencies**:
    ```bash
    go mod tidy
    ```
    This ensures all dependencies listed in `go.mod` (e.g., `go-app/v10`, `viper`, `zap`) are downloaded and consistent.

3. **Build and Run**:
    ```bash
    make run
    ```
    This will build both the WebAssembly client and Go server, then start the application.

## Configuration
`go-media-control` uses environment variables for configuration, managed by `viper`. Set these variables before running the app:

### Required Variables
- `APP_DISCORD_WEBHOOK_URL`: The Discord webhook URL to send media details (e.g., `https://discord.com/api/webhooks/your-id/your-token`).
- `APP_MEDIA_SOURCE`: The media source to use (`m3u` or `xtreams`).

### Source-Specific Variables
- For `.m3u`:
  - `APP_M3U_URL`: URL to the `.m3u` playlist file (e.g., `http://example.com/playlist.m3u`).
- For Xtreams API:
  - `APP_XTREAMS_BASE_URL`: Base URL of the Xtreams API (e.g., `http://xtreams-api.com`).
  - `APP_XTREAMS_USERNAME`: Your Xtreams API username.
  - `APP_XTREAMS_PASSWORD`: Your Xtreams API password.

### Optional Variable
- `APP_PORT`: The port to run the server on (defaults to `8080`).

### Example Configuration
For `.m3u`:
  ```bash
  export APP_M3U_URL="http://example.com/playlist.m3u"
  export APP_DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/your-id/your-token"
  export APP_MEDIA_SOURCE="m3u"
  export APP_PORT="8080"
  ```

For Xtreams:
  ```bash
  export APP_XTREAMS_BASE_URL="http://xtreams-api.com"
  export APP_XTREAMS_USERNAME="your-username"
  export APP_XTREAMS_PASSWORD="your-password"
  export APP_DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/your-id/your-token"
  export APP_MEDIA_SOURCE="xtreams"
  export APP_PORT="8080"
  ```

Alternatively, use a `.env` file and source it:
    // bash
    source .env

## Usage

### Running with Make
The easiest way to build and run the application is using the Makefile:

```bash
# Build the application (both server and WASM)
make build

# Build and run
make run
```

### Docker Commands
If using Docker:

```bash
# Build the Docker image
docker build -t go-media-control .

# Run the container
docker run -p 8080:8080 \
  -e APP_MEDIA_SOURCE=m3u \
  -e APP_M3U_URL=http://example.com/playlist.m3u \
  -e APP_DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/your-id/your-token \
  go-media-control
```

### Interacting with the App
1. Open your browser to `http://localhost:8080` (or your configured port).
2. View all media cards fetched from the configured source.
3. Use the search bar to filter media by name.
4. Click a card to send its details to the Discord webhook.
5. Click "Refresh" to reload the media list.

### Running Tests
```bash
# Run all tests
go test -v ./...

# Run media package tests only
go test -v ./internal/media

# Run with race detection
go test -race -v ./...
```

## Development Notes
- **Logging**: Uses `zap` (v1.27.0) for structured logging, viewable in the console or configurable for file output.
- **Error Handling**: Errors are displayed in the UI and logged for debugging.
- **Asynchronous Fetching**: Media is fetched asynchronously using `go-app/v10` to keep the UI responsive.
- **Timeouts**: HTTP requests (media fetching, Discord webhook) have a 10-second timeout.
- **Retries**: Discord webhook requests retry up to 3 times with simple backoff.
- **Caching**: Media data is cached for 30 minutes to reduce API load, with a force-refresh option.
- **Dark Mode**: The UI is designed with a dark theme for better viewing experience.
- **Containerization**: Docker image uses a multi-stage build to keep the final image small and secure.
- **Static Files**: Both embedded static files (for development) and external files (for production) are supported.

## Potential Improvements
- **Testing**: Expand unit tests for `app`, `config`, and `discord` packages.
- **Security**: Use a secrets manager (e.g., HashiCorp Vault) for sensitive data in production.
- **Monitoring**: Integrate with Prometheus, Tempo, and Loki for full observability (planned for later).
- **Features**: Add pagination, media categories, or playback previews.

## Contributing
Feel free to fork this project, submit pull requests, or open issues for bugs and feature requests. Contributions are welcome!

## License
This project is unlicensed by default. Add a `LICENSE` file if you wish to specify terms (e.g., MIT, Apache 2.0).

## Acknowledgments
- Built with [go-app v10](https://github.com/maxence-charriere/go-app) for WebAssembly magic.
- Styled with [Bulma CSS](https://bulma.io/) for a sleek UI.
- Configured with [Viper v1.19.0](https://github.com/spf13/viper) and logged with [Zap v1.27.0](https://github.com/uber-go/zap).
