module github.com/timgluz/wasserspiegel/app

go 1.24

toolchain go1.24.5

require (
	github.com/spinframework/spin-go-sdk/v2 v2.2.1
	github.com/timgluz/wasserspiegel v0.0.0-20250724174105-dcf34ff1746d
	go.uber.org/fx v1.24.0
)

require (
	github.com/gosimple/slug v1.15.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
)

replace github.com/timgluz/wasserspiegel => ./../..
