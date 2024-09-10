CREATE TABLE tasks (
                       id SERIAL PRIMARY KEY,
                       type INT CHECK (type >= 0 AND type <= 9),
                       value INT CHECK (value >= 0 AND value <= 99),
                       state TEXT CHECK (state IN ('received', 'processing', 'done')),
                       creation_time TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                       last_update_time TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);