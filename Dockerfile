# Stage 1: Build Tailwind CSS
FROM node:20 AS css-builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install
# Copy template files so Tailwind can scan for classes
COPY templates ./templates
COPY static ./static
RUN npx tailwindcss -i static/css/input.css -o static/css/styles.css --minify

# Stage 2: Build Go binary with Templ
FROM golang:1.23 AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy the properly built CSS from css-builder
COPY --from=css-builder /app/static/css/styles.css ./static/css/styles.css
RUN go install github.com/a-h/templ/cmd/templ@v0.3.865
RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-media-control ./cmd/go-media-control

# Stage 3: Final runtime image
FROM alpine:3.19
WORKDIR /app
COPY --from=go-builder /go-media-control .
# Copy static files from go-builder (which has the properly built CSS)
COPY --from=go-builder /app/static ./static
EXPOSE 8080
CMD ["./go-media-control"]
