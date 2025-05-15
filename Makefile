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
	else \
		echo 'infrastructure уже существует'; \
	fi
	@if [ ! -d packages/transport-bus ]; then \
		git clone git@github.com:petrixs/cr-transport-bus.git packages/transport-bus; \
	else \
		echo 'transport-bus уже существует'; \
	fi 