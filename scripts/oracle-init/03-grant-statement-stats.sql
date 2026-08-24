-- Statement-statistics access for performance tooling (cycle 52).
-- engine_slow_queries / workload_suggestions read V$SQLAREA on Oracle;
-- without this grant the actions degrade gracefully to a hint instead.
CONNECT sys/oracle@//localhost:1521/TESTDB AS SYSDBA

GRANT SELECT ON SYS.V_$SQLAREA TO testuser;
