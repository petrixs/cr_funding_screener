run: deps
	@if ! docker ps --format '{{.Names}}' | grep -q '^packages-infrastructure-rabbitmq'; then \
		echo 'Запускаю RabbitMQ через docker-compose...'; \
		docker-compose -f packages/infrastructure/docker-compose.yaml up -d rabbitmq; \
	else \
		echo 'RabbitMQ уже запущен'; \
	fi
	docker-compose -f packages/infrastructure/docker-compose.yaml up --build funding-screener 

deps:
	@mkdir -p packages
	@if [ ! -d packages/infrastructure ]; then \
		git clone git@github.com:petrixs/cr-infrastructure.git packages/infrastructure; \
		cd packages/infrastructure && git checkout v1.0.0; \
	else \
		echo 'infrastructure уже существует'; \
	fi
	@if [ ! -d packages/transport-bus ]; then \
		git clone git@github.com:petrixs/cr-transport-bus.git packages/transport-bus; \
		cd packages/transport-bus && git checkout v1.0.1; \
	else \
		echo 'transport-bus уже существует'; \
	fi
	@if [ ! -d packages/exchanges ]; then \
		git clone git@github.com:petrixs/cr-exchanges.git packages/exchanges; \
		cd packages/exchanges && git checkout v1.0.0; \
	else \
		echo 'exchanges уже существует'; \
	fi 