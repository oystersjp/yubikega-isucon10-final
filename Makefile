.PHONY: gogo

gogo: stop-services build truncate-logs start-services

build:
	cd golang && make all
	scp -rC ~/webapp/golang/bin isucon-app3:~/webapp/golang

stop-services:
	sudo systemctl stop nginx
	ssh isucon-app2 sudo systemctl stop mysql
	ssh isucon-app3 sudo systemctl stop xsuportal-web-golang.service
	ssh isucon-app3 sudo systemctl stop xsuportal-api-golang.service

start-services:
	ssh isucon-app2 sudo systemctl start mysql
	ssh isucon-app3 sudo systemctl start xsuportal-api-golang.service
	ssh isucon-app3 sudo systemctl start xsuportal-web-golang.service
	sleep 5
	sudo systemctl start nginx

truncate-logs:
	ssh isucon-app3 sudo truncate --size 0 /var/log/mysql/mysql-slow.log
	sudo sudo truncate --size 0 /var/log/nginx/access.log

