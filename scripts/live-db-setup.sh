#!/bin/bash
# Spin up throwaway PostgreSQL + MySQL instances matching the DSNs the
# *_Live regression tests expect (docker-compose.test.yml ports), so the
# live tests run against real engines without Docker.
#
#   scripts/live-db-setup.sh start   # init + launch both engines, seed db1
#   scripts/live-db-setup.sh stop    # shut down and remove all state
#
# Engines are found in PATH first, then common Homebrew locations.
set -euo pipefail

ACTION="${1:-start}"
PG_DIR=/tmp/dbmcp-live-pg
PG_SOCK=/tmp/dbmcp-live-pgsock
MY_DIR=/tmp/dbmcp-live-my
MY_SOCK=/tmp/dbmcp-live-my.sock
MYSQLD_CANDIDATES=(mysqld /opt/homebrew/opt/mysql/bin/mysqld)
MYSQL_CANDIDATES=(mysql /opt/homebrew/opt/mysql/bin/mysql)

find_bin() {
    for c in "$@"; do
        if command -v "$c" > /dev/null 2>&1; then
            command -v "$c"
            return 0
        fi
    done
    return 1
}

seed_pg() {
    psql -h localhost -p 15432 -U user1 -d db1 -v ON_ERROR_STOP=1 << 'SQL'
CREATE TABLE IF NOT EXISTS orders (id SERIAL PRIMARY KEY, customer_id INT, region TEXT, total REAL);
CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders (customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_customer_copy ON orders (customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_cust_region ON orders (customer_id, region);
UPDATE pg_index SET indisvalid = false WHERE indexrelid = 'idx_orders_customer_copy'::regclass;
SQL
    # Seed rows only when empty (idempotent across restarts of same datadir).
    local n
    n=$(psql -h localhost -p 15432 -U user1 -d db1 -tAc "SELECT count(*) FROM orders")
    if [ "$n" -eq 0 ]; then
        psql -h localhost -p 15432 -U user1 -d db1 -c \
            "INSERT INTO orders (customer_id, region, total) SELECT i%100, 'r'||(i%5), i*1.0 FROM generate_series(1,2000) i"
    fi
}

seed_my() {
    local my
    my=$(find_bin "${MYSQL_CANDIDATES[@]}")
    "$my" -h 127.0.0.1 -P 13306 -u root << 'SQL'
CREATE USER IF NOT EXISTS 'user1'@'localhost' IDENTIFIED BY 'password1';
CREATE USER IF NOT EXISTS 'user1'@'127.0.0.1' IDENTIFIED BY 'password1';
-- Tests connect over TCP from the host, which MySQL sees as '%'.
CREATE USER IF NOT EXISTS 'user1'@'%' IDENTIFIED BY 'password1';
GRANT ALL PRIVILEGES ON *.* TO 'user1'@'localhost';
GRANT ALL PRIVILEGES ON *.* TO 'user1'@'127.0.0.1';
GRANT ALL PRIVILEGES ON *.* TO 'user1'@'%';
FLUSH PRIVILEGES;
SQL
    "$my" -h 127.0.0.1 -P 13306 -u user1 -ppassword1 << 'SQL'
CREATE DATABASE IF NOT EXISTS db1;
USE db1;
CREATE TABLE IF NOT EXISTS orders (id INT PRIMARY KEY AUTO_INCREMENT, customer_id INT, region VARCHAR(50), total REAL);
CREATE INDEX idx_orders_customer ON orders (customer_id);
CREATE INDEX idx_orders_customer_copy ON orders (customer_id);
SET SESSION cte_max_recursion_depth = 5000;
INSERT INTO orders (customer_id, region, total)
WITH RECURSIVE seq AS (SELECT 1 AS i UNION ALL SELECT i+1 FROM seq WHERE i < 2000)
SELECT i%100, CONCAT('r', i%5), i*1.0 FROM seq;
SQL
}

start_pg() {
    if pg_ctl -D "$PG_DIR" status > /dev/null 2>&1; then
        echo "pg already running on 15432"; return
    fi
    if [ ! -d "$PG_DIR" ]; then
        mkdir -p "$PG_SOCK"
        initdb -D "$PG_DIR" -U postgres --auth=trust > /dev/null
    fi
    pg_ctl -D "$PG_DIR" -o "-p 15432 -k $PG_SOCK" -l /tmp/dbmcp-live-pg.log start
    sleep 1
    psql -h localhost -p 15432 -U postgres -d postgres -v ON_ERROR_STOP=1 << 'SQL'
CREATE ROLE user1 LOGIN PASSWORD 'password1' SUPERUSER;
CREATE DATABASE db1 OWNER user1;
SQL
    seed_pg
    echo "postgres ready: host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable"
}

start_my() {
    local mysqld
    mysqld=$(find_bin "${MYSQLD_CANDIDATES[@]}") || { echo "mysqld not found; skipping mysql" >&2; return 0; }
    if [ ! -d "$MY_DIR/mysql" ]; then
        mkdir -p "$MY_DIR"
        "$mysqld" --initialize-insecure --datadir="$MY_DIR" --user="$(whoami)" > /dev/null 2>&1
    fi
    "$mysqld" --datadir="$MY_DIR" --port=13306 --socket="$MY_SOCK" --bind-address=127.0.0.1 >> /tmp/dbmcp-live-my.log 2>&1 &
    sleep 5
    local my
    my=$(find_bin "${MYSQL_CANDIDATES[@]}")
    "$my" -h 127.0.0.1 -P 13306 -u root -e "SELECT 1" > /dev/null 2>&1 || { echo "mysql not reachable; see log" >&2; return 0; }
    seed_my
    echo "mysql ready: user1:password1@tcp(localhost:13306)/db1?parseTime=true"
}

case "$ACTION" in
    start)
        start_pg
        start_my
        ;;
    stop)
        pg_ctl -D "$PG_DIR" -m fast stop 2>/dev/null || true
        MY=$(find_bin "${MYSQL_CANDIDATES[@]}" || true)
        if [ -n "${MY:-}" ]; then
            "$MY" -h 127.0.0.1 -P 13306 -u root -e "SHUTDOWN;" 2> /dev/null || true
        fi
        sleep 2
        rm -rf "$PG_DIR" "$PG_SOCK" "$MY_DIR" "$MY_SOCK"
        echo "live engines stopped and state removed"
        ;;
    *)
        echo "usage: $0 {start|stop}" >&2
        exit 1
        ;;
esac
