PWD = $(shell pwd)

all: run

run:
	@go build && ./2022-Coding-challenge

.PHONY: all run
