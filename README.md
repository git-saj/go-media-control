# Go Media Control

A web application built with Go, Templ, Tailwind CSS v4, DaisyUI, and HTMX to browse and control media streams via Discord webhooks. It fetches live streams from an Xtream API, displays them as clickable cards, and sends stream URLs to a Discord channel with a configurable command prefix.

## Features

- **Responsive UI**: Card-based layout with search and pagination, optimized for all screen sizes.
- **Discord Integration**: Click a card to send its stream URL to Discord with a custom prefix (e.g., `! <url>`).
- **Lightweight**: Built with a minimal Alpine-based Docker image.

## Project Structure

```
├── cmd/                # Main application entry point
├── handlers/           # HTTP handlers
├── internal/           # Private packages (cache, config, discord, xtream)
├── static/             # CSS, JS, and image assets
│   ├── css/
│   │   ├── input.css   # Source CSS for Tailwind
│   │   └── styles.css  # Generated CSS (ignored in Git)
│   └── js/             # HTMX and form-json scripts
├── templates/          # Templ files for server-side rendering
├── Dockerfile          # Multi-stage build for production
├── Makefile            # Build and watch commands
├── package.json        # Node.js dependencies (Tailwind, DaisyUI)
└── .env                # Environment variables
```

## Prerequisites

- [Go](https://golang.org/dl/) (v1.23+)
- [Node.js](https://nodejs.org/) (v20+)
- [Docker](https://www.docker.com/get-started) (optional, for containerized deployment)
- An Xtream API account (base URL, username, password)
- A Discord webhook URL

## Setup

### Local Development

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/git-saj/go-media-control.git
   cd go-media-control
   ```

2. **Install Dependencies**:
   - Go modules:
     ```bash
     go mod download
     ```
   - Node.js packages:
     ```bash
     npm install
     ```

3. **Configure Environment**:
   - Copy `.env.example` to `.env` and fill in your credentials:
     ```bash
     cp .env.example .env
     ```
     Example `.env`:
     ```bash
     PORT=8080
     XTREAM_BASEURL=https://your-xtream-api.com
     XTREAM_USERNAME=your-username
     XTREAM_PASSWORD=your-password
     DISCORD_WEBHOOK=https://discord.com/api/webhooks/your-webhook
     COMMAND_PREFIX=!
     ```

4. **Build and Run**:
   - Generate Templ files and CSS:
     ```bash
     make generate-templ
     make build-css
     ```
   - Run with live reloading:
     ```bash
     make watch-templ & make watch-css & air
     ```
   - Open `http://localhost:8080/` in your browser.

### Docker Deployment

1. **Build the Docker Image**:
   ```bash
   docker build -t go-media-control:latest .
   ```

2. **Run the Container**:
   ```bash
   docker run -p 8080:8080 --env-file .env go-media-control:latest
   ```
   - Open `http://localhost:8080/`.

## Usage

- **Browse Channels**: View up to 15 media stream cards (5 columns on large screens, fewer on smaller devices).
- **Search**: Type in the search bar to filter channels dynamically.
- **Send to Discord**: Click a card to send its stream URL to your Discord channel (e.g., `! https://stream-url`).
- **Navigate**: Use Previous/Next buttons for pagination.
