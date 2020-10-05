
main: main.go

deploy: main
	GOOS=linux CGO_ENABLED=0 go build -o main main.go
	sls deploy