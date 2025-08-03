module github.com/timgluz/wasserspiegel/search

go 1.24

toolchain go1.24.5

require (
	github.com/spinframework/spin-go-sdk/v2 v2.2.1
	github.com/timgluz/wasserspiegel v0.0.0-20250724174105-dcf34ff1746d
)

require (
	github.com/gosimple/slug v1.15.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
)

replace github.com/timgluz/wasserspiegel => ./../..
