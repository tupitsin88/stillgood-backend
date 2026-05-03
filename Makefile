.PHONY: seed-demo

seed-demo:
	docker compose build app
	docker compose run --rm -e ALLOW_DEMO_SEED=true app go run ./cmd/seed
