.PHONY: init
init:
	docker-compose up -d

.PHONY: psql
psql:
	docker-compose exec postgres bash -c 'psql postgresql://$$POSTGRES_USER:$$POSTGRES_PASSWORD@postgres:5432/$$POSTGRES_DB'
