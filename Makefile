.PHONY: gogo

gogo: stop-services build truncate-logs start-services

build:
	make -C golang

stop-services:
	sudo systemctl stop envoy
	sudo systemctl stop xsuportal-web-golang.service
	sudo systemctl stop xsuportal-api-golang.service
	ssh isucon-app2 sudo systemctl stop mysql

start-services:
	ssh isucon-app2 sudo systemctl start mysql
	sudo systemctl start xsuportal-api-golang.service
	sudo systemctl start xsuportal-web-golang.service
	sleep 5
	sudo systemctl start envoy

status:
	sudo systemctl status xsuportal-api-golang.service
	sudo systemctl status xsuportal-web-golang.service
	ssh isucon-app2 sudo systemctl status mysql
	sudo systemctl status envoy


truncate-logs:
	ssh isucon-app2 sudo truncate --size 0 /var/log/mysql/mysql-slow.sql
	sudo sudo truncate --size 0 /var/log/nginx/access.log

