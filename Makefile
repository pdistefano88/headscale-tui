.PHONY: test vet lab-build lab-up lab-tui lab-status lab-logs lab-down lab-reset

test:
	go test ./...

vet:
	go vet ./...

lab-build:
	bash scripts/lab.sh build

lab-up:
	bash scripts/lab.sh up

lab-tui:
	bash scripts/lab.sh tui

lab-full: lab-reset lab-build lab-up
	bash scripts/lab.sh tui

lab-status:
	bash scripts/lab.sh status

lab-logs:
	bash scripts/lab.sh logs

lab-down:
	bash scripts/lab.sh down

lab-reset:
	bash scripts/lab.sh reset
