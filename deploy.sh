#!/bin/bash

# 拉取最新镜像
docker-compose -f docker-compose.yml pull

# 重启服务
docker-compose -f docker-compose.yml down
docker-compose -f docker-compose.yml up -d 
