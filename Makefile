
main: main.go

deploy: main
	GOOS=linux CGO_ENABLED=0 go build -o main main.go
	sls deploy

dart:
	# 'bootstrap' is intentional, see aws lambda custom runtimes
	docker run -v $PWD:/app -w /app -it google/dart dart2native main.dart -o bootstrap
