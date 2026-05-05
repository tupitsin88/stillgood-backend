.PHONY: migrate-up seed-demo

migrate-up:
	docker compose build app
	docker compose run --rm --no-deps app go run ./cmd/migrate

seed-demo:
	docker compose build app
	docker compose run --rm -e ALLOW_DEMO_SEED=true app go run ./cmd/seed
