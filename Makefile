
generate:
	docker run -v $(PWD):/app -w /app -it google/dart ./docker-generate.sh

bootstrap: main.dart
	# 'bootstrap' is intentional, see aws lambda custom runtimes
	# though serverless may fix this.
	docker run -v $(PWD):/app -w /app -it google/dart ./docker-build.sh

deploy: bootstrap
	sls deploy

