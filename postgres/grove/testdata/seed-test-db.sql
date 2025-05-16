-- This file updates the ephemeral Docker Postgres test database initialized in postgres/docker_test.go
-- with just enough data to run the test of the database driver using an actual Postgres DB instance.

-- Insert into the 'accounts' table
INSERT INTO accounts (id, plan_type, monthly_user_limit)
VALUES ('account_1', 'PLAN_FREE', NULL),
    ('account_2', 'PLAN_UNLIMITED', NULL),
    ('account_3', 'PLAN_FREE', NULL),
    ('account_4', 'PLAN_UNLIMITED', 10000000);

-- Insert into the 'portal_applications' table
INSERT INTO portal_applications (id, account_id)
VALUES ('portal_app_1_no_auth', 'account_1'),
    ('portal_app_2_static_key', 'account_2'),
    ('portal_app_3_static_key', 'account_3'),
    ('portal_app_4_no_auth', 'account_1'),
    ('portal_app_5_static_key', 'account_2'),
    ('portal_app_6_user_limit', 'account_4');

-- Insert into the 'portal_application_settings' table
INSERT INTO portal_application_settings (application_id, secret_key_required, secret_key)
VALUES ('portal_app_1_no_auth', FALSE, NULL),
    ('portal_app_2_static_key', TRUE, 'secret_key_2'),
    ('portal_app_3_static_key', TRUE, 'secret_key_3'),
    ('portal_app_4_no_auth', FALSE, NULL),
    ('portal_app_5_static_key', TRUE, 'secret_key_5'),
    ('portal_app_6_user_limit', FALSE, NULL);