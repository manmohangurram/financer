//! Analytics endpoint tests: dashboard totals and spending buckets/categories.

use axum::http::StatusCode;

use crate::tests::harness::TestApp;

#[tokio::test]
async fn dashboard_totals_reflect_transactions() {
    let app = TestApp::new().await;
    app.create_txn("Salary", 1000.0, "CREDIT").await;
    app.create_txn("Rent", 400.0, "DEBIT").await;

    let (status, d) = app.get_json("/api/dashboard", Some(&app.token)).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(d["totalIncome"].as_f64(), Some(1000.0), "dashboard: {d}");
    assert_eq!(d["totalExpenses"].as_f64(), Some(400.0), "dashboard: {d}");
    assert_eq!(d["totalBalance"].as_f64(), Some(600.0), "dashboard: {d}");
}

#[tokio::test]
async fn spending_counts_multi_category_txn_once_per_bucket() {
    let app = TestApp::new().await;

    let cats = app
        .post_json(
            "/api/categories",
            &serde_json::json!({ "categories": [{ "name": "Food" }, { "name": "Travel" }] }),
            Some(&app.token),
        )
        .await;
    assert_eq!(cats.0, StatusCode::CREATED, "cat create: {}", cats.1);
    let (_, list) = app.get_json("/api/categories", Some(&app.token)).await;
    let ids: Vec<String> = list["categories"].as_array().unwrap().iter().map(|c| c["id"].as_str().unwrap().to_string()).collect();

    let occurred_at = (chrono::Utc::now() - chrono::Duration::days(1)).format("%Y-%m-%dT%H:%M:%SZ").to_string();
    let txn = app
        .post_json(
            "/api/transactions",
            &serde_json::json!({
                "transactions": [{ "name": "Trip snack", "amount": 40.0, "type": "DEBIT",
                                   "accountId": app.account_id, "categoryIds": ids, "occurredAt": occurred_at }]
            }),
            Some(&app.token),
        )
        .await;
    assert_eq!(txn.0, StatusCode::CREATED, "txn create: {}", txn.1);

    let (_, s) = app.get_json("/api/spending?range=1M", Some(&app.token)).await;
    // One transaction, two categories → the bucket still counts it once.
    let bucket_total: f64 = s["buckets"].as_array().unwrap().iter().map(|b| b["amount"].as_f64().unwrap()).sum();
    assert_eq!(bucket_total, 40.0, "bucket double-counted a multi-category txn: {s}");
    // Both categories attribute the amount (existing behaviour).
    assert_eq!(s["categories"].as_array().map(Vec::len), Some(2), "{s}");
}

#[tokio::test]
async fn spending_buckets_and_categories() {
    let app = TestApp::new().await;

    let cat = app
        .post_json(
            "/api/categories",
            &serde_json::json!({ "categories": [{ "name": "Food" }] }),
            Some(&app.token),
        )
        .await;
    assert_eq!(cat.0, StatusCode::CREATED, "cat create: {}", cat.1);
    let (_, cats) = app.get_json("/api/categories", Some(&app.token)).await;
    let cat_id = cats["categories"][0]["id"].as_str().unwrap().to_string();

    // Explicit timestamp a day ago: the spending window is [from, now) and
    // go_ts truncates to seconds, so a txn at "now" can fall on the boundary.
    let occurred_at = (crate::utils::timex::now_utc() - chrono::Duration::days(1)).format("%Y-%m-%dT%H:%M:%SZ").to_string();
    let txn = app
        .post_json(
            "/api/transactions",
            &serde_json::json!({
                "transactions": [{ "name": "Lunch", "amount": 30.0, "type": "DEBIT",
                                   "accountId": app.account_id, "categoryIds": [cat_id],
                                   "occurredAt": occurred_at }]
            }),
            Some(&app.token),
        )
        .await;
    assert_eq!(txn.0, StatusCode::CREATED, "txn create: {}", txn.1);

    let (status, s) = app.get_json("/api/spending?range=1M", Some(&app.token)).await;
    assert_eq!(status, StatusCode::OK, "spending: {s}");
    assert_eq!(s["categories"].as_array().map(Vec::len), Some(1), "spending: {s}");
    assert_eq!(s["categories"][0]["name"].as_str(), Some("Food"));
    assert_eq!(s["categories"][0]["debit"].as_f64(), Some(30.0));
    assert!(!s["buckets"].as_array().unwrap().is_empty(), "spending: {s}");
}
