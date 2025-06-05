cd sql/schema
echo "Running schema.sql"
goose postgres "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable" down
goose postgres "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable" up
