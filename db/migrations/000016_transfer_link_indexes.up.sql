-- The list and spending queries join transfer_links on either side of a transfer
-- (`l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id`). Without
-- these, SQLite scans the whole links table for every transaction row.
CREATE INDEX IF NOT EXISTS idx_transfer_links_debit ON transfer_links(debit_transaction_id);
CREATE INDEX IF NOT EXISTS idx_transfer_links_credit ON transfer_links(credit_transaction_id);
