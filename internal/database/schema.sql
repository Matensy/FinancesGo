-- FinancesGo database schema. SQLite.

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bills (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    amount     REAL NOT NULL,
    due_day    INTEGER NOT NULL,
    category   TEXT NOT NULL DEFAULT 'Outros',
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bill_payments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_id    INTEGER NOT NULL REFERENCES bills(id) ON DELETE CASCADE,
    period     TEXT NOT NULL,        -- YYYY-MM
    amount     REAL NOT NULL,
    paid_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (bill_id, period)
);

CREATE TABLE IF NOT EXISTS recurring_incomes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    amount     REAL NOT NULL,
    day        INTEGER NOT NULL,
    category   TEXT NOT NULL DEFAULT 'Salário',
    is_salary  INTEGER NOT NULL DEFAULT 0,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS incomes (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    description        TEXT NOT NULL,
    amount             REAL NOT NULL,
    date               DATE NOT NULL,
    category           TEXT NOT NULL DEFAULT 'Outros',
    confirmed          INTEGER NOT NULL DEFAULT 1,
    source             TEXT NOT NULL DEFAULT 'manual',
    pokemon_account_id INTEGER REFERENCES pokemon_accounts(id) ON DELETE SET NULL,
    period             TEXT,
    external_id        TEXT,
    voided             INTEGER NOT NULL DEFAULT 0,
    sale_date          DATE,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- NOTE: the unique index on external_id is created in migrate() (db.go), after
-- the column is guaranteed to exist on databases created by older versions.

CREATE TABLE IF NOT EXISTS expenses (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    description        TEXT NOT NULL,
    amount             REAL NOT NULL,
    date               DATE NOT NULL,
    category           TEXT NOT NULL DEFAULT 'Outros',
    source             TEXT NOT NULL DEFAULT 'manual',
    pokemon_account_id INTEGER REFERENCES pokemon_accounts(id) ON DELETE SET NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pokemon_accounts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL DEFAULT '',
    level         INTEGER NOT NULL DEFAULT 0,
    team          TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    legendaries   TEXT NOT NULL DEFAULT '',
    shinies       INTEGER NOT NULL DEFAULT 0,
    pokemon_count INTEGER NOT NULL DEFAULT 0,
    bag_capacity  INTEGER NOT NULL DEFAULT 0,
    base_value    REAL NOT NULL DEFAULT 0,
    ggmax_rate    REAL NOT NULL DEFAULT 0.1598,
    tags          TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'ativa',
    problem_note  TEXT NOT NULL DEFAULT '',
    problem_at    DATETIME,
    sold_at       DATETIME,
    sold_value    REAL,
    matures_at    DATETIME,
    matured       INTEGER NOT NULL DEFAULT 0,
    refunded_at   DATETIME,
    refund_reason TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pokemon_price_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id  INTEGER NOT NULL REFERENCES pokemon_accounts(id) ON DELETE CASCADE,
    base_value  REAL NOT NULL,
    ggmax_rate  REAL NOT NULL,
    final_price REAL NOT NULL,
    recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS export_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    filename      TEXT NOT NULL,
    format        TEXT NOT NULL,
    path          TEXT NOT NULL,
    period        TEXT NOT NULL,
    account_count INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_incomes_date ON incomes(date);
CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date);
CREATE INDEX IF NOT EXISTS idx_bill_payments_period ON bill_payments(period);
CREATE INDEX IF NOT EXISTS idx_pokemon_status ON pokemon_accounts(status);
