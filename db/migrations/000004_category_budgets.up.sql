-- Per-category monthly budget: one amount, repeating every month. 0 means no
-- budget (the category only reports spend).
ALTER TABLE categories ADD COLUMN monthly_budget REAL NOT NULL DEFAULT 0;
