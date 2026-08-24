-- Restricted account for engine-level read-only tests (cycle 44).
-- Oracle enforces read-only through privileges, so this user holds
-- CREATE SESSION plus explicit SELECT grants only — no table creation,
-- no ANY-privileges, no object-level writes.
--
-- The entrypoint runs this script as SYSDBA in the CDB root; connect to
-- the TESTDB PDB first so the user and grants live where tests connect.
CONNECT sys/oracle@//localhost:1521/TESTDB AS SYSDBA

CREATE USER mcp_ro IDENTIFIED BY "mcp_ro_pass" QUOTA 0 ON USERS;
GRANT CREATE SESSION TO mcp_ro;
GRANT SELECT ON testuser.test_users TO mcp_ro;
