-- Oracle test schema initialization script
-- Runs automatically on first container startup.
--
-- The gvenzl entrypoint executes scripts here connected as SYSDBA in the
-- CDB root, so unqualified CREATE TABLE lands in SYS — not in the app
-- schema or even the ORACLE_DATABASE PDB. Connect explicitly to the PDB
-- as the application user before creating anything.
CONNECT testuser/testpass@//localhost:1521/TESTDB

-- Create test table for unit tests
CREATE TABLE test_users (
    id NUMBER(10) PRIMARY KEY,
    username VARCHAR2(50) NOT NULL,
    email VARCHAR2(100) UNIQUE,
    created_date DATE DEFAULT SYSDATE
);

-- Insert sample data
INSERT INTO test_users (id, username, email) VALUES (1, 'alice', 'alice@example.com');
INSERT INTO test_users (id, username, email) VALUES (2, 'bob', 'bob@example.com');
INSERT INTO test_users (id, username, email) VALUES (3, 'charlie', 'charlie@example.com');
COMMIT;

-- Create sequence for auto-increment
CREATE SEQUENCE test_users_seq START WITH 4 INCREMENT BY 1;
