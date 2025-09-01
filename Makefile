build-css:
	npx @tailwindcss/cli -i static/css/input.css -o static/css/styles.css

watch-css:
	npx @tailwindcss/cli -i static/css/input.css -o static/css/styles.css --watch

generate-templ:
	templ generate

watch-templ:
	templ generate --watch
