
generate: lib/friend.g.dart

lib/friend.g.dart: lib/friend.dart
	docker run -v $(PWD):/app -w /app -it google/dart ./docker-generate.sh

bootstrap: *.dart lib/*.dart pubspec.*
	# 'bootstrap' is intentional, see aws lambda custom runtimes
	# though serverless may fix this.
	docker run -v $(PWD):/app -w /app -it google/dart ./docker-build.sh

deploy: bootstrap
	sls deploy

