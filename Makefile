build:
	mkdir -p out/web
	mkdir -p out/static
	mkdir -p cmd/go-media-control/static
	cp -r static/* cmd/go-media-control/static/
	GOARCH=wasm GOOS=js go build -o out/web/app.wasm ./cmd/go-media-control
	go build -o out/go-media-control ./cmd/go-media-control
	cp -r static/* out/static/

run: build
	./out/go-media-control
