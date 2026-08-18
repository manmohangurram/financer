# Architecture — Data Scoping

Data is scoped per user — every resource table has a `user_id` column and repository queries filter by it. Follow that precedent when adding queries.
