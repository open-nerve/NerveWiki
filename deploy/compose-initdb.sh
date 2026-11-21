#!/bin/sh
# deploy/compose.yaml 的数据库初始化（M12/P5/S3）：建一个不是超级用户的 nervewiki，作库与之后各表的所有者；
# 服务与迁移都用它。只在第一次建库时执行：改密码要在库里 ALTER ROLE，只改 .env 不够（deploy/README.md）
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres -v pw="$NWIKI_DB_PASSWORD" <<'SQL'
CREATE ROLE nervewiki LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'pw';
CREATE DATABASE nervewiki OWNER nervewiki;
SQL
