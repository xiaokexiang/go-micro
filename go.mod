module main

go 1.19


require (
	client v0.0.1
	server v0.0.1
	registry v0.0.1
	service v0.0.1
)
replace (
	client => ./client
	server => ./server
	registry => ./registry
	service => ./service
)