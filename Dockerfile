# Stage 1: Build Tailwind CSS
FROM node:20 AS css-builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install
COPY static/css/input.css ./static/css/
RUN npx @tailwindcss/cli -i static/css/input.css -o static/css/styles.css --minify

# Stage 2: Build Go binary with Templ
FROM golang:1.23 AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=css-builder /app/static/css/styles.css ./static/css/styles.css
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-media-control ./cmd/go-media-control

# Stage 3: Final runtime image
FROM alpine:3.19
WORKDIR /app
COPY --from=go-builder /go-media-control .
COPY static ./static
EXPOSE 8080
CMD ["./go-media-control"]
