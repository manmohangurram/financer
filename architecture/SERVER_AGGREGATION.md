# Architecture — Server-Side Aggregation

All finance math lives in the backend. `GET /api/dashboard` returns balance/income/expense sums + portfolio + accounts + investments; `GET /api/spending?range=…` returns day/month buckets and per-category debit/credit/net computed in SQL (transfer exclusion via `transfer_links` + debt account types). The frontend renders, never aggregates. Transaction filtering/pagination/sorting (`GET /api/transactions`) is server-side.
