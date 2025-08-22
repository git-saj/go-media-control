# Go Media Control

A web application built with Go, Templ, Tailwind CSS v4, DaisyUI, and HTMX to browse and control media streams via Discord webhooks. It fetches live streams from an Xtream API, displays them as clickable cards, and sends stream URLs to a Discord channel with a configurable command prefix. The application includes Authentik OIDC authentication for secure access.

## Features

- **Responsive UI**: Card-based layout with search and pagination, optimized for all screen sizes.
- **Discord Integration**: Click a card to send its stream URL to Discord with a custom prefix (e.g., `! <url>`).
- **Authentication**: Secure OIDC authentication using Authentik.
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
   - Copy `env.template` to `.env` and fill in your credentials:
     ```bash
     cp env.template .env
     ```
     Example `.env`:
     ```bash
     # Server configuration
     PORT=8080
     
     # Xtream API configuration
     XTREAM_BASEURL=https://your-xtream-api.com
     XTREAM_USERNAME=your-username
     XTREAM_PASSWORD=your-password
     
     # Discord configuration
     DISCORD_WEBHOOK=https://discord.com/api/webhooks/your-webhook
     COMMAND_PREFIX=!
     
     # Authentik OIDC configuration
     AUTHENTIK_URL=https://your-authentik-instance.com
     AUTHENTIK_CLIENT_ID=your_client_id
     AUTHENTIK_CLIENT_SECRET=your_client_secret
     AUTHENTIK_REDIRECT_URL=http://localhost:8080/auth/callback
     SESSION_SECRET=your-very-secure-random-session-secret-key-here
     ```

4. **Set up Authentik (OIDC Provider)**:
   
   In your Authentik admin interface:
   
   a. **Create OAuth2/OpenID Provider**:
   - Go to **Applications** → **Providers** → **Create**
   - Choose **OAuth2/OpenID Provider**
   - Set **Name**: `go-media-control`
   - Set **Authorization flow**: `default-authorization-flow`
   - Set **Client type**: `Confidential`
   - Set **Redirect URIs**: `http://localhost:8080/auth/callback` (adjust for your domain)
   - Leave other settings as default
   - **Save** and note the **Client ID** and **Client Secret**

   b. **Create Application**:
   - Go to **Applications** → **Applications** → **Create**
   - Set **Name**: `Go Media Control`
   - Set **Slug**: `go-media-control`
   - Set **Provider**: Select the provider you just created
   - Set **Launch URL**: `http://localhost:8080/` (your app URL)
   - **Save**

   c. **Update your `.env` file** with the Client ID and Client Secret from step 4a.

5. **Build and Run**:
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

## Authentication

The application uses Authentik for OIDC authentication. All routes except `/auth/*` require authentication.

### Authentication Flow

1. **Unauthenticated Access**: Users accessing any protected route are redirected to `/auth/login`
2. **Login**: The login handler redirects to your Authentik instance for authentication
3. **Callback**: After successful authentication, Authentik redirects back to `/auth/callback`
4. **Session Creation**: A secure session is created and the user is redirected to the home page
5. **Logout**: Users can logout at `/auth/logout` (local) or `/auth/logout?global=true` (Authentik + local)

### Authentication Endpoints

- `GET /auth/login` - Initiate OIDC login
- `GET /auth/callback` - Handle OIDC callback
- `GET /auth/logout` - Logout (local session only)
- `GET /auth/logout?global=true` - Logout from both app and Authentik
- `GET /auth/user` - Get current user info (JSON, for debugging)

### Environment Variables

The following environment variables are required for authentication:

- `AUTHENTIK_URL` - Your Authentik instance URL (without trailing slash)
- `AUTHENTIK_CLIENT_ID` - OAuth2 Client ID from Authentik
- `AUTHENTIK_CLIENT_SECRET` - OAuth2 Client Secret from Authentik  
- `AUTHENTIK_REDIRECT_URL` - Callback URL (must match Authentik configuration)
- `SESSION_SECRET` - Secure random string for session encryption (32+ characters)

## Usage

- **Authentication**: Navigate to the application URL and you'll be redirected to Authentik for login
- **Browse Channels**: View up to 15 media stream cards (5 columns on large screens, fewer on smaller devices).
- **Search**: Type in the search bar to filter channels dynamically.
- **Send to Discord**: Click a card to send its stream URL to your Discord channel (e.g., `! https://stream-url`).
- **Navigate**: Use Previous/Next buttons for pagination.
- **Logout**: Access `/auth/logout` to logout locally, or `/auth/logout?global=true` to logout from Authentik as well.
