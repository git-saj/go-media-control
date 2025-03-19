build-css:
	tailwindcss -i static/css/input.css -o static/css/styles.css

watch-css:
	tailwindcss -i static/css/input.css -o static/css/styles.css --watch

generate-templ:
	templ generate

watch-templ:
	templ generate --watch
