
generate:
	docker run -v $(PWD):/app -w /app -it google/dart pub get
	docker run -v $(PWD):/app -w /app -it google/dart pub run build_runner build

bootstrap: main.dart
	# 'bootstrap' is intentional, see aws lambda custom runtimes
	docker run -v $(PWD):/app -w /app -it google/dart dart2native main.dart -o bootstrap

deploy: bootstrap
	sls deploy

