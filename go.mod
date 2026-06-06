module github.com/cleanstartup/way2go

go 1.25.2

require (
	github.com/a-h/templ v0.3.943
	github.com/caarlos0/env/v11 v11.3.1
	github.com/go-chi/chi/v5 v5.2.3
	github.com/joho/godotenv v1.5.1
)

replace github.com/caarlos0/env/v11 => ./deps/env

replace github.com/go-chi/chi/v5 => ./deps/chi

replace github.com/joho/godotenv => ./deps/godotenv
