.PHONY: gogo

gogo: stop-services build truncate-logs start-services

build:
	make -C golang
	scp -C ~/webapp/golang/bin/benchmark_server isucon-app2:~/webapp/golang/bin/benchmark_server
	scp -C ~/webapp/golang/bin/send_web_push isucon-app2:~/webapp/golang/bin/send_web_push
	scp -C ~/webapp/golang/bin/xsuportal isucon-app2:~/webapp/golang/bin/xsuportal

start-services:
	ssh isucon-app2 sudo systemctl start xsuportal-api-golang.service
	ssh isucon-app2 sudo systemctl start xsuportal-web-golang.service

stop-services:
	ssh isucon-app2 sudo systemctl stop xsuportal-api-golang.service
	ssh isucon-app2 sudo systemctl stop xsuportal-web-golang.service

truncate-logs:
	ssh isucon-app2 sudo truncate --size 0 /var/log/mysql/mysql-slow.log
	#sudo truncate --size 0 /var/log/nginx/access.log

