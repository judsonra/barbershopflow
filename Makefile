SHELL := /bin/sh
DEV_COMPOSE := docker compose -f compose.yml -f compose.dev.yml
PROD_COMPOSE := docker compose -f compose.yml -f compose.prod.yml

.DEFAULT_GOAL := help
.PHONY: help dev prod down logs ps test test-backend test-frontend build build-backend build-frontend config clean

help: ## Lista os comandos disponíveis
	@awk 'BEGIN {FS = ":.*## "; printf "\nBarberFlow\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Sobe o ambiente de desenvolvimento
	$(DEV_COMPOSE) up --build

prod: ## Sobe o ambiente de produção local
	$(PROD_COMPOSE) up --build -d

down: ## Encerra os ambientes
	$(DEV_COMPOSE) down --remove-orphans
	$(PROD_COMPOSE) down --remove-orphans

logs: ## Acompanha os logs
	docker compose logs -f

ps: ## Exibe o estado dos containers
	docker compose ps

test: test-backend test-frontend ## Executa todos os testes

test-backend: ## Executa testes Go
	cd backend && go test ./...

test-frontend: ## Executa testes do frontend
	cd frontend && npm test

build: build-backend build-frontend ## Gera todos os builds

build-backend: ## Compila a API
	cd backend && go build -o bin/api ./cmd/api

build-frontend: ## Compila a PWA
	cd frontend && npm run build

config: ## Valida e exibe a configuração Docker
	$(DEV_COMPOSE) config --quiet
	$(PROD_COMPOSE) config --quiet

clean: ## Remove containers e volumes (destrutivo)
	@printf "Isso apagará os dados locais. Continuar? [y/N] " && read answer && [ "$$answer" = "y" ]
	$(DEV_COMPOSE) down --volumes --remove-orphans
